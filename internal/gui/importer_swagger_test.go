package gui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/vivaui/viva/ui"
)

const guiSwagger = `{"swagger":"2.0","info":{"title":"宠物 API"},"host":"api.example.test","basePath":"/v1","schemes":["https"],"paths":{"/pets/{id}":{"get":{"summary":"宠物详情","parameters":[{"name":"id","in":"path","required":true,"type":"integer","default":42}]}}}}`
const guiOpenAPI = `{"openapi":"3.0.3","info":{"title":"URL API"},"servers":[{"url":"./v2"}],"paths":{"/ping":{"get":{}}}}`

func TestSwaggerFileReviewAppendAndPathOptions(t *testing.T) {
	svc, dir := importRegressionServices(t)
	h := newHarness(t, svc)
	col, err := h.Svc.Collections.Create("Destination")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "swagger.json")
	if err := os.WriteFile(path, []byte(guiSwagger), 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.D.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Import.Title)
	text := h.App.S.Import
	h.click(text.SwaggerSource)
	if importSmokeHit(t, h, text.AddHost).Disabled || importSmokeHit(t, h, text.AddBasePath).Disabled {
		t.Fatal("path options are disabled")
	}
	h.click(text.CurrentCollection)
	h.click(text.AddHost)
	h.click(text.AddBasePath)
	h.click(text.LoadFile)
	h.waitFor("Swagger file", func() bool { return !h.App.importer.loading && h.App.importer.input == guiSwagger })
	reviewImportForTest(h)
	preview := h.App.importer.preview
	if preview == nil {
		t.Fatal(h.App.importer.err)
	}
	if preview.Kind != importer.KindSwagger || preview.Collection.Items[0].Request.URL != "https://api.example.test/v1/pets/{{id}}" {
		t.Fatal(preview)
	}
	existing, _ := h.Svc.Collections.Get(col.ID)
	if len(existing.Items) != 0 || !importSmokeHit(t, h, text.Submit).Disabled {
		t.Fatal("review wrote data or bypassed warning acknowledgement")
	}
	h.click(text.Back)
	if h.App.importer.acknowledged {
		t.Fatal("review acceptance retained")
	}
	h.click(text.AddHost)
	reviewImportForTest(h)
	if h.App.importer.preview.Collection.Items[0].Request.URL != "/v1/pets/{{id}}" {
		t.Fatal("path options did not rebuild requests")
	}
	h.click(text.Acknowledge)
	h.click(text.Submit)
	existing, _ = h.Svc.Collections.Get(col.ID)
	if h.App.importer.open || len(h.Svc.Collections.List()) != 1 || len(existing.Items) != 1 || existing.Items[0].Name != "宠物 API" || existing.Items[0].Items[0].Request.URL != "/v1/pets/{{id}}" || len(existing.Variables) != 1 {
		t.Fatal("Swagger did not append in its own folder")
	}
	reloaded, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	stored, ok := reloaded.Collections().Get(col.ID)
	if !ok || !reflect.DeepEqual(stored, existing) || len(reloaded.LoadErrors()) != 0 {
		t.Fatal("Swagger import did not survive reload")
	}
}

func TestSwaggerSourceValidationAndAutomaticDetection(t *testing.T) {
	h := newHarness(t)
	h.click(h.App.S.Import.Title)
	text := h.App.S.Import
	h.click(text.SwaggerSource)
	importTypeInto(h, "curl https://example.test/mismatch")
	reviewImportForTest(h)
	if h.App.importer.preview != nil || h.App.importer.err != text.SourceMismatch {
		t.Fatal("Swagger accepted curl")
	}
	h.click(text.Input)
	if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Type(guiSwagger); err != nil {
		t.Fatal(err)
	}
	h.click(text.AutoSource)
	reviewImportForTest(h)
	if h.App.importer.preview == nil || h.App.importer.preview.Kind != importer.KindSwagger || h.App.importer.preview.Collection.Items[0].Request.URL != "/pets/{{id}}" {
		t.Fatal("automatic detection or disabled path options failed")
	}
	h.click(text.Cancel)
	if len(h.Svc.Collections.List()) != 0 {
		t.Fatal("cancel persisted Swagger")
	}
}

func TestSwaggerURLFetchReviewRetryAndConfirm(t *testing.T) {
	var calls atomic.Int32
	var failing atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if failing.Load() {
			http.Error(w, "PRIVATE_SERVER_ERROR", http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/spec/api.json", http.StatusFound)
			return
		}
		fmt.Fprint(w, guiOpenAPI)
	}))
	t.Cleanup(server.Close)
	h := newHarness(t)
	h.click(h.App.S.Import.Title)
	text := h.App.S.Import
	h.click(text.SwaggerURLSource)
	h.typeInto(text.SchemaURL, server.URL+"/redirect")
	h.click(text.AddHost)
	h.click(text.AddBasePath)
	if calls.Load() != 0 {
		t.Fatal("fetched before explicit review")
	}
	failing.Store(true)
	reviewImportForTest(h)
	if h.App.importer.preview != nil || !strings.Contains(h.App.importer.err, text.FetchFailed) || strings.Contains(h.App.importer.err, "PRIVATE") || h.App.importer.url != server.URL+"/redirect" {
		t.Fatal("failed fetch discarded input or exposed response")
	}
	failing.Store(false)
	reviewImportForTest(h)
	preview := h.App.importer.preview
	if preview == nil {
		t.Fatal(h.App.importer.err)
	}
	if preview.Kind != importer.KindOpenAPI || preview.Collection.Items[0].Request.URL != server.URL+"/spec/v2/ping" || len(h.Svc.Collections.List()) != 0 {
		t.Fatal("URL resolution failed or review wrote data")
	}
	h.click(text.Acknowledge)
	before := calls.Load()
	h.click(text.Submit)
	cols := h.Svc.Collections.List()
	if h.App.importer.open || len(cols) != 1 || cols[0].Name != "URL API" || calls.Load() != before {
		t.Fatal("confirmation failed or refetched schema")
	}
}

func TestSwaggerURLCancelStopsFetchAndAllowsRetry(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			close(started)
			<-r.Context().Done()
			close(cancelled)
			return
		}
		fmt.Fprint(w, guiOpenAPI)
	}))
	t.Cleanup(server.Close)
	h := newHarness(t)
	h.click(h.App.S.Import.Title)
	text := h.App.S.Import
	h.click(text.SwaggerURLSource)
	h.typeInto(text.SchemaURL, server.URL+"/slow")
	h.click(text.Review)
	h.waitFor("fetch started", func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	h.click(text.Cancel)
	h.waitFor("fetch cancelled", func() bool {
		select {
		case <-cancelled:
			return true
		default:
			return false
		}
	})
	if h.App.importer.preview != nil || len(h.Svc.Collections.List()) != 0 {
		t.Fatal("cancelled fetch published results")
	}
	h.click(text.Title)
	h.click(text.SchemaURL)
	if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Type(server.URL + "/api.json"); err != nil {
		t.Fatal(err)
	}
	reviewImportForTest(h)
	if h.App.importer.preview == nil || h.App.importer.preview.Kind != importer.KindOpenAPI {
		t.Fatal("cancelled URL import could not retry")
	}
}

func TestSwaggerImportBilingualThemesAndLayout(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		for _, theme := range []string{"light", "dark"} {
			for _, size := range [][2]int{{960, 600}, {1280, 800}} {
				t.Run(fmt.Sprintf("%s/%s/%dx%d", lang, theme, size[0], size[1]), func(t *testing.T) {
					svc := fake.New()
					settings := svc.Settings.Get()
					settings.Language = string(lang)
					settings.Theme = theme
					if err := svc.Settings.Save(settings); err != nil {
						t.Fatal(err)
					}
					h := newHarness(t, svc)
					if err := h.D.Resize(size[0], size[1]); err != nil {
						t.Fatal(err)
					}
					h.click(h.App.S.Import.Title)
					text := h.App.S.Import
					for _, label := range []string{text.SwaggerSource, text.SwaggerURLSource} {
						if importSmokeHit(t, h, label).Disabled {
							t.Fatal("Swagger disabled")
						}
					}
					h.click(text.SwaggerURLSource)
					importSmokeLayout(t, h)
					for _, label := range []string{text.SchemaURL, text.Review, text.Cancel} {
						hit := importSmokeHit(t, h, label)
						if hit.Bounds.Y+hit.Bounds.Height > float32(size[1]) {
							t.Fatalf("unreachable %s", label)
						}
					}
					h.click(text.SwaggerSource)
					importTypeInto(h, guiSwagger)
					reviewImportForTest(h)
					importSmokeLayout(t, h)
					if h.App.importer.preview == nil {
						t.Fatal(h.App.importer.err)
					}
					h.click(text.Acknowledge)
					h.click(text.Submit)
					if len(svc.Collections.List()) != 1 {
						t.Fatal("bilingual import failed")
					}
				})
			}
		}
	}
}
