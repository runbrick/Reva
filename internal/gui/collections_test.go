package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

// A direct application mutation has no native input event to request a paint.
func collectionsPaint(h *harness) {
	h.t.Helper()
	if err := h.D.Resize(1280, 800); err != nil {
		h.t.Fatal(err)
	}
}

func collectionsTreeHasItem(h *harness, itemID string) bool {
	for _, node := range h.App.collections.visible {
		if node.id == itemID {
			return true
		}
	}
	return false
}

func collectionsOption(h *harness, name string) {
	h.t.Helper()
	var selected *gfx.Hit
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == name && !hit.Container && (selected == nil || hit.Layer >= selected.Layer) {
			hit := hit
			selected = &hit
		}
	}
	if selected == nil {
		h.t.Fatalf("option %q missing", name)
	}
	b := selected.Bounds
	if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
		h.t.Fatal(err)
	}
}

func collectionsReplace(h *harness, name, value string) {
	h.t.Helper()
	h.click(name)
	if err := h.D.Key(gfx.KeyHome, 0); err != nil {
		h.t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyEnd, gfx.ModifierShift); err != nil {
		h.t.Fatal(err)
	}
	if err := h.D.Type(value); err != nil {
		h.t.Fatal(err)
	}
}

func collectionsContext(h *harness, name, action string) {
	h.t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label != name || hit.Expandable {
			continue
		}
		b := hit.Bounds
		if err := h.D.ClickButton(b.X+b.Width/2, b.Y+b.Height/2, gfx.PointerButtonSecondary); err != nil {
			h.t.Fatal(err)
		}
		collectionsOption(h, action)
		h.pump()
		return
	}
	h.t.Fatalf("no collection row %q; have %q", name, h.hitLabels())
}

func collectionsCreate(h *harness, name string) model.Collection {
	h.t.Helper()
	h.click(h.App.S.Collections.Add)
	h.click(h.App.S.Collections.NewCollection)
	h.typeInto(h.App.S.Collections.Name, name)
	h.click(h.App.S.Collections.Create)
	h.pump()
	for _, col := range h.Svc.Collections.List() {
		if col.Name == name {
			return col
		}
	}
	h.t.Fatalf("collection %q was not stored", name)
	return model.Collection{}
}

func TestCollectionsCreateCollectionAndNestedFolder(t *testing.T) {
	h := newHarness(t)
	if !h.shows(h.App.S.Collections.Empty) {
		t.Fatal("missing empty state")
	}
	col := collectionsCreate(h, "Payments")
	if !h.shows("Payments") || h.shows(h.App.S.Collections.Empty) {
		t.Fatal("tree did not replace empty state")
	}
	collectionsContext(h, "Payments", h.App.S.Collections.NewFolder)
	h.typeInto(h.App.S.Collections.Name, "Invoices")
	h.click(h.App.S.Collections.Create)
	h.pump()
	collectionsContext(h, "Invoices", h.App.S.Collections.NewFolder)
	h.typeInto(h.App.S.Collections.Name, "Archived")
	h.click(h.App.S.Collections.Create)
	h.pump()
	stored, _ := h.Svc.Collections.Get(col.ID)
	if len(stored.Items) != 1 || stored.Items[0].Name != "Invoices" || len(stored.Items[0].Items) != 1 || stored.Items[0].Items[0].Name != "Archived" {
		t.Fatalf("wrong hierarchy: %+v", stored.Items)
	}
	if !h.shows("Archived") {
		t.Fatal("nested folder missing from tree")
	}
}

func TestCollectionsSaveRequestAndSaveAgainInPlace(t *testing.T) {
	h := newHarness(t)
	col := collectionsCreate(h, "Payments")
	collectionsContext(h, "Payments", h.App.S.Collections.NewFolder)
	h.typeInto(h.App.S.Collections.Name, "Invoices")
	h.click(h.App.S.Collections.Create)
	h.pump()
	tab := h.App.Active()
	tab.Request.URL, tab.Dirty = "https://example.test/invoices?limit=10", true
	h.App.RequestSave(tab)
	collectionsPaint(h)
	if !h.App.collections.saveOpen || h.App.collections.saveName != "example.test/invoices" {
		t.Fatal("save did not open with URL-derived name")
	}
	collectionsReplace(h, h.App.S.Collections.Name, "List invoices")
	h.click(h.App.S.Collections.Folder)
	collectionsOption(h, "Invoices")
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	stored, _ := h.Svc.Collections.Get(col.ID)
	if tab.CollectionID != col.ID || tab.Request.Name != "List invoices" || tab.Dirty || h.App.collections.saveOpen {
		t.Fatalf("tab was not saved: %+v", tab)
	}
	if len(stored.Items[0].Items) != 1 || stored.Items[0].Items[0].Request.URL != tab.Request.URL {
		t.Fatal("request not saved inside selected folder")
	}
	tab.Request.URL, tab.Dirty = "https://example.test/invoices?limit=20", true
	h.App.RequestSave(tab)
	collectionsPaint(h)
	stored, _ = h.Svc.Collections.Get(col.ID)
	if h.App.collections.saveOpen || tab.Dirty || len(stored.Items) != 1 || len(stored.Items[0].Items) != 1 || stored.Items[0].Items[0].Request.URL != tab.Request.URL {
		t.Fatal("second save did not update the same nested request")
	}
	if !h.shows(h.App.S.Collections.Saved) {
		t.Fatal("success toast missing")
	}
}

func TestCollectionsSaveCreatesCollectionInline(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	tab.Request.Name, tab.Dirty = "Health check", true
	h.App.RequestSave(tab)
	collectionsPaint(h)
	h.typeInto(h.App.S.Collections.CollectionName, "Monitoring")
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	cols := h.Svc.Collections.List()
	if len(cols) != 1 || cols[0].Name != "Monitoring" || len(cols[0].Items) != 1 || cols[0].Items[0].Name != "Health check" || tab.CollectionID != cols[0].ID || tab.Dirty {
		t.Fatalf("inline save failed: %+v", cols)
	}
}

func TestCollectionsTreeOpensRequestAndTogglesFolders(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	folder, _ := svc.Collections.AddFolder(col.ID, "", "Invoices")
	r := model.NewRequest("list-invoices")
	r.Name, r.URL, r.Method = "List invoices", "https://example.test/invoices", model.MethodPost
	_, _ = svc.Collections.SaveRequest(col.ID, folder.ID, r)
	h := newHarness(t, svc)
	if !h.shows("POST") {
		t.Fatal("method badge missing")
	}
	h.click("List invoices")
	h.pump()
	if h.App.Active().Request.ID != r.ID || h.App.Active().CollectionID != col.ID || len(h.App.Tabs()) != 1 {
		t.Fatal("choosing tree request did not open it in active tab")
	}
	for i, n := range h.App.collections.visible {
		if n.id != folder.ID {
			continue
		}
		id := h.App.collections.tree.DisclosureID(i)
		for _, hit := range h.D.Snapshot().Hits {
			if hit.ID != id {
				continue
			}
			b := hit.Bounds
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			h.pump()
			if collectionsTreeHasItem(h, r.ID) {
				t.Fatal("collapsed folder still shows request")
			}
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			h.pump()
			if !collectionsTreeHasItem(h, r.ID) {
				t.Fatal("expanded folder did not restore request")
			}
			return
		}
	}
	t.Fatal("folder disclosure missing")
}

func TestCollectionsRenameAndDeleteTreeNodes(t *testing.T) {
	h := newHarness(t)
	col := collectionsCreate(h, "Payments")
	collectionsContext(h, "Payments", h.App.S.Collections.NewFolder)
	h.typeInto(h.App.S.Collections.Name, "Invoices")
	h.click(h.App.S.Collections.Create)
	h.pump()
	collectionsContext(h, "Invoices", h.App.S.Collections.Rename)
	collectionsReplace(h, h.App.S.Collections.Name, "Receipts")
	h.click(h.App.S.Collections.Rename)
	h.pump()
	if h.shows("Invoices") || !h.shows("Receipts") {
		t.Fatal("renamed folder not reflected in tree")
	}
	collectionsContext(h, "Receipts", h.App.S.Collections.Delete)
	if !h.shows(h.App.S.Collections.DeleteTitle) {
		t.Fatal("delete confirmation missing")
	}
	stored, _ := h.Svc.Collections.Get(col.ID)
	if len(stored.Items) != 1 {
		t.Fatal("deleted before confirmation")
	}
	h.click(h.App.S.Collections.Cancel)
	h.pump()
	if !h.shows("Receipts") {
		t.Fatal("cancelled deletion removed folder")
	}
	collectionsContext(h, "Receipts", h.App.S.Collections.Delete)
	h.click(h.App.S.Collections.Delete)
	h.pump()
	stored, _ = h.Svc.Collections.Get(col.ID)
	if len(stored.Items) != 0 || h.shows("Receipts") {
		t.Fatal("confirmed deletion did not update tree")
	}
	collectionsContext(h, "Payments", h.App.S.Collections.Rename)
	collectionsReplace(h, h.App.S.Collections.Name, "Billing")
	h.click(h.App.S.Collections.Rename)
	h.pump()
	if !h.shows("Billing") || h.shows("Payments") {
		t.Fatal("collection rename not reflected in tree")
	}
	collectionsContext(h, "Billing", h.App.S.Collections.Delete)
	h.click(h.App.S.Collections.Delete)
	h.pump()
	if len(h.Svc.Collections.List()) != 0 || !h.shows(h.App.S.Collections.Empty) {
		t.Fatal("deleted collection still shown")
	}
}

func TestCollectionsNewRequestDuplicateRenameAndDelete(t *testing.T) {
	h := newHarness(t)
	col := collectionsCreate(h, "Payments")
	collectionsContext(h, "Payments", h.App.S.Collections.NewRequest)
	r := h.App.Active().Request
	if r.ID == "" || r.Method != model.MethodGet || h.App.Active().CollectionID != col.ID {
		t.Fatal("new request was not saved and opened")
	}
	collectionsContext(h, r.Name, h.App.S.Collections.Rename)
	collectionsReplace(h, h.App.S.Collections.Name, "Get payment")
	h.click(h.App.S.Collections.Rename)
	h.pump()
	if h.App.Active().Request.Name != "Get payment" {
		t.Fatal("open editor retained the old request name")
	}
	collectionsContext(h, "Get payment", h.App.S.Collections.Duplicate)
	copyName := fmt.Sprintf(h.App.S.Collections.CopyName, "Get payment")
	copy := h.App.Active().Request
	if copy.ID == r.ID || copy.Name != copyName || !collectionsTreeHasItem(h, copy.ID) {
		t.Fatal("duplicate was not stored with a new ID")
	}
	stored, _ := h.Svc.Collections.Get(col.ID)
	if len(stored.Items) != 2 {
		t.Fatal("duplicate replaced the original")
	}
	collectionsContext(h, copyName, h.App.S.Collections.Delete)
	h.click(h.App.S.Collections.Delete)
	h.pump()
	if collectionsTreeHasItem(h, copy.ID) || !collectionsTreeHasItem(h, r.ID) {
		t.Fatal("request deletion affected wrong row")
	}
}

func TestCollectionsSavingDeletedRequestOpensDialog(t *testing.T) {
	h := newHarness(t)
	col := collectionsCreate(h, "Payments")
	collectionsContext(h, "Payments", h.App.S.Collections.NewRequest)
	tab := h.App.Active()
	if err := h.Svc.Collections.DeleteItem(col.ID, tab.Request.ID); err != nil {
		t.Fatal(err)
	}
	tab.Dirty = true
	h.App.RequestSave(tab)
	collectionsPaint(h)
	if !h.App.collections.saveOpen || !tab.Dirty {
		t.Fatal("deleted request should ask where to save")
	}
}

type collectionsFailing struct {
	service.CollectionService
	fail bool
}

func (s *collectionsFailing) SaveRequest(colID, parentID string, r model.Request) (model.Request, error) {
	if s.fail {
		return r, errors.New("disk unavailable")
	}
	return s.CollectionService.SaveRequest(colID, parentID, r)
}

func TestCollectionsSaveErrorRetainsDialogAndDirtyRequest(t *testing.T) {
	svc := fake.New()
	failing := &collectionsFailing{CollectionService: svc.Collections, fail: true}
	svc.Collections = failing
	h := newHarness(t, svc)
	tab := h.App.Active()
	tab.Dirty = true
	h.App.RequestSave(tab)
	collectionsPaint(h)
	h.typeInto(h.App.S.Collections.CollectionName, "Payments")
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	if !h.App.collections.saveOpen || !tab.Dirty || tab.CollectionID != "" || !h.shows(h.App.S.Common.Error) || !h.shows("disk unavailable") {
		t.Fatal("failed save lost edits or did not report error")
	}
	failing.fail = false
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	if len(h.Svc.Collections.List()) != 1 || h.App.collections.saveOpen || tab.Dirty {
		t.Fatal("retry failed or created an extra collection")
	}
	tab.Dirty = true
	failing.fail = true
	h.App.RequestSave(tab)
	collectionsPaint(h)
	if !tab.Dirty || h.App.collections.saveOpen {
		t.Fatal("failed in-place save lost dirty state")
	}
}

func TestCollectionsExternalChangesRefreshTree(t *testing.T) {
	h := newHarness(t)
	col, _ := h.Svc.Collections.Create("Imported")
	r := model.NewRequest("imported")
	r.Name = "Imported request"
	_, _ = h.Svc.Collections.SaveRequest(col.ID, "", r)
	collectionsPaint(h)
	if !h.shows("Imported request") {
		t.Fatal("external collection change not shown")
	}
	r.URL = "https://example.test/latest"
	_, _ = h.Svc.Collections.SaveRequest(col.ID, "", r)
	collectionsPaint(h)
	h.click("Imported request")
	if h.App.Active().Request.URL != r.URL {
		t.Fatal("tree opened stale request contents")
	}
}

func BenchmarkCollectionsPanePaint(b *testing.B) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	for i := range 20 {
		r := model.NewRequest(fmt.Sprintf("request-%d", i))
		r.Name = fmt.Sprintf("Request %d", i)
		_, _ = svc.Collections.SaveRequest(col.ID, "", r)
	}
	a := New(svc)
	page := ui.NewPage(ui.PageConfig{View: a.collectionsPane, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: 260, Height: 700}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	for range 5 {
		if err := d.Pump(); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := d.Resize(260, 700); err != nil {
			b.Fatal(err)
		}
	}
}

func collectionsRowHit(h *harness, name string) gfx.Hit {
	h.t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == name && !hit.Expandable && !hit.Container {
			return hit
		}
	}
	h.t.Fatalf("missing row %q; have %q", name, h.hitLabels())
	return gfx.Hit{}
}

func collectionsDrag(h *harness, from, to string) {
	h.t.Helper()
	source, target := collectionsRowHit(h, from).Bounds, collectionsRowHit(h, to).Bounds
	x, y := source.X+source.Width/3, source.Y+source.Height/2
	tx, ty := target.X+target.Width/3, target.Y+target.Height/2
	if err := h.D.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		h.t.Fatal(err)
	}
	for _, step := range []float32{0.5, 1} {
		if err := h.D.Move(x+(tx-x)*step, y+(ty-y)*step); err != nil {
			h.t.Fatal(err)
		}
	}
	if err := h.D.Release(tx, ty, gfx.PointerButtonPrimary); err != nil {
		h.t.Fatal(err)
	}
	h.pump()
}

func TestCollectionsActiveRowAndEllipsisRename(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	r := model.NewRequest("invoice")
	r.Name = "List invoices"
	_, _ = svc.Collections.SaveRequest(col.ID, "", r)
	h := newHarness(t, svc)
	h.click(r.Name)
	h.pump()
	if !collectionsRowHit(h, r.Name).Selected {
		t.Fatal("active request has no selected row highlight")
	}
	h.click(fmt.Sprintf(h.App.S.Collections.Actions, r.Name))
	h.click(h.App.S.Collections.Rename)
	collectionsReplace(h, h.App.S.Collections.Name, "All invoices")
	h.click(h.App.S.Collections.Rename)
	h.pump()
	stored, _ := svc.Collections.Get(col.ID)
	if model.FindItem(stored.Items, r.ID).Name != "All invoices" || h.App.Active().Request.Name != "All invoices" {
		t.Fatal("ellipsis rename did not persist")
	}
	if !collectionsRowHit(h, "All invoices").Selected {
		t.Fatal("rename lost the active highlight")
	}
}

func TestCollectionsDragRequestBetweenFoldersAndCollections(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	first, _ := svc.Collections.AddFolder(col.ID, "", "Invoices")
	second, _ := svc.Collections.AddFolder(col.ID, "", "Receipts")
	other, _ := svc.Collections.Create("Archive")
	r := model.NewRequest("invoice")
	r.Name = "List invoices"
	_, _ = svc.Collections.SaveRequest(col.ID, first.ID, r)
	h := newHarness(t, svc)
	h.click(r.Name)
	collectionsDrag(h, r.Name, "Receipts")
	stored, _ := svc.Collections.Get(col.ID)
	if len(model.FindItem(stored.Items, first.ID).Items) != 0 || model.FindItem(model.FindItem(stored.Items, second.ID).Items, r.ID) == nil {
		t.Fatal("drag did not move request between folders")
	}
	collectionsDrag(h, r.Name, "Archive")
	stored, _ = svc.Collections.Get(col.ID)
	destination, _ := svc.Collections.Get(other.ID)
	if model.FindItem(stored.Items, r.ID) != nil || model.FindItem(destination.Items, r.ID) == nil {
		t.Fatal("drag did not move request between collections")
	}
	if h.App.Active().CollectionID != other.ID {
		t.Fatal("moving request left open editor in original collection")
	}
}

func TestCollectionsRejectFolderDropIntoChild(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	parent, _ := svc.Collections.AddFolder(col.ID, "", "Invoices")
	child, _ := svc.Collections.AddFolder(col.ID, parent.ID, "Archived")
	h := newHarness(t, svc)
	collectionsDrag(h, "Invoices", "Archived")
	stored, _ := svc.Collections.Get(col.ID)
	if len(stored.Items) != 1 || stored.Items[0].ID != parent.ID || len(stored.Items[0].Items) != 1 || stored.Items[0].Items[0].ID != child.ID {
		t.Fatal("invalid drop changed folder hierarchy")
	}
	collectionsDrag(h, "Invoices", "Invoices")
	stored, _ = svc.Collections.Get(col.ID)
	if len(stored.Items) != 1 || stored.Items[0].ID != parent.ID {
		t.Fatal("folder dropped into itself")
	}
}

func TestCollectionsNewRequestMenuAndLanguages(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		t.Run(string(lang), func(t *testing.T) {
			h := newHarness(t)
			settings := h.Svc.Settings.Get()
			settings.Language = string(lang)
			h.App.ApplySettings(settings)
			collectionsPaint(h)
			if !h.shows(h.App.S.Collections.Title) || h.shows(h.App.S.Collections.NewRequest) {
				t.Fatal("localized sidebar missing or removed footer button still visible")
			}
			h.click(h.App.S.Shell.Collections)
			if h.App.Section != SectionCollections || !h.shows(h.App.S.Collections.Overview) {
				t.Fatal("collections rail did not open overview")
			}
			if h.shows(h.App.S.Collections.NewRequest) {
				t.Fatal("removed footer button still visible in empty overview")
			}
			h.click(h.App.S.Shell.APIs)
			before := len(h.App.Tabs())
			h.click(h.App.S.Collections.Add)
			collectionsOption(h, h.App.S.Collections.NewRequest)
			h.pump()
			if len(h.App.Tabs()) != before+1 || h.App.Main != PageWorkbench || h.App.Section != SectionAPIs {
				t.Fatal("creation menu did not open a new workbench tab")
			}
			_, _ = h.Svc.Collections.Create("Payments")
			collectionsPaint(h)
			if !h.shows("Payments") || h.shows(h.App.S.Collections.NewRequest) {
				t.Fatal("collection tree missing or removed footer button still visible")
			}
		})
	}
}

func TestCollectionsFilterKeepsMatchingParents(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	folder, _ := svc.Collections.AddFolder(col.ID, "", "Invoices")
	r := model.NewRequest("invoice")
	r.Name = "List invoices"
	_, _ = svc.Collections.SaveRequest(col.ID, folder.ID, r)
	_, _ = svc.Collections.AddFolder(col.ID, "", "Unrelated")
	h := newHarness(t, svc)
	collectionsDrag(h, "Invoices", "Invoices")
	h.typeInto(h.App.S.Collections.Filter, "LIST")
	h.pump()
	if !h.shows("Payments") || !h.shows("Invoices") || !h.shows(r.Name) || h.shows("Unrelated") {
		t.Fatalf("filter failed to preserve matching request ancestry: %+v", h.App.collections.visible)
	}
	collectionsReplace(h, h.App.S.Collections.Filter, "missing")
	h.pump()
	if !h.shows(h.App.S.Collections.NoMatches) {
		t.Fatal("missing no matches feedback")
	}
	collectionsReplace(h, h.App.S.Collections.Filter, "")
	h.pump()
	if !h.shows("Unrelated") {
		t.Fatal("clearing filter did not restore tree")
	}
}

func TestCollectionsOverviewVariablesSaveCancelAndExport(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	col.Variables = []model.KV{{Key: "host", Value: "before"}}
	_ = svc.Collections.Save(col)
	h := newHarness(t, svc)
	h.click(h.App.S.Shell.Collections)
	if !h.shows(fmt.Sprintf(h.App.S.Collections.RequestCount, 0)) {
		t.Fatal("missing request count")
	}
	h.click(h.App.S.Collections.Variables)
	collectionsReplace(h, h.App.S.Common.Value, "after")
	h.click(h.App.S.Collections.Cancel)
	h.pump()
	stored, _ := svc.Collections.Get(col.ID)
	if stored.Variables[0].Value != "before" {
		t.Fatal("cancel persisted variables")
	}
	h.click(h.App.S.Collections.Variables)
	collectionsReplace(h, h.App.S.Common.Value, "after")
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	stored, _ = svc.Collections.Get(col.ID)
	if stored.Variables[0].Value != "after" || !h.shows(h.App.S.Collections.VariablesSaved) {
		t.Fatal("overview did not save edited variables")
	}
	path := filepath.Join(t.TempDir(), "collection.json")
	if err := h.D.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Collections.Export)
	h.pump()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var exported model.Collection
	if err = json.Unmarshal(data, &exported); err != nil {
		t.Fatal(err)
	}
	if exported.ID != col.ID || exported.Variables[0].Value != "after" {
		t.Fatal("exported collection does not contain saved variables")
	}
	if !h.shows(h.App.S.Collections.Exported) {
		t.Fatal("export feedback missing")
	}
}

func TestCollectionsLayoutBothThemesAndLanguages(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(lang+"/"+theme, func(t *testing.T) {
				h := newHarness(t)
				settings := h.Svc.Settings.Get()
				settings.Language, settings.Theme = lang, theme
				h.App.ApplySettings(settings)
				h.App.sideSplit.Pos = 220
				collectionsPaint(h)
				check := func(section Section) {
					label := h.App.S.Shell.APIs
					if section == SectionCollections {
						label = h.App.S.Shell.Collections
					}
					h.click(label)
					collectionsPaint(h)
					for _, issue := range headless.CheckLayout(h.D.Snapshot()) {
						if issue.Bounds.X >= RailWidth && issue.Bounds.X < RailWidth+220 && issue.Bounds.Y > 48 {
							t.Errorf("section %d: %s: %q (%s)", section, issue.Kind, issue.Text, issue.Detail)
						}
					}
				}
				check(SectionAPIs)
				check(SectionCollections)
				col, _ := h.Svc.Collections.Create("Payments")
				folder, _ := h.Svc.Collections.AddFolder(col.ID, "", "Invoices")
				r := model.NewRequest("invoice")
				r.Name = "List invoices"
				_, _ = h.Svc.Collections.SaveRequest(col.ID, folder.ID, r)
				check(SectionAPIs)
				check(SectionCollections)
			})
		}
	}
}

func TestCollectionsAddMenuUsesSelectedFolder(t *testing.T) {
	h := newHarness(t)
	col := collectionsCreate(h, "Payments")
	h.click("Payments")
	h.click(h.App.S.Collections.Add)
	collectionsOption(h, h.App.S.Collections.NewFolder)
	h.typeInto(h.App.S.Collections.Name, "Invoices")
	h.click(h.App.S.Collections.Create)
	h.pump()
	h.click("Invoices")
	h.click(h.App.S.Collections.Add)
	collectionsOption(h, h.App.S.Collections.NewRequest)
	h.pump()
	stored, _ := h.Svc.Collections.Get(col.ID)
	if len(stored.Items) != 1 || len(stored.Items[0].Items) != 1 || stored.Items[0].Items[0].ID != h.App.Active().Request.ID {
		t.Fatal("creation menu ignored selected folder")
	}
}

func TestCollectionsDragFolderPreservesChildrenAndOpenTab(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	folder, _ := svc.Collections.AddFolder(col.ID, "", "Invoices")
	other, _ := svc.Collections.Create("Archive")
	r := model.NewRequest("invoice")
	r.Name = "List invoices"
	_, _ = svc.Collections.SaveRequest(col.ID, folder.ID, r)
	h := newHarness(t, svc)
	h.click(r.Name)
	collectionsDrag(h, "Invoices", "Archive")
	source, _ := svc.Collections.Get(col.ID)
	target, _ := svc.Collections.Get(other.ID)
	if model.FindItem(source.Items, folder.ID) != nil || model.FindItem(target.Items, r.ID) == nil || h.App.Active().CollectionID != other.ID {
		t.Fatal("folder move lost children or open editor ownership")
	}
}

type collectionsFailingMove struct {
	service.CollectionService
	sourceID string
}

func (s *collectionsFailingMove) Save(col model.Collection) error {
	if col.ID == s.sourceID {
		return errors.New("disk unavailable")
	}
	return s.CollectionService.Save(col)
}

func TestCollectionsFailedCrossCollectionMoveKeepsOriginal(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Payments")
	other, _ := svc.Collections.Create("Archive")
	r := model.NewRequest("invoice")
	r.Name = "List invoices"
	_, _ = svc.Collections.SaveRequest(col.ID, "", r)
	svc.Collections = &collectionsFailingMove{CollectionService: svc.Collections, sourceID: col.ID}
	h := newHarness(t, svc)
	h.click(r.Name)
	collectionsDrag(h, r.Name, "Archive")
	source, _ := svc.Collections.Get(col.ID)
	target, _ := svc.Collections.Get(other.ID)
	if model.FindItem(source.Items, r.ID) == nil || model.FindItem(target.Items, r.ID) != nil || h.App.Active().CollectionID != col.ID {
		t.Fatal("failed move lost or duplicated original request")
	}
	if !h.shows("disk unavailable") {
		t.Fatal("failed move did not report error")
	}
}
