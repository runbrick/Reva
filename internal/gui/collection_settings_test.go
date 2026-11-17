package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func settingsDraft(h *harness) *collectionSettingsDraft {
	return h.App.collections.settings[h.App.Active().ID]
}

func settingsAssertWorkbench(h *harness) {
	h.t.Helper()
	if h.App.Active().Kind != collectionSettingsTabKind || h.App.Active().Request.ID != "" {
		h.t.Fatal("configuration is not an independent tab with a zero request")
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Request.URL || hit.Label == h.App.S.Request.Send || hit.Label == h.App.S.Shell.SplitLabel {
			h.t.Fatalf("request-only control %q present in configuration", hit.Label)
		}
		if hit.Layer > 0 {
			h.t.Fatal("configuration opened as a dialog")
		}
	}
}

func TestCollectionSettingsMenuSectionsReuseTabAndRetainDraft(t *testing.T) {
	h, col, _, req := authFixture(t)
	writes := &authFailingCollections{CollectionService: h.Svc.Collections}
	h.App.Svc.Collections = writes
	collectionsContext(h, col.Name, h.App.S.Collections.Settings)
	tab := h.App.Active()
	settingsAssertWorkbench(h)
	collectionsReplace(h, h.App.S.Collections.Name, "Draft name")
	collectionsOption(h, h.App.S.Collections.Authentication)
	collectionsOption(h, h.App.S.Request.Basic)
	requestType(h, h.App.S.Request.Password, "credential", true)
	authAssertMasked(h, h.App.S.Request.Password, "credential")
	collectionsOption(h, h.App.S.Collections.Variables)
	collectionsReplace(h, h.App.S.Common.Value, "draft-variable")
	draft := settingsDraft(h)
	for _, page := range []MainPage{PageHistory, PageSettings, PageWorkbench} {
		h.App.Main = page
		requestRepaint(h)
	}
	h.App.active = h.App.requestTabIndex(req.ID)
	requestRepaint(h)
	collectionsContext(h, col.Name, h.App.S.Collections.Authentication)
	if h.App.Active() != tab || len(h.App.Tabs()) != 2 || settingsDraft(h) != draft {
		t.Fatal("opening existing configuration did not focus its independent draft")
	}
	collectionsContext(h, col.Name, h.App.S.Collections.Variables)
	if h.App.Active() != tab || draft.section != collectionSettingsVariables || draft.name != "Draft name" || draft.auth.Password != "credential" || draft.variables[0].Value != "draft-variable" || !tab.Dirty {
		t.Fatal("internal navigation or request/page switching lost draft")
	}
	if writes.writes != 0 || req.Dirty || strings.Contains(h.App.requestTabTitle(tab), "credential") {
		t.Fatal("view/navigation persisted data, dirtied request or leaked credentials")
	}
	stored, _ := writes.Get(col.ID)
	if !reflect.DeepEqual(stored, col) {
		t.Fatal("unsaved configuration changed stored collection")
	}
}

func TestCollectionSettingsUnifiedSaveFreshTreeAndInheritance(t *testing.T) {
	svc, dir := importRegressionServices(t)
	col, err := svc.Collections.Create("Saved collection")
	authMust(t, err)
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{shared}}"}
	col.Variables = []model.KV{{Key: "shared", Value: "before"}}
	authMust(t, svc.Collections.Save(col))
	folder, err := svc.Collections.AddFolder(col.ID, "", "Keep folder")
	authMust(t, err)
	r := model.NewRequest("settings-request")
	r.Name, r.URL, r.Auth = "Keep request", "https://example.test/", model.Auth{Type: model.AuthInherit}
	r, err = svc.Collections.SaveRequest(col.ID, folder.ID, r)
	authMust(t, err)
	h := newHarness(t, svc)
	h.App.OpenRequest(r, col.ID)
	req := h.App.Active()
	tab := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
	requestRepaint(h)
	collectionsReplace(h, h.App.S.Collections.Name, "Renamed together")
	collectionsOption(h, h.App.S.Collections.Authentication)
	collectionsOption(h, h.App.S.Request.Basic)
	requestType(h, h.App.S.Request.Username, "{{shared}}", true)
	requestType(h, h.App.S.Request.Password, "credential", true)
	collectionsOption(h, h.App.S.Collections.Variables)
	collectionsReplace(h, h.App.S.Common.Value, "after")
	cfg, _, err := h.App.effectiveAuth(req)
	authMust(t, err)
	if cfg.Type != model.AuthBearer || h.App.VarsFor(req)[0].Value != "before" {
		t.Fatal("unsaved settings affected live request inheritance")
	}
	added := model.NewRequest("external-request")
	added.Name, added.URL = "Added independently", "https://example.test/new"
	_, err = svc.Collections.SaveRequest(col.ID, folder.ID, added)
	authMust(t, err)
	latest, _ := svc.Collections.Get(col.ID)
	latest.Items = cloneCollectionItems(latest.Items)
	model.FindItem(latest.Items, folder.ID).Name = "Latest folder name"
	model.FindItem(latest.Items, r.ID).Request.Options.TimeoutMs = 19000
	authMust(t, svc.Collections.Save(latest))
	_, err = h.D.ShortcutChord(ui.Primary, 's')
	authMust(t, err)
	h.pump()
	stored, _ := svc.Collections.Get(col.ID)
	if stored.Name != "Renamed together" || stored.Auth.Type != model.AuthBasic || stored.Auth.Password != "credential" || stored.Variables[0].Value != "after" || !reflect.DeepEqual(stored.Items, latest.Items) || tab.Dirty {
		t.Fatal("unified save lost edited fields or independent latest hierarchy")
	}
	cfg, _, err = h.App.effectiveAuth(req)
	authMust(t, err)
	if cfg != stored.Auth || h.App.VarsFor(req)[0].Value != "after" || req.Dirty {
		t.Fatal("saved settings did not become live inheritance, or mutated request")
	}
	reloaded, err := store.Open(dir)
	authMust(t, err)
	onDisk, _ := reloaded.Collections().Get(col.ID)
	if !reflect.DeepEqual(onDisk, stored) {
		t.Fatal("configuration did not persist to real store")
	}
	// Editing just the name must preserve a newer auth/variable update.
	d := settingsDraft(h)
	d.name = "Only the name"
	stored.Auth.Token, stored.Variables[0].Value = "external-token", "external-value"
	authMust(t, svc.Collections.Save(stored))
	h.App.saveCollectionSettings(tab)
	got, _ := svc.Collections.Get(col.ID)
	if got.Name != d.name || got.Auth != stored.Auth || !slices.Equal(got.Variables, stored.Variables) {
		t.Fatal("save overwrote fields unchanged in draft")
	}
}

func TestCollectionSettingsDiscardReloadAndDirtyCloseReopen(t *testing.T) {
	h, col, _, _ := authFixture(t)
	tab := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
	requestRepaint(h)
	collectionsReplace(h, h.App.S.Collections.Name, "Discard this")
	latest, _ := h.Svc.Collections.Get(col.ID)
	latest.Name = "Latest saved name"
	authMust(t, h.Svc.Collections.Save(latest))
	h.click(h.App.S.Collections.DiscardReload)
	if tab.Dirty || settingsDraft(h).name != latest.Name {
		t.Fatal("discard did not reload fresh saved value")
	}
	collectionsReplace(h, h.App.S.Collections.Name, "Close this draft")
	_, err := h.D.ShortcutChord(ui.Primary, 'w')
	authMust(t, err)
	if !h.shows(h.App.S.Tabs.ConfirmTitle) {
		t.Fatal("dirty configuration close lacked confirmation")
	}
	h.click(h.App.S.Tabs.Cancel)
	if h.App.Active() != tab || settingsDraft(h).name != "Close this draft" {
		t.Fatal("close cancel lost draft")
	}
	h.App.closeRequestTabs(tab.ID, false)
	requestRepaint(h)
	collectionsOption(h, h.App.S.Tabs.Discard)
	if _, exists := h.App.collections.settings[tab.ID]; exists {
		t.Fatal("closed configuration retained draft state")
	}
	_, err = h.D.ShortcutChord(ui.Primary|ui.Shift, 't')
	authMust(t, err)
	requestRepaint(h)
	reopened := h.App.Active()
	if reopened.Kind != collectionSettingsTabKind || reopened.ID == tab.ID || reopened.Request.ID != "" || reopened.Dirty || settingsDraft(h).name != latest.Name {
		t.Fatal("reopen generated a request or restored discarded settings")
	}
	h.App.closeRequestTabs(reopened.ID, false)
	if h.App.tabBarState.confirm || len(h.App.collections.settings) != 0 {
		t.Fatal("clean close required confirmation or retained state")
	}
}

func TestCollectionSettingsFolderIndependentDraftAndLatestFields(t *testing.T) {
	h, col, folder, req := authFixture(t)
	root := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
	h.App.collections.settings[root.ID].name = "Root unsaved"
	collectionsContext(h, folder.Name, h.App.S.Collections.FolderSettings)
	tab := h.App.Active()
	if tab == root || tab.TargetID != folder.ID {
		t.Fatal("folder configuration not scoped to target")
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Collections.Variables {
			t.Fatal("folder offered unsupported variables section")
		}
	}
	collectionsOption(h, h.App.S.Collections.Authentication)
	if !h.shows(fmt.Sprintf(h.App.S.Request.AuthFromCollection, h.App.S.Request.Bearer, col.Name)) {
		t.Fatal("folder inheritance preview missing")
	}
	collectionsOption(h, h.App.S.Request.None)
	latest, _ := h.Svc.Collections.Get(col.ID)
	latest.Items = cloneCollectionItems(latest.Items)
	model.FindItem(latest.Items, folder.ID).Name = "Externally renamed folder"
	latest.Auth = model.Auth{Type: model.AuthBasic, Username: "independent"}
	authMust(t, h.Svc.Collections.Save(latest))
	collectionsOption(h, h.App.S.Collections.Save)
	got, _ := h.Svc.Collections.Get(col.ID)
	if got.Name != col.Name || got.Auth != latest.Auth || model.FindItem(got.Items, folder.ID).Name != "Externally renamed folder" || model.FindItem(got.Items, folder.ID).Auth.Type != model.AuthNone || h.App.collections.settings[root.ID].name != "Root unsaved" || req.Dirty {
		t.Fatal("folder save overwrote another draft/latest field")
	}
	if !strings.Contains(h.App.requestTabTitle(tab), "Externally renamed folder") {
		t.Fatal("folder tab title stale after rename")
	}
}

func TestCollectionSettingsNoSendAndRequestOpenPathsKeepDraft(t *testing.T) {
	h, col, _, req := authFixture(t)
	var calls atomic.Int32
	h.Svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		calls.Add(1)
		return model.Response{StatusCode: 200}, nil
	}
	tab := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsAuthentication)
	d := settingsDraft(h)
	d.auth.Token = "unsaved-secret"
	requestRepaint(h)
	authMust(t, h.D.Key(gfx.KeyEnter, gfx.ModifierControl))
	h.App.Send(tab)
	h.pump()
	if calls.Load() != 0 || len(h.Svc.History.List()) != 0 || tab.Sending || h.App.collections.saveOpen {
		t.Fatal("configuration started request send/save flow")
	}
	h.App.OpenRequest(req.Request, col.ID)
	if h.App.Active() != req {
		t.Fatal("existing request not activated")
	}
	for i, open := range []func(){
		func() { h.App.newCollectionTab() },
		func() { r := model.NewRequest("new-request"); r.URL = "https://new.test/"; h.App.OpenRequest(r, "") },
		func() {
			h.App.openHistory(model.HistoryEntry{Request: model.Request{ID: "history", Method: model.MethodGet, URL: "https://history.test/"}}, false)
		},
		func() {
			h.App.runSearch(searchResult{kind: searchRequest, request: model.Request{ID: "search", Method: model.MethodGet, URL: "https://search.test/"}})
		},
	} {
		h.App.active = h.App.requestTabIndex(tab.ID)
		open()
		if h.App.Active() == tab || h.App.Active().Kind != requestTabKind || tab.Kind != collectionSettingsTabKind || tab.Request.ID != "" || h.App.collections.settings[tab.ID] != d || d.auth.Token != "unsaved-secret" {
			t.Fatalf("request open path %d changed/lost configuration", i)
		}
	}
	h.App.active = h.App.requestTabIndex(tab.ID)
	requestRepaint(h)
	// Actual curl import confirmation must use OpenRequest without reusing config.
	importRegressionReview(h, `curl https://import.test/items`)
	importRegressionConfirm(h)
	h.pump()
	if h.App.Active().Kind != requestTabKind || h.App.Active() == tab || d.auth.Token != "unsaved-secret" || calls.Load() != 0 || len(h.Svc.History.List()) != 0 {
		t.Fatal("import overwrote/sent configuration or lost its draft")
	}
}

func TestCollectionSettingsMixedTabsSearchDuplicateReorderAndClose(t *testing.T) {
	h, col, _, first := authFixture(t)
	tab := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
	d := settingsDraft(h)
	d.auth.Password = "credential"
	last := h.App.NewTab()
	last.Request.Name = "Last request"
	h.App.reorderRequestTab(tab.ID, first.ID)
	if h.App.Tabs()[0] != tab || h.App.Active() != last {
		t.Fatal("mixed reorder changed active tab or lost config")
	}
	h.App.active = 0
	requestRepaint(h)
	openTabsContext(t, h, h.App.requestTabTitle(tab))
	h.click(h.App.S.Tabs.Duplicate)
	if h.App.Active() != tab || len(h.App.Tabs()) != 3 || settingsDraft(h) != d {
		t.Fatal("configuration duplicate created a phantom request or reset draft")
	}
	h.App.indexSearch()
	var found searchResult
	for _, row := range h.App.search.rows {
		if row.tabID == tab.ID {
			found = row
			if row.request.ID != "" || strings.Contains(row.name+row.detail, "credential") || row.name != h.App.collectionSettingsTitle(tab) {
				t.Fatal("search indexed configuration as request or leaked credentials")
			}
		}
	}
	if found.tabID == 0 {
		t.Fatal("configuration not searchable")
	}
	h.App.active = 2
	h.App.runSearch(found)
	if h.App.Active() != tab {
		t.Fatal("search did not activate configuration")
	}
	requestRepaint(h)
	authMust(t, h.D.Key(gfx.KeyTab, gfx.ModifierControl))
	if h.App.Active() != first {
		t.Fatal("Ctrl+Tab did not switch from config to request")
	}
	h.App.closeRequestTabs(tab.ID, true)
	if len(h.App.Tabs()) != 1 || h.App.Active() != tab {
		t.Fatal("close others failed across mixed kinds")
	}
	h.App.reopenRequestTab()
	if h.App.Active().Kind != requestTabKind || h.App.Active().Request.ID != last.Request.ID {
		t.Fatal("mixed reopen lost request kind/content")
	}
}

func TestCollectionSettingsMixedOverflowCloseRightAndReopen(t *testing.T) {
	h, col, _, first := authFixture(t)
	first.Request.Name = "Left request tab"
	tab := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
	for i := range 7 {
		h.App.NewTab().Request.Name = fmt.Sprintf("Overflow request %d", i)
	}
	requestRepaint(h)
	requestRepaint(h)
	h.click(h.App.S.Tabs.AllTabs)
	clickTabsContextItem(t, h, h.App.collectionSettingsTitle(tab))
	if h.App.Active() != tab || h.App.tabBarState.menu {
		t.Fatal("overflow menu did not activate mixed configuration tab")
	}
	hit := tabsHit(t, h, h.App.collectionSettingsTitle(tab))
	if hit.Bounds.X < SidebarWidth+RailWidth {
		t.Fatal("overflow selection did not reveal configuration tab")
	}
	// Close-right from a request must confirm a dirty configuration beside it.
	h.App.collections.settings[tab.ID].name = "Dirty configuration"
	h.App.active = h.App.requestTabIndex(first.ID)
	requestRepaint(h)
	openTabsContext(t, h, first.Request.Name)
	h.click(h.App.S.Tabs.CloseRight)
	if !h.shows(h.App.S.Tabs.ConfirmTitle) || h.App.requestTabIndex(tab.ID) < 0 {
		t.Fatal("mixed close-right skipped dirty configuration confirmation")
	}
	collectionsOption(h, h.App.S.Tabs.Discard)
	if len(h.App.Tabs()) != 1 || h.App.Active() != first || len(h.App.collections.settings) != 0 {
		t.Fatal("mixed close-right removed left request or retained configuration")
	}
	// Requests to its right close after the confirmed config, so reopen walks
	// their history before returning to the freshly loaded configuration.
	for range 8 {
		h.App.reopenRequestTab()
	}
	if h.App.Active().Kind != collectionSettingsTabKind || settingsDraft(h).name != col.Name || h.App.Active().Dirty {
		t.Fatal("mixed reopen order lost configuration or restored discarded draft")
	}
}

func TestCollectionSettingsDeletedTargetErrorAndSafeTitles(t *testing.T) {
	h, col, _, _ := authFixture(t)
	tab := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsAuthentication)
	d := settingsDraft(h)
	d.auth.Token = "credential"
	failing := &authFailingCollections{CollectionService: h.Svc.Collections, fail: true}
	h.App.Svc.Collections = failing
	h.App.RequestSave(tab)
	requestRepaint(h)
	if !h.shows(h.App.S.Collections.SettingsSaveFailed) || d.auth.Token != "credential" || !tab.Dirty {
		t.Fatal("save failure did not keep draft and localized error")
	}
	if strings.Contains(h.App.requestTabTitle(tab), "credential") || h.shows("credential-private-backend-error") {
		t.Fatal("credential in title/error")
	}
	failing.fail = false
	h.App.RequestSave(tab)
	if tab.Dirty || d.saveFailed {
		t.Fatal("retry did not clear dirty/error")
	}
	d.name = "Do not recreate"
	authMust(t, failing.Delete(col.ID))
	writes := failing.writes
	requestRepaint(h)
	h.App.RequestSave(tab)
	if !h.shows(h.App.S.Collections.SettingsMissing) || !d.missing || d.name != "Do not recreate" || failing.writes != writes {
		t.Fatal("deleted target was recreated or draft/error lost")
	}
}

func TestCollectionSettingsClearsPreviewAndCompletesInflightDownload(t *testing.T) {
	h, col, _, req := authFixture(t)
	started, release := make(chan struct{}, 1), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	h.Svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, _ model.Request) (model.Response, error) {
		started <- struct{}{}
		select {
		case <-release:
			return model.Response{StatusCode: 200, Body: []byte("download contents")}, nil
		case <-ctx.Done():
			return model.Response{}, ctx.Err()
		}
	}
	h.App.Send(req)
	s := h.App.requestTab(req)
	s.downloadJob, s.downloadPath = req.job, filepath.Join(t.TempDir(), "response.txt")
	requestRepaint(h)
	h.waitFor("started send", func() bool { return len(started) > 0 })
	s.previewOpen = true
	requestRepaint(h)
	h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
	requestRepaint(h)
	if s.previewOpen || h.App.requestPreviewOpen() || h.shows(h.App.S.Request.PreviewTitle) {
		t.Fatal("config inherited hidden request preview")
	}
	settingsAssertWorkbench(h)
	close(release)
	h.waitFor("background request and download", func() bool {
		data, err := os.ReadFile(s.downloadPath)
		return err == nil && string(data) == "download contents" && !req.Sending
	})
	if len(h.Svc.History.List()) != 1 || h.App.Active().Kind != collectionSettingsTabKind {
		t.Fatal("background request history/active config lost")
	}
}

func TestCollectionSettingsPreviewMasksRootFolderAndOverflowTitles(t *testing.T) {
	h, col, folder, req := authFixture(t)
	// Both target names contain a current variable/token value, short enough
	// to remain fully visible in ordinary tabs and overflow entries.
	const canary = "qx7c9"
	col.Name = "Root " + canary
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{shared}}"}
	col.Variables = []model.KV{{Key: "shared", Value: canary}}
	col.Items = cloneCollectionItems(col.Items)
	model.FindItem(col.Items, folder.ID).Name = "Folder " + canary
	authMust(t, h.Svc.Collections.Save(col))
	root := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
	rootDraft := settingsDraft(h)
	rootDraft.name = "Root draft"
	folderTab := h.App.openCollectionSettings(collectionNode{collectionID: col.ID, id: folder.ID}, collectionSettingsAuthentication)
	folderDraft := settingsDraft(h)
	folderDraft.auth = model.Auth{Type: model.AuthBasic, Username: "unsaved user"}
	for i := range 6 {
		h.App.NewTab().Request.Name = fmt.Sprintf("Overflow request %d", i)
	}
	h.App.active = h.App.requestTabIndex(req.ID)
	requestRepaint(h)
	requestRepaint(h)
	rootTitle, folderTitle := h.App.requestTabTitle(root), h.App.requestTabTitle(folderTab)
	if !strings.Contains(rootTitle, canary) || !strings.Contains(folderTitle, canary) {
		t.Fatal("fixture did not expose canary in ordinary configuration titles")
	}
	assertPrivate := func() {
		t.Helper()
		for _, label := range h.D.Snapshot().Labels {
			if strings.Contains(label.Value, canary) {
				t.Fatal("preview painted a configuration-name secret")
			}
		}
		for _, hit := range h.D.Snapshot().Hits {
			if strings.Contains(hit.Label, canary) || strings.Contains(hit.Value, canary) {
				t.Fatal("preview exposed a configuration-name secret in accessibility")
			}
			if hit.Label == h.App.S.Search.Field || hit.Label == rootTitle || hit.Label == folderTitle {
				t.Fatal("preview exposed search palette or raw configuration menu entry")
			}
		}
		for _, tab := range h.App.Tabs() {
			if h.App.requestTabTitle(tab) != h.App.S.Request.PreviewTitle {
				t.Fatal("preview did not mask every mixed tab title")
			}
		}
	}
	h.click(h.App.S.Request.Preview)
	requestRepaint(h)
	assertPrivate()
	// Force an already-open overflow surface to be declared behind preview;
	// inspect actual menu hit names, not just the title helper's return values.
	h.App.tabBarState.menu = true
	requestRepaint(h)
	assertPrivate()
	maskedEntries := 0
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Role == gfx.RoleMenuItem && hit.Label == h.App.S.Request.PreviewTitle {
			maskedEntries++
		}
	}
	if maskedEntries != len(h.App.Tabs()) {
		t.Fatalf("overflow did not declare all masked mixed-tab entries: got %d, want %d", maskedEntries, len(h.App.Tabs()))
	}
	// Even an existing palette state cannot bypass the shell's preview gate.
	h.App.openSearch()
	requestRepaint(h)
	assertPrivate()
	h.App.search.open, h.App.tabBarState.menu = false, false
	h.click(h.App.S.Request.PreviewClose)
	requestRepaint(h)
	if h.App.requestTabTitle(root) != rootTitle || h.App.requestTabTitle(folderTab) != folderTitle {
		t.Fatal("configuration titles did not restore after preview closed")
	}
	if h.App.collections.settings[root.ID] != rootDraft || h.App.collections.settings[folderTab.ID] != folderDraft || rootDraft.name != "Root draft" || folderDraft.auth.Username != "unsaved user" {
		t.Fatal("preview changed independent configuration drafts")
	}
}

func TestCollectionSettingsLanguagesThemesAndMinimumLayout(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, mode := range []string{"light", "dark"} {
			for _, size := range [][2]int{{960, 600}, {1440, 900}} {
				t.Run(fmt.Sprintf("%s/%s/%dx%d", lang, mode, size[0], size[1]), func(t *testing.T) {
					h, col, folder, _ := authFixture(t)
					settings := h.Svc.Settings.Get()
					settings.Language, settings.Theme = lang, mode
					h.App.ApplySettings(settings)
					for _, target := range []collectionNode{{collectionID: col.ID}, {collectionID: col.ID, id: folder.ID}} {
						tab := h.App.openCollectionSettings(target, collectionSettingsGeneral)
						d := settingsDraft(h)
						for _, section := range []collectionSettingsSection{collectionSettingsGeneral, collectionSettingsAuthentication, collectionSettingsVariables} {
							if target.id != "" && section == collectionSettingsVariables {
								continue
							}
							d.section = section
							d.auth = authBenchmarkConfig(model.AuthBasic)
							authMust(t, h.D.Resize(size[0], size[1]))
							settingsAssertWorkbench(h)
							for _, label := range []string{h.App.S.Collections.Save, h.App.S.Collections.DiscardReload, h.App.S.Collections.General, h.App.S.Collections.Authentication} {
								hit := tabsHit(t, h, label)
								if hit.Bounds.X < RailWidth+220 || hit.Bounds.Y < TitlebarHeight+TabBarHeight || hit.Bounds.X+hit.Bounds.Width > float32(size[0])+1 || hit.Bounds.Y+hit.Bounds.Height > float32(size[1])+1 || hit.Bounds.Width <= 0 || hit.Bounds.Height <= 0 {
									t.Fatalf("config control %q outside workbench: %+v", label, hit.Bounds)
								}
							}
							for _, issue := range headless.CheckLayout(h.D.Snapshot()) {
								if slices.Contains([]string{h.App.S.Collections.General, h.App.S.Collections.DiscardReload, h.App.S.Collections.Authentication, h.App.S.Request.Username, h.App.S.Request.Password}, issue.Text) {
									t.Errorf("configuration layout %s: %q (%s)", issue.Kind, issue.Text, issue.Detail)
								}
							}
							if tab.Request.ID != "" || h.App.Dark() != (mode == "dark") {
								t.Fatal("config kind/theme changed")
							}
						}
					}
				})
			}
		}
	}
}
