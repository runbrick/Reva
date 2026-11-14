package gui

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/store"
)

func TestOpenAPIVendorExportReviewSaveAndReload(t *testing.T) {
	svc, dir := importRegressionServices(t)
	h := newHarness(t, svc)
	file, err := filepath.Abs("../importer/testdata/openapi-vendor-compat.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.D.QueueOpenFiles([]string{file}, nil); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Import.Title)
	text := h.App.S.Import
	h.click(text.SwaggerSource)
	h.click(text.AddHost)
	h.click(text.AddBasePath)
	h.click(text.LoadFile)
	h.waitFor("vendor export file", func() bool { return !h.App.importer.loading && h.App.importer.input != "" })
	reviewImportForTest(h)
	if h.App.importer.preview == nil {
		t.Fatal(h.App.importer.err)
	}
	requests, folders := importCounts(h.App.importer.preview.Collection.Items)
	if requests != 9 || folders != 1 {
		t.Fatal("review lost operations")
	}
	if len(svc.Collections.List()) != 0 || !importSmokeHit(t, h, text.Submit).Disabled {
		t.Fatal("notices failed to gate import")
	}
	var values []model.Request
	for _, item := range h.App.importer.preview.Collection.Items[0].Items {
		values = append(values, *item.Request)
	}
	importRegressionConfirm(h)
	reloaded, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cols := reloaded.Collections().List()
	if len(cols) != 1 || len(reloaded.LoadErrors()) != 0 {
		t.Fatal("saved export did not reload")
	}
	items := cols[0].Items[0].Items
	if len(items) != len(values) {
		t.Fatal("save lost requests")
	}
	for i, item := range items {
		got, want := item.Request.Clone(), values[i].Clone()
		got.ID = ""
		want.ID = ""
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("save changed request template %d", i)
		}
	}
	if len(svc.History.List()) != 0 {
		t.Fatal("import sent a request")
	}
}
