package gui

import (
	"errors"
	"fmt"
	"testing"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A direct application mutation has no native input event to request a paint.
func collectionsPaint(h *harness) {
	h.t.Helper()
	if err := h.D.Resize(1280, 800); err != nil {
		h.t.Fatal(err)
	}
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
		h.click(action)
		h.pump()
		return
	}
	h.t.Fatalf("no collection row %q; have %q", name, h.hitLabels())
}

func collectionsCreate(h *harness, name string) model.Collection {
	h.t.Helper()
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
	h.click(h.App.S.Collections.Save)
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
	h.click(h.App.S.Collections.Save)
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
			if h.shows("List invoices") {
				t.Fatal("collapsed folder still shows request")
			}
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			h.pump()
			if !h.shows("List invoices") {
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
	if copy.ID == r.ID || copy.Name != copyName || !h.shows(copyName) {
		t.Fatal("duplicate was not stored with a new ID")
	}
	stored, _ := h.Svc.Collections.Get(col.ID)
	if len(stored.Items) != 2 {
		t.Fatal("duplicate replaced the original")
	}
	collectionsContext(h, copyName, h.App.S.Collections.Delete)
	h.click(h.App.S.Collections.Delete)
	h.pump()
	if h.shows(copyName) || !h.shows("Get payment") {
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
	h.click(h.App.S.Collections.Save)
	h.pump()
	if !h.App.collections.saveOpen || !tab.Dirty || tab.CollectionID != "" || !h.shows(h.App.S.Common.Error) || !h.shows("disk unavailable") {
		t.Fatal("failed save lost edits or did not report error")
	}
	failing.fail = false
	h.click(h.App.S.Collections.Save)
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
