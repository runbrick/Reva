package gui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestImportCurlOpensRequest(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		t.Run(string(lang), func(t *testing.T) {
			svc := fake.New()
			settings := svc.Settings.Get()
			settings.Language = string(lang)
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			h.click(h.App.S.Import.Title)
			h.typeInto(h.App.S.Import.Input, `curl 'https://example.test/users?q=active' -H 'Authorization: Bearer {{token}}' -d '{"name":"你好"}'`)
			reviewImportForTest(h)
			h.click(h.App.S.Import.Submit)
			r := h.App.Active().Request
			if r.URL != "https://example.test/users?q=active" || r.Method != model.MethodPost || r.Body.Mode != model.BodyJSON || r.Body.Text != `{"name":"你好"}` || len(r.Params) != 1 || len(r.Headers) != 1 || r.Headers[0].Key != "Authorization" {
				t.Fatalf("imported request = %#v", r)
			}
			if h.App.importer.open || h.App.Active().CollectionID != "" {
				t.Fatal("curl import should close and open an unsaved request")
			}
		})
	}
}

func TestImportPostmanAddsCollection(t *testing.T) {
	h := newHarness(t)
	h.click(h.App.S.Import.Title)
	h.typeInto(h.App.S.Import.Input, `{"info":{"name":"Imported API"},"item":[{"name":"Ping","request":{"method":"GET","url":"https://example.test/ping"}}]}`)
	reviewImportForTest(h)
	h.click(h.App.S.Import.Submit)
	cols := h.Svc.Collections.List()
	if len(cols) != 1 || cols[0].Name != "Imported API" || len(cols[0].Items) != 1 || cols[0].Items[0].Request.URL != "https://example.test/ping" {
		t.Fatalf("collections = %#v", cols)
	}
	h.pump()
	if h.App.importer.open || !h.shows(h.App.S.Import.Success) {
		t.Fatal("collection import should close and show success")
	}
}

func TestImportInvalidInputStaysOpen(t *testing.T) {
	for _, tc := range []struct {
		name, input string
	}{
		{"unknown", "not an import"}, {"curl", "curl --request POST"},
		{"postman", `{"info":{"name":"Broken"},"item":42}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.click(h.App.S.Import.Title)
			h.typeInto(h.App.S.Import.Input, tc.input)
			reviewImportForTest(h)
			h.pump()
			if !h.App.importer.open || h.App.importer.err == "" || !h.shows(string([]rune(strings.Split(h.App.importer.err, "\n")[0])[:8])) || h.App.importer.input != tc.input {
				t.Fatal("invalid import must keep input and display an inline error")
			}
			if len(h.Svc.Collections.List()) != 0 || h.App.Active().Request.URL != "" {
				t.Fatal("invalid input changed app data")
			}
			h.click(h.App.S.Import.Cancel)
			if h.App.importer.open {
				t.Fatal("cancel did not close dialog")
			}
		})
	}
}

func TestImportJSONFile(t *testing.T) {
	h := newHarness(t)
	path := filepath.Join(t.TempDir(), "collection.json")
	data := `{"info":{"name":"From file"},"item":[]}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.D.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Import.Title)
	h.click(h.App.S.Import.LoadFile)
	h.waitFor("file contents", func() bool { return h.App.importer.input == data })
	request, ok := h.D.LastOpenFilesRequest()
	if !ok || request.AllowMultiple || len(request.Filters) != 3 || request.Filters[0].Pattern != "*.json" || request.Filters[1].Pattern != "*.har" || request.Filters[2].Pattern != "*" {
		t.Fatalf("file picker options = %#v", request)
	}
	reviewImportForTest(h)
	h.click(h.App.S.Import.Submit)
	if cols := h.Svc.Collections.List(); len(cols) != 1 || cols[0].Name != "From file" {
		t.Fatalf("file was not imported: %#v", cols)
	}
}

func TestImportFileErrorsAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths []string
		err   error
	}{
		{"cancel", nil, app.ErrCanceled},
		{"picker failure", nil, errors.New("picker unavailable")},
		{"missing file", []string{filepath.Join(t.TempDir(), "missing.json")}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.click(h.App.S.Import.Title)
			h.typeInto(h.App.S.Import.Input, "keep me")
			if err := h.D.QueueOpenFiles(tc.paths, tc.err); err != nil {
				t.Fatal(err)
			}
			h.click(h.App.S.Import.LoadFile)
			if tc.name != "cancel" {
				h.waitFor("file error", func() bool { return h.App.importer.err != "" })
				h.pump()
				if !h.shows(h.App.S.Import.FileError) {
					t.Fatal("file error not displayed")
				}
			} else if h.App.importer.err != "" {
				t.Fatal("cancellation displayed an error")
			}
			if !h.App.importer.open || h.App.importer.input != "keep me" {
				t.Fatal("file failure discarded input or closed dialog")
			}
		})
	}
}

type failedImportCollections struct{ service.CollectionService }

func (s failedImportCollections) Import(model.Collection) (model.Collection, error) {
	return model.Collection{}, errors.New("disk full")
}

func TestImportCollectionSaveFailureStaysOpen(t *testing.T) {
	svc := fake.New()
	svc.Collections = failedImportCollections{svc.Collections}
	h := newHarness(t, svc)
	h.click(h.App.S.Import.Title)
	h.typeInto(h.App.S.Import.Input, `{"info":{"name":"Fail"},"item":[]}`)
	reviewImportForTest(h)
	h.click(h.App.S.Import.Submit)
	h.pump()
	if !h.App.importer.open || !h.shows(h.App.S.Import.SaveError) || len(h.Svc.Collections.List()) != 0 {
		t.Fatal("save failure should remain open and display error")
	}
}

func BenchmarkImportDialogPaint(b *testing.B) {
	for _, open := range []bool{false, true} {
		name := "closed"
		if open {
			name = "open"
		}
		b.Run(name, func(b *testing.B) {
			a := New(fake.New())
			a.importer.open = open
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

// reviewImportForTest follows the deliberate asynchronous input -> review step.
func reviewImportForTest(h *harness) {
	h.t.Helper()
	h.click(h.App.S.Import.Review)
	h.waitFor("import review", func() bool {
		return !h.App.importer.parsing && (h.App.importer.preview != nil || h.App.importer.err != "")
	})
	h.pump()
}
