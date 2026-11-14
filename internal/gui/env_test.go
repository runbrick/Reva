package gui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func envReplaceField(h *harness, label, value string) {
	h.t.Helper()
	envClickField(h, label)
	if handled, err := h.D.Shortcut('a'); err != nil || !handled {
		h.t.Fatalf("select field text: handled=%v err=%v", handled, err)
	}
	if err := h.D.Type(value); err != nil {
		h.t.Fatal(err)
	}
}

func envClickField(h *harness, label string) {
	h.t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label && hit.Role == gfx.RoleTextField {
			b := hit.Bounds
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				h.t.Fatal(err)
			}
			return
		}
	}
	h.t.Fatalf("no text field %q", label)
}

func envSelect(h *harness, name string) {
	h.t.Helper()
	h.click(h.App.S.Env.Title)
	h.click(name)
}

func envCaptureRequests(h *harness) <-chan model.Request {
	h.t.Helper()
	received := make(chan model.Request, 8)
	h.Svc.Requests.(*fake.Requests).Fn = func(_ context.Context, req model.Request) (model.Response, error) {
		received <- req
		return model.Response{StatusCode: 200, URL: req.URL}, nil
	}
	return received
}

func envSend(h *harness, received <-chan model.Request) model.Request {
	h.t.Helper()
	h.click("Send")
	h.waitFor("environment request", func() bool { return !h.App.Active().Sending && h.App.Active().Response != nil })
	select {
	case req := <-received:
		return req
	default:
		h.t.Fatal("request fake did not receive the send")
		return model.Request{}
	}
}

func envManage(h *harness) {
	h.t.Helper()
	h.click(h.App.S.Env.Title)
	h.click(h.App.S.Env.Manage)
	if h.App.Section != SectionEnvironments || h.App.Main != PageWorkbench || !h.App.envReady() {
		h.t.Fatal("Manage environments did not open the environment page")
	}
}

func envAction(h *harness, name, action string) {
	h.t.Helper()
	h.click(name)
	found := false
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == name {
			found = true
			continue
		}
		if found && hit.Label == h.App.S.Env.Actions {
			b := hit.Bounds
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				h.t.Fatal(err)
			}
			h.click(action)
			return
		}
	}
	h.t.Fatalf("no actions for %q", name)
}

func TestEnvironmentCreateSelectAndSend(t *testing.T) {
	for _, language := range []string{"zh", "en"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(language+"/"+theme, func(t *testing.T) {
				svc := fake.New()
				settings := svc.Settings.Get()
				settings.Language, settings.Theme, settings.HistoryLimit = language, theme, 37
				if err := svc.Settings.Save(settings); err != nil {
					t.Fatal(err)
				}
				h := newHarness(t, svc)
				received := envCaptureRequests(h)
				envManage(h)
				if !h.shows(h.App.S.Env.Empty) {
					t.Fatal("missing empty environment page")
				}
				h.click(h.App.S.Env.New)
				if len(svc.Environments.List()) != 1 {
					t.Fatal("sidebar create did not call the service")
				}
				envReplaceField(h, h.App.S.Env.Name, "Development")
				h.click(h.App.S.Common.Add)
				envClickField(h, h.App.S.Common.Key)
				if err := h.D.Type("host"); err != nil {
					t.Fatal(err)
				}
				h.typeInto(h.App.S.Common.Value, "https://dev.example.test")
				h.typeInto(h.App.S.Request.Description, "Development host")
				if !h.shows(h.App.S.Env.SaveHint) || !h.App.env.current().dirty() {
					t.Fatal("editor missing save hint or dirty state")
				}
				before := svc.Environments.List()[0]
				if before.Name == "Development" || len(before.Variables) != 0 {
					t.Fatal("typing changed stored environment before Save")
				}
				h.click(h.App.S.Env.Save)
				envs := svc.Environments.List()
				want := []model.KV{{Key: "host", Value: "https://dev.example.test", Description: "Development host"}}
				if len(envs) != 1 || envs[0].Name != "Development" || !reflect.DeepEqual(envs[0].Variables, want) || h.App.env.current().dirty() {
					t.Fatalf("saved environments: %+v", envs)
				}
				h.click(h.App.S.Shell.APIs)
				envSelect(h, "Development")
				settings.ActiveEnvID = envs[0].ID
				if got := svc.Settings.Get(); got != settings {
					t.Fatalf("selection changed other settings: %+v", got)
				}
				h.App.Active().Request.URL = "{{host}}/users"
				h.pump()
				if req := envSend(h, received); req.URL != "https://dev.example.test/users" {
					t.Fatalf("sent URL %q", req.URL)
				}
				reopened := newHarness(t, svc)
				if reopened.App.env.activeID != envs[0].ID || !reopened.shows("Development") {
					t.Fatal("selection was not restored")
				}
				envSelect(h, h.App.S.Env.NoEnvironment)
				settings.ActiveEnvID = ""
				if got := svc.Settings.Get(); got != settings {
					t.Fatalf("No environment settings: %+v", got)
				}
				if req := envSend(h, received); req.URL != "{{host}}/users" {
					t.Fatalf("No environment sent URL %q", req.URL)
				}
			})
		}
	}
}

func TestEnvironmentOverridesCollectionVariables(t *testing.T) {
	h := newHarness(t)
	received := envCaptureRequests(h)
	col, err := h.Svc.Collections.Create("Collection")
	if err != nil {
		t.Fatal(err)
	}
	col.Variables = []model.KV{{Key: "host", Value: "https://collection.example.test"}, {Key: "path", Value: "users"}}
	if err := h.Svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	env := model.Environment{ID: "dev", Name: "Development", Variables: []model.KV{
		{Key: "host", Value: "https://env.example.test"},
		{Key: "host", Value: "https://disabled.example.test", Disabled: true},
	}}
	if err := h.Svc.Environments.Save(env); err != nil {
		t.Fatal(err)
	}
	h.App.Active().CollectionID = col.ID
	h.App.Active().Request.URL = "{{host}}/{{path}}"
	h.pump()
	envSelect(h, env.Name)
	want := append(append([]model.KV(nil), col.Variables...), env.Variables[0])
	if got := h.App.Vars(); !reflect.DeepEqual(got, want) {
		t.Fatalf("variable precedence: got %+v want %+v", got, want)
	}
	if req := envSend(h, received); req.URL != "https://env.example.test/users" {
		t.Fatalf("environment override: %q", req.URL)
	}
	envSelect(h, h.App.S.Env.NoEnvironment)
	if req := envSend(h, received); req.URL != "https://collection.example.test/users" {
		t.Fatalf("collection fallback: %q", req.URL)
	}
	// Returned variables belong to the send, not the service's backing slice.
	vs := h.App.Vars()
	vs[0].Value = "changed"
	stored, _ := h.Svc.Collections.Get(col.ID)
	if stored.Variables[0].Value != col.Variables[0].Value {
		t.Fatal("Vars aliases stored collection variables")
	}
}

func TestEnvironmentUnresolvedVarsAndTooltip(t *testing.T) {
	h := newHarness(t)
	h.App.Active().Request.URL = "https://{{missing}}/{{ missing }}/{{known}}"
	h.App.Active().Request.Headers = []model.KV{
		{Key: "X-Token", Value: "{{token}}/{{missing}}"},
		{Key: "{{ignored-key}}", Value: "literal"},
		{Key: "X-Disabled", Value: "{{ignored-value}}", Disabled: true},
	}
	env := model.Environment{ID: "dev", Name: "Development", Variables: []model.KV{
		{Key: "known", Value: "value"}, {Key: "token", Value: "disabled", Disabled: true},
	}}
	if err := h.Svc.Environments.Save(env); err != nil {
		t.Fatal(err)
	}
	settings := h.Svc.Settings.Get()
	settings.ActiveEnvID = env.ID
	if err := h.Svc.Settings.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	if got := h.App.UnresolvedVars(); !reflect.DeepEqual(got, []string{"missing", "token"}) {
		t.Fatalf("unresolved variables: %q", got)
	}
	badge := fmt.Sprintf(h.App.S.Env.Unresolved, 2)
	if h.shows(badge) {
		t.Fatal("warning should be a compact dot, not a text badge")
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == badge {
			if err := h.D.Move(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Advance(time.Second); err != nil {
				t.Fatal(err)
			}
			if !h.shows(h.App.S.Env.UnresolvedTip + "missing, token") {
				t.Fatal("tooltip does not list unresolved names")
			}
			return
		}
	}
	t.Fatal("warning badge cannot be hovered")
}

func TestEnvironmentSidebarRenameDuplicateDeleteAndSetActive(t *testing.T) {
	h := newHarness(t)
	original := model.Environment{Version: model.SchemaVersion, ID: "original", Name: "Original", Variables: []model.KV{{Key: "host", Value: "original", Description: "Original host"}}}
	if err := h.Svc.Environments.Save(original); err != nil {
		t.Fatal(err)
	}
	h.pump()
	envManage(h)
	envAction(h, original.Name, h.App.S.Env.Rename)
	envReplaceField(h, h.App.S.Env.Name, "Renamed")
	h.click(h.App.S.Env.Save)
	stored, _ := h.Svc.Environments.Get(original.ID)
	if stored.Name != "Renamed" {
		t.Fatalf("rename did not persist: %+v", stored)
	}
	envAction(h, "Renamed", h.App.S.Env.Duplicate)
	envs := h.Svc.Environments.List()
	if len(envs) != 2 {
		t.Fatalf("duplicate not persisted: %+v", envs)
	}
	copy := h.App.env.current()
	if copy.id == original.ID || copy.name != fmt.Sprintf(h.App.S.Env.CopyName, "Renamed") {
		t.Fatal("duplicate did not get a new identity and name")
	}
	envReplaceField(h, h.App.S.Common.Value, "copy value")
	h.click(h.App.S.Env.Save)
	stored, _ = h.Svc.Environments.Get(original.ID)
	if stored.Variables[0].Value != "original" || stored.Variables[0].Description != "Original host" {
		t.Fatal("duplicate aliases original variables")
	}
	envAction(h, copy.name, h.App.S.Env.SetActive)
	if h.Svc.Settings.Get().ActiveEnvID != copy.id {
		t.Fatal("Set active did not persist")
	}
	envAction(h, copy.name, h.App.S.Env.Delete)
	if !h.App.env.deleteOpen || !h.shows(copy.name) {
		t.Fatalf("delete missing confirmation: %+v", h.D.Snapshot().Labels)
	}
	h.click(h.App.S.Env.Cancel)
	if len(h.Svc.Environments.List()) != 2 {
		t.Fatal("cancel deleted the environment")
	}
	envAction(h, copy.name, h.App.S.Env.Delete)
	h.click(h.App.S.Env.Delete)
	if _, ok := h.Svc.Environments.Get(copy.id); ok || h.Svc.Settings.Get().ActiveEnvID != "" {
		t.Fatal("confirmed delete did not clear active environment")
	}
	if h.App.env.current().id != original.ID {
		t.Fatal("deleting selected environment did not choose remaining row")
	}
}

func TestEnvironmentPageDiscardShortcutAndRowIdentity(t *testing.T) {
	h := newHarness(t)
	env := model.Environment{ID: "dev", Name: "Development", Variables: []model.KV{{Key: "first", Value: "one"}, {Key: "second", Value: "two"}}}
	if err := h.Svc.Environments.Save(env); err != nil {
		t.Fatal(err)
	}
	h.pump()
	envManage(h)
	var secondValueID gfx.ID
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Common.Value {
			secondValueID = hit.ID
		}
	}
	h.click(h.App.S.Common.Delete)
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Common.Value && hit.ID != secondValueID {
			t.Fatal("deleting first row changed surviving field identity")
		}
	}
	envReplaceField(h, h.App.S.Common.Value, "discarded")
	h.click(h.App.S.Env.Discard)
	stored, _ := h.Svc.Environments.Get(env.ID)
	if !reflect.DeepEqual(stored, env) || h.App.env.current().dirty() || len(h.App.env.current().variables) != 2 {
		t.Fatal("Discard did not restore service snapshot")
	}
	envReplaceField(h, h.App.S.Common.Value, "saved by shortcut")
	handled, err := h.D.ShortcutChord(ui.Primary, 's')
	if err != nil || !handled {
		t.Fatalf("Primary+S: handled=%v err=%v", handled, err)
	}
	stored, _ = h.Svc.Environments.Get(env.ID)
	if stored.Variables[0].Value != "saved by shortcut" || h.App.env.current().dirty() {
		t.Fatal("Primary+S did not save current environment")
	}
	h.click(h.App.S.Common.Key) // first row's enabled checkbox
	h.click(h.App.S.Env.Save)
	stored, _ = h.Svc.Environments.Get(env.ID)
	if !stored.Variables[0].Disabled {
		t.Fatal("enabled toggle was not saved")
	}
}

func TestEnvironmentDraftsSurviveRowAndSectionSwitches(t *testing.T) {
	h := newHarness(t)
	for _, env := range []model.Environment{{ID: "a", Name: "A", Variables: []model.KV{{Key: "host", Value: "one"}}}, {ID: "b", Name: "B"}} {
		if err := h.Svc.Environments.Save(env); err != nil {
			t.Fatal(err)
		}
	}
	h.pump()
	envManage(h)
	envReplaceField(h, h.App.S.Common.Value, "draft")
	h.click("B")
	envReplaceField(h, h.App.S.Env.Name, "B draft")
	h.click(h.App.S.Shell.APIs)
	envManage(h)
	h.click("A")
	if h.App.env.current().variables[0].Value != "draft" {
		t.Fatal("row or section switch lost draft")
	}
	h.click(h.App.S.Env.Save)
	stored, _ := h.Svc.Environments.Get("b")
	if stored.Name != "B" {
		t.Fatal("saving A persisted another environment's draft")
	}
	h.click("B")
	if h.App.env.current().name != "B draft" {
		t.Fatal("saving A lost B draft")
	}
}

func TestEnvironmentSidebarKeyboardContextMenu(t *testing.T) {
	h := newHarness(t)
	if err := h.Svc.Environments.Save(model.Environment{ID: "dev", Name: "Development"}); err != nil {
		t.Fatal(err)
	}
	h.pump()
	envManage(h)
	h.click("Development")
	if err := h.D.Key(gfx.KeyF10, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Env.SetActive)
	if h.Svc.Settings.Get().ActiveEnvID != "dev" {
		t.Fatal("context menu did not set active environment")
	}
}

type envFailSave struct {
	service.EnvironmentService
	fail bool
}

func (s *envFailSave) Save(env model.Environment) error {
	if s.fail {
		return errors.New("write failed")
	}
	return s.EnvironmentService.Save(env)
}

func TestEnvironmentSaveFailureKeepsDraftForRetry(t *testing.T) {
	svc := fake.New()
	failing := &envFailSave{EnvironmentService: svc.Environments, fail: true}
	svc.Environments = failing
	h := newHarness(t, svc)
	envManage(h)
	h.click(h.App.S.Env.New)
	envReplaceField(h, h.App.S.Env.Name, "Retry")
	h.click(h.App.S.Env.Save)
	if !h.shows("write failed") || h.App.env.current() == nil || !h.App.env.current().dirty() {
		t.Fatal("failed Save lost draft or hid error")
	}
	failing.fail = false
	h.click(h.App.S.Env.Save)
	if envs := svc.Environments.List(); len(envs) != 1 || envs[0].Name != "Retry" {
		t.Fatalf("retry lost edits: %+v", envs)
	}
}

func TestEnvironmentNameRequiredBeforeSaving(t *testing.T) {
	h := newHarness(t)
	envManage(h)
	h.click(h.App.S.Env.New)
	envReplaceField(h, h.App.S.Env.Name, "  ")
	h.click(h.App.S.Env.Save)
	if !h.shows(h.App.S.Env.NameRequired) || h.Svc.Environments.List()[0].Name != h.App.S.Env.NewEnvironment {
		t.Fatal("blank name was saved or validation was hidden")
	}
	envReplaceField(h, h.App.S.Env.Name, "  Development  ")
	h.click(h.App.S.Env.Save)
	if envs := h.Svc.Environments.List(); len(envs) != 1 || envs[0].Name != "Development" {
		t.Fatalf("name correction was not saved: %+v", envs)
	}
}

type envFailSettings struct{ service.SettingsService }

func (s *envFailSettings) Save(model.Settings) error { return errors.New("settings write failed") }

func TestEnvironmentSelectionFailurePreservesSettings(t *testing.T) {
	svc := fake.New()
	if err := svc.Environments.Save(model.Environment{ID: "dev", Name: "Development"}); err != nil {
		t.Fatal(err)
	}
	settings := svc.Settings.Get()
	svc.Settings = &envFailSettings{SettingsService: svc.Settings}
	h := newHarness(t, svc)
	envSelect(h, "Development")
	if got := svc.Settings.Get(); got != settings || !h.shows("settings write failed") {
		t.Fatalf("failed settings Save: settings=%+v", got)
	}
	if len(h.App.Vars()) != 0 || !h.shows(h.App.S.Env.NoEnvironment) {
		t.Fatal("failed selection was used instead of persisted settings")
	}
}

type envFailDelete struct {
	service.EnvironmentService
	failID string
}

func (s *envFailDelete) Delete(id string) error {
	if id == s.failID {
		return errors.New("delete failed")
	}
	if _, ok := s.EnvironmentService.Get(id); !ok {
		return fake.ErrNotFound
	}
	return s.EnvironmentService.Delete(id)
}

func TestEnvironmentDeleteFailureKeepsConfirmationAndCanRetry(t *testing.T) {
	svc := fake.New()
	if err := svc.Environments.Save(model.Environment{ID: "a", Name: "A"}); err != nil {
		t.Fatal(err)
	}
	settings := svc.Settings.Get()
	settings.ActiveEnvID = "a"
	if err := svc.Settings.Save(settings); err != nil {
		t.Fatal(err)
	}
	failing := &envFailDelete{EnvironmentService: svc.Environments, failID: "a"}
	svc.Environments = failing
	h := newHarness(t, svc)
	envManage(h)
	envAction(h, "A", h.App.S.Env.Delete)
	h.click(h.App.S.Env.Delete)
	if !h.shows("delete failed") || !h.App.env.deleteOpen || len(svc.Environments.List()) != 1 || svc.Settings.Get() != settings {
		t.Fatal("failed delete lost environment or active selection")
	}
	failing.failID = ""
	h.click(h.App.S.Env.Delete)
	if h.App.env.deleteOpen || len(svc.Environments.List()) != 0 || svc.Settings.Get().ActiveEnvID != "" {
		t.Fatal("delete retry did not finish")
	}
	if !h.shows(h.App.S.Env.Empty) {
		t.Fatal("last deletion did not show empty state")
	}
}

func BenchmarkEnvironmentPaint(b *testing.B) {
	for _, editor := range []bool{false, true} {
		b.Run(fmt.Sprintf("editor=%v", editor), func(b *testing.B) {
			svc := fake.New()
			if err := svc.Environments.Save(model.Environment{ID: "dev", Name: "Development", Variables: []model.KV{{Key: "host", Value: "https://example.test"}}}); err != nil {
				b.Fatal(err)
			}
			a := New(svc)
			if editor {
				a.openEnvironments()
			}
			page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
			config := page.Config(app.Config{Width: 1280, Height: 800})
			d, err := app.NewDriver(config, nil)
			if err != nil {
				b.Fatal(err)
			}
			defer d.Close()
			for range 5 {
				config.Invalidator.Invalidate()
				if err := d.Pump(); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				config.Invalidator.Invalidate()
				if err := d.Pump(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
