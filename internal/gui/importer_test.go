package gui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
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
			h.click(h.App.S.Import.Submit)
			r := h.App.Active().Request
			if r.URL != "https://example.test/users" || r.Method != model.MethodPost || r.Body.Mode != model.BodyJSON || r.Body.Text != `{"name":"你好"}` || len(r.Params) != 1 || len(r.Headers) != 1 || r.Headers[0].Key != "Authorization" {
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
			h.click(h.App.S.Import.Submit)
			h.pump()
			if !h.App.importer.open || h.App.importer.err == "" || !h.shows(h.App.importer.err) || h.App.importer.input != tc.input {
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
	if !ok || request.AllowMultiple || len(request.Filters) != 1 || request.Filters[0].Pattern != "*.json" {
		t.Fatalf("file picker options = %#v", request)
	}
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
