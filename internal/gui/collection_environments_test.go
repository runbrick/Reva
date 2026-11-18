package gui

import (
	"slices"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/vivaui/viva/gfx"
)

func TestCollectionEnvironmentSelectionAndVisibilityPersist(t *testing.T) {
	svc, dir := importRegressionServices(t)
	global := model.Environment{ID: "shared-dev", Name: "Development", Variables: []model.KV{{Key: "host", Value: "dev.example.test"}}}
	other := model.Environment{ID: "shared-stage", Name: "Staging", Variables: []model.KV{{Key: "host", Value: "stage.example.test"}}}
	authMust(t, svc.Environments.Save(global))
	authMust(t, svc.Environments.Save(other))
	settings := svc.Settings.Get()
	settings.Language, settings.ActiveEnvID = "en", global.ID
	authMust(t, svc.Settings.Save(settings))
	first, err := svc.Collections.Create("First workspace")
	authMust(t, err)
	second, err := svc.Collections.Create("Second workspace")
	authMust(t, err)
	h := newHarness(t, svc)
	firstRequest := model.NewRequest("first")
	firstRequest.URL = "https://{{host}}/first"
	secondRequest := model.NewRequest("second")
	secondRequest.URL = "https://{{host}}/second"
	h.App.OpenRequest(firstRequest, first.ID)
	firstTab := h.App.Active()
	requestRepaint(h)
	if h.App.env.activeID != "" {
		t.Fatal("new collection inherited the unfiled selection")
	}
	envSelect(h, global.Name)
	h.App.OpenRequest(secondRequest, second.ID)
	secondTab := h.App.Active()
	requestRepaint(h)
	envSelect(h, other.Name)
	updated, _ := svc.Environments.Get(global.ID)
	updated.Variables[0].Value = "new-dev.example.test"
	authMust(t, svc.Environments.Save(updated))
	if got := h.App.requestPreviewSnapshot(firstTab).EnvironmentName; got != global.Name {
		t.Fatalf("background collection preview did not use its own shared selection: %s", got)
	}
	if got := h.App.requestPreviewSnapshot(secondTab).EnvironmentName; got != other.Name {
		t.Fatalf("second collection preview used a different selection: %s", got)
	}
	if got := h.App.VarsFor(firstTab); len(got) != 1 || got[0].Value != "new-dev.example.test" {
		t.Fatalf("background collection did not see the shared edit: %+v", got)
	}
	h.App.openCollectionSettings(collectionNode{collectionID: first.ID}, collectionSettingsGeneral)
	requestRepaint(h)
	collectionsOption(h, h.App.S.Collections.Environments)
	clickCollectionEnvironmentCheckbox(h, global.Name)
	if h.App.env.activeID != global.ID || !h.App.Active().Dirty {
		t.Fatal("an unsaved visibility edit changed the live picker or did not mark the tab dirty")
	}
	h.click(h.App.S.Collections.DiscardReload)
	if h.App.Active().Dirty || h.App.env.activeID != global.ID {
		t.Fatal("discard did not restore the visibility draft")
	}
	clickCollectionEnvironmentCheckbox(h, global.Name)
	clickCollectionEnvironmentCheckbox(h, other.Name)
	h.click(h.App.S.Collections.Save)
	h.click(h.App.S.Env.Title)
	if h.App.env.activeID != "" {
		t.Fatal("hidden environment was still active")
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Layer > 0 && (hit.Label == global.Name || hit.Label == other.Name) {
			t.Fatalf("hidden environment %q was offered by the picker", hit.Label)
		}
	}
	if svc.Settings.Get() != settings || len(h.App.VarsFor(firstTab)) != 0 {
		t.Fatal("hiding a collection environment affected unrelated settings or request variables")
	}
	reopened, err := store.Open(dir)
	authMust(t, err)
	savedFirst, _ := reopened.Collections().Get(first.ID)
	savedSecond, _ := reopened.Collections().Get(second.ID)
	if savedFirst.EnvironmentScope.VisibleIDs == nil || len(savedFirst.EnvironmentScope.VisibleIDs) != 0 || savedFirst.EnvironmentScope.ActiveID != "" || savedSecond.EnvironmentScope.ActiveID != other.ID {
		t.Fatalf("unexpected environment state after reload: first=%+v second=%+v", savedFirst.EnvironmentScope, savedSecond.EnvironmentScope)
	}
}

func TestCollectionOnlyEnvironmentEditorAndRunner(t *testing.T) {
	h := newHarness(t)
	owner, err := h.Svc.Collections.Create("Owner")
	authMust(t, err)
	other, err := h.Svc.Collections.Create("Other")
	authMust(t, err)
	h.App.openCollectionSettings(collectionNode{collectionID: owner.ID}, collectionSettingsEnvironments)
	requestRepaint(h)
	h.click(h.App.S.Collections.NewLocalEnvironment)
	envReplaceField(h, h.App.S.Env.Name, "Owner sandbox")
	collectionsOption(h, h.App.S.Common.Add)
	envClickField(h, h.App.S.Common.Key)
	authMust(t, h.D.Type("scope"))
	h.typeInto(h.App.S.Common.Value, "owner-only")
	h.click(h.App.S.Collections.Save)
	stored, _ := h.Svc.Collections.Get(owner.ID)
	if len(stored.EnvironmentScope.Local) != 1 || len(h.Svc.Environments.List()) != 0 {
		t.Fatal("collection environment did not stay in its owner")
	}
	private := stored.EnvironmentScope.Local[0]
	h.click(h.App.S.Env.Title)
	collectionsOption(h, private.Name)
	if got := h.App.Vars(); len(got) != 1 || got[0].Value != "owner-only" {
		t.Fatalf("private environment was not applied: %+v", got)
	}
	h.App.openCollectionRunner(other.ID)
	if slices.Contains(h.App.collections.runner.environmentIDs, private.ID) {
		t.Fatal("different collection's runner offered a private environment")
	}
	h.App.dismissCollectionRunner()
	h.App.openCollectionRunner(owner.ID)
	if !slices.Contains(h.App.collections.runner.environmentIDs, private.ID) {
		t.Fatal("owning collection's runner omitted a private environment")
	}
	h.App.dismissCollectionRunner()
	requestRepaint(h)
	h.click(h.App.S.Env.Edit)
	h.click(h.App.S.Env.Delete)
	h.click(h.App.S.Collections.Save)
	stored, _ = h.Svc.Collections.Get(owner.ID)
	if len(stored.EnvironmentScope.Local) != 0 || stored.EnvironmentScope.ActiveID != "" || h.App.env.activeID != "" {
		t.Fatal("deleting the selected environment left it active")
	}
}

func clickCollectionEnvironmentCheckbox(h *harness, label string) {
	h.t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Role == gfx.RoleCheckbox && hit.Label == label {
			bounds := hit.Bounds
			authMust(h.t, h.D.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2))
			return
		}
	}
	h.t.Fatalf("no environment checkbox %q; controls: %q", label, h.hitLabels())
}
