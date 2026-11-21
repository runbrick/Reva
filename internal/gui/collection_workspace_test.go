package gui

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func workspaceStore(t *testing.T) *store.Store {
	t.Helper()
	dir, err := storage.Dir()
	authMust(t, err)
	disk, err := store.Open(dir)
	authMust(t, err)
	if errs := disk.LoadErrors(); len(errs) > 0 {
		t.Fatalf("store load errors: %v", errs)
	}
	return disk
}

func startCollectionWorkspace(t *testing.T) *harness {
	t.Helper()
	disk := workspaceStore(t)
	svc := fake.New()
	svc.Collections, svc.Environments = disk.Collections(), disk.Environments()
	svc.Settings, svc.History = disk.Settings(), disk.History()
	svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		t.Error("collection workspace sent a request")
		return model.Response{}, nil
	}
	a := New(svc)
	authMust(t, a.InitializeCollectionWorkspace(a.S.Collections.WorkspaceDefaultName))
	config := a.Config()
	config.Width, config.Height = 960, 600
	driver, err := app.NewDriver(config, nil)
	authMust(t, err)
	t.Cleanup(driver.Close)
	h := &harness{t: t, App: a, Svc: svc, D: driver}
	studioResize(t, h, config.Width, config.Height)
	return h
}

func workspaceInSidebar(h *harness, bounds gfx.Rect) bool {
	return bounds.X >= RailWidth && bounds.X < RailWidth+h.App.sideSplit.Pos && bounds.Y >= TitlebarHeight
}

func workspaceSidebarShows(h *harness, text string) bool {
	var painted strings.Builder
	for _, label := range h.D.Snapshot().Labels {
		if workspaceInSidebar(h, label.Bounds) {
			painted.WriteString(strings.Join(strings.Fields(label.Value), ""))
		}
	}
	return strings.Contains(painted.String(), strings.Join(strings.Fields(text), ""))
}

func workspaceHasSidebarHit(h *harness, label string) bool {
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label && !hit.Container && workspaceInSidebar(h, hit.Bounds) {
			return true
		}
	}
	return false
}

func workspacePick(h *harness, name string) {
	h.t.Helper()
	h.click(h.App.S.Collections.Collection)
	collectionsOption(h, name)
	h.pump()
}

func workspaceAssertFocus(h *harness, name string) {
	h.t.Helper()
	for _, label := range h.D.Snapshot().Labels {
		if label.Bounds.Y < TitlebarHeight && label.Value == name {
			if !workspaceHasSidebarHit(h, name) {
				h.t.Fatalf("%q is in the picker but missing from the sidebar", name)
			}
			return
		}
	}
	h.t.Fatalf("collection picker does not display %q", name)
}

func workspaceFixtureRequest(t *testing.T, disk *store.Store, collection, name string) (model.Collection, model.Request) {
	t.Helper()
	col, err := disk.Collections().Create(collection)
	authMust(t, err)
	request := model.NewRequest(storage.NewID())
	request.Name, request.URL = name, "https://example.invalid/"+request.ID
	request, err = disk.Collections().SaveRequest(col.ID, "", request)
	authMust(t, err)
	return col, request
}

func TestCollectionWorkspaceFreshInstallAndRestart(t *testing.T) {
	for _, locale := range []struct{ language, collection string }{{"zh", "我的集合"}, {"en", "My collection"}} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(locale.language+"/"+theme, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv(storage.EnvHome, dir)
				want := model.DefaultSettings()
				want.Language, want.Theme = locale.language, theme
				if want != model.DefaultSettings() {
					authMust(t, workspaceStore(t).Settings().Save(want))
				}
				h := startCollectionWorkspace(t)
				cols := h.Svc.Collections.List()
				if len(cols) != 1 || cols[0].ID == "" || cols[0].Name != locale.collection || len(cols[0].Items) != 0 {
					t.Fatalf("first launch collections = %+v, want one empty %q", cols, locale.collection)
				}
				if h.App.Dark() != (theme == "dark") || h.App.Section != SectionAPIs || h.App.Main != PageWorkbench {
					t.Fatal("first launch did not show the workbench in the chosen theme")
				}
				workspaceAssertFocus(h, locale.collection)
				onboardingWorkbenchButton(t, h, h.App.S.Settings.GuideWorkbenchOpen)
				if tab := h.App.Active(); len(h.App.Tabs()) != 1 || tab.CollectionID != "" || tab.Dirty || tab.Request.Method != model.MethodGet {
					t.Fatalf("seeding changed the blank startup tab: %+v", tab)
				}
				assertTabsEditorURL(t, h, "")
				want.CollectionWorkspaceInitialized = true
				if got := h.Svc.Settings.Get(); got != want || len(h.Svc.History.List()) != 0 {
					t.Fatalf("seeding changed unrelated settings or history: %+v", got)
				}
				beforeRestart := researchDisk(t, dir)
				h.D.Close()
				h = startCollectionWorkspace(t)
				if !reflect.DeepEqual(h.Svc.Collections.List(), cols) || !reflect.DeepEqual(researchDisk(t, dir), beforeRestart) {
					t.Fatalf("restart changed the seeded store: %+v", h.Svc.Collections.List())
				}
				workspaceAssertFocus(h, locale.collection)
			})
		}
	}
}

func TestCollectionWorkspaceMigratesLegacyStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	disk := workspaceStore(t)
	col, request := workspaceFixtureRequest(t, disk, "Legacy API", "Existing request")
	env, err := disk.Environments().Create("Staging")
	authMust(t, err)
	authMust(t, disk.History().Add(model.HistoryEntry{Request: request, StatusCode: 200}))
	legacy := `{"version":1,"theme":"dark","language":"en","activeEnvId":"` + env.ID + `","onboardingCompleted":true,"historyLimit":17,"requestTimeoutSec":12,"noFollowRedirects":true,"insecureTls":true}`
	authMust(t, os.WriteFile(filepath.Join(dir, "settings.json"), []byte(legacy), 0o600))
	before := researchDisk(t, dir)

	h := startCollectionWorkspace(t)
	workspaceAssertFocus(h, col.Name)
	if !workspaceHasSidebarHit(h, request.Name) {
		t.Fatal("existing request disappeared from the workspace")
	}
	want := model.Settings{
		Version: model.SchemaVersion, Theme: "dark", Language: "en", ActiveEnvID: env.ID,
		OnboardingCompleted: true, HistoryLimit: 17, RequestTimeoutSec: 12, NoFollowRedirects: true, InsecureTLS: true,
		CollectionWorkspaceInitialized: true,
	}
	if got := h.Svc.Settings.Get(); got != want {
		t.Fatalf("migrated settings = %+v, want %+v", got, want)
	}
	after := researchDisk(t, dir)
	before["settings.json"] = after["settings.json"]
	if !reflect.DeepEqual(after, before) {
		t.Fatal("migration created a default collection or modified existing data files")
	}
	h.D.Close()
	h = startCollectionWorkspace(t)
	if h.Svc.Settings.Get() != want || !reflect.DeepEqual(researchDisk(t, dir), after) {
		t.Fatal("reopening migrated data did not preserve settings and files")
	}
}

func TestCollectionWorkspacePickerReusesTabsAndPreservesDrafts(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	disk := workspaceStore(t)
	first, firstRequest := workspaceFixtureRequest(t, disk, "Archive", "Archive listing")
	second, secondRequest := workspaceFixtureRequest(t, disk, "Billing", "Invoice lookup")
	h := startCollectionWorkspace(t)
	text := h.App.S.Collections
	standalone := h.App.Active()
	const standaloneURL = "https://example.invalid/standalone-draft"
	const invoiceURL = "https://example.invalid/invoice-draft"
	requestType(h, h.App.S.Request.URL, standaloneURL, false)
	workspaceAssertFocus(h, first.Name)
	if workspaceHasSidebarHit(h, second.Name) || workspaceHasSidebarHit(h, secondRequest.Name) {
		t.Fatal("unfocused collection is present in the APIs tree")
	}
	h.typeInto(text.WorkspaceFilter, "Invoice")
	if !workspaceSidebarShows(h, text.WorkspaceNoMatches) || workspaceHasSidebarHit(h, firstRequest.Name) || workspaceHasSidebarHit(h, secondRequest.Name) {
		t.Fatal("filter matched a request outside the focused collection")
	}
	workspacePick(h, second.Name)
	workspaceAssertFocus(h, second.Name)
	if h.App.Active() != standalone || workspaceHasSidebarHit(h, first.Name) || !workspaceHasSidebarHit(h, secondRequest.Name) ||
		requestField(h, text.WorkspaceFilter, 0).Value != "" {
		t.Fatal("switching did not replace the tree and reset its filter while retaining the standalone tab")
	}
	assertTabsEditorURL(t, h, standaloneURL)
	h.click(secondRequest.Name)
	invoiceTab := h.App.Active()
	requestType(h, h.App.S.Request.URL, invoiceURL, true)
	workspacePick(h, first.Name)
	assertTabsEditorURL(t, h, standaloneURL)
	h.click(firstRequest.Name)
	archiveTab := h.App.Active()
	if archiveTab.CollectionID != first.ID || archiveTab.Request.ID != firstRequest.ID {
		t.Fatal("clicking the scoped tree did not open the stored archive request")
	}
	workspacePick(h, second.Name)
	if h.App.Active() != invoiceTab || invoiceTab.CollectionID != second.ID || !invoiceTab.Dirty || len(h.App.Tabs()) != 3 {
		t.Fatal("picking a collection did not reactivate its existing edited tab")
	}
	assertTabsEditorURL(t, h, invoiceURL)
	h.click(firstRequest.Name)
	workspaceAssertFocus(h, first.Name)
	if h.App.Active() != archiveTab || h.Svc.Settings.Get().WorkspaceCollectionID != second.ID {
		t.Fatal("activating an existing tab did not focus its collection ahead of the remembered picker selection")
	}
	assertTabsEditorURL(t, h, firstRequest.URL)
	h.D.Close()
	h = startCollectionWorkspace(t)
	workspaceAssertFocus(h, second.Name)
	stored, _ := h.Svc.Collections.Get(second.ID)
	if item := model.FindItem(stored.Items, secondRequest.ID); item == nil || item.Request == nil || item.Request.URL != secondRequest.URL {
		t.Fatal("switching wrote an unsaved edit to disk")
	}
	h.click(text.Collection)
	collectionsOption(h, text.Settings)
	h.pump()
	if tab := h.App.Active(); tab.Kind != collectionSettingsTabKind || tab.CollectionID != second.ID || tab.TargetID != "" {
		t.Fatal("picker settings did not open the focused collection's overview")
	}
}

func TestCollectionWorkspaceGlobalSearchAndSaveAcrossCollections(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	disk := workspaceStore(t)
	first, _ := workspaceFixtureRequest(t, disk, "Commerce", "List accounts")
	second, remote := workspaceFixtureRequest(t, disk, "Inventory", "Find stock")
	folder, err := disk.Collections().AddFolder(second.ID, "", "Incoming")
	authMust(t, err)
	h := startCollectionWorkspace(t)
	workspaceAssertFocus(h, first.Name)
	if workspaceHasSidebarHit(h, remote.Name) {
		t.Fatal("global search fixture must start outside the focused tree")
	}
	h.click(h.App.S.Search.Title)
	searchType(t, h, remote.Name)
	if !searchHasHit(h, remote.Name) {
		t.Fatal("global search hid a request in another collection")
	}
	searchKey(t, h, gfx.KeyEnter)
	workspaceAssertFocus(h, second.Name)
	if tab := h.App.Active(); tab.CollectionID != second.ID || tab.Request.ID != remote.ID {
		t.Fatal("global search did not open the other collection's request")
	}
	workspacePick(h, first.Name)
	workspaceAssertFocus(h, first.Name)
	draft := h.App.Active()
	const draftURL = "https://example.invalid/new-stock"
	requestType(h, h.App.S.Request.URL, draftURL, false)
	h.click(h.App.S.Request.Save)
	text := h.App.S.Collections
	collectionsReplace(h, text.Name, "Create stock")
	collectionsOption(h, text.Collection)
	collectionsOption(h, second.Name)
	collectionsOption(h, text.Folder)
	collectionsOption(h, folder.Name)
	collectionsOption(h, text.Save)
	h.pump()
	if draft.Dirty || draft.CollectionID != second.ID || h.App.Active() != draft {
		t.Fatal("saving into the other collection did not update the active draft")
	}
	workspaceAssertFocus(h, second.Name)
	if !workspaceHasSidebarHit(h, "Create stock") {
		t.Fatal("saved request did not appear in the destination tree")
	}
	requestID := draft.Request.ID
	h.D.Close()
	reopened := workspaceStore(t)
	source, _ := reopened.Collections().Get(first.ID)
	target, _ := reopened.Collections().Get(second.ID)
	if model.FindItem(source.Items, requestID) != nil {
		t.Fatal("save also persisted the request in the originally focused collection")
	}
	parent := model.FindItem(target.Items, folder.ID)
	if parent == nil || len(parent.Items) != 1 || parent.Items[0].ID != requestID || parent.Items[0].Request == nil || parent.Items[0].Request.URL != draftURL {
		t.Fatalf("reopened destination folder did not contain the saved request: %+v", parent)
	}
}

func TestCollectionWorkspaceDeletionFallsBackAndStaysEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	h := startCollectionWorkspace(t)
	seedName := h.App.S.Collections.WorkspaceDefaultName
	text := h.App.S.Collections
	first := collectionsCreate(h, "Archive")
	workspaceAssertFocus(h, first.Name)
	h.click(text.Collection)
	collectionsOption(h, text.NewCollection)
	h.typeInto(text.Name, "Billing")
	collectionsOption(h, text.Create)
	h.pump()
	workspaceAssertFocus(h, "Billing")
	remove := func(name string) {
		collectionsContext(h, name, h.App.S.Collections.Delete)
		collectionsOption(h, h.App.S.Collections.Delete)
		h.pump()
		if workspaceHasSidebarHit(h, name) {
			t.Fatalf("deleted collection %q is still in the tree", name)
		}
	}
	remove("Billing")
	workspaceAssertFocus(h, first.Name)
	h.D.Close()
	h = startCollectionWorkspace(t)
	workspaceAssertFocus(h, first.Name)
	remove(first.Name)
	workspaceAssertFocus(h, seedName)
	remove(seedName)
	assertEmpty := func() {
		text := h.App.S.Collections
		if len(h.Svc.Collections.List()) != 0 || !workspaceSidebarShows(h, text.WorkspaceEmpty) ||
			!workspaceHasSidebarHit(h, text.NewCollection) || !workspaceHasSidebarHit(h, h.App.S.Import.Title) ||
			workspaceHasSidebarHit(h, text.WorkspaceFilter) {
			t.Fatalf("empty workspace did not show create and import actions: %v", h.hitLabels())
		}
	}
	assertEmpty()
	afterDeletion := researchDisk(t, dir)
	h.D.Close()
	h = startCollectionWorkspace(t)
	assertEmpty()
	if !reflect.DeepEqual(researchDisk(t, dir), afterDeletion) {
		t.Fatal("restart recreated the last deleted collection or changed the empty store")
	}
}
