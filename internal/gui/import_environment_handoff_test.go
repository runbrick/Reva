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

	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const handoffCollectionFixture = `{"info":{"name":"Migrated collection","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"event":[{"listen":"prerequest","script":{"exec":["throw new Error('script-marker')"]}}],"variable":[{"key":"fallback","value":"collection-path"}],"item":[{"name":"Migrated folder","item":[{"name":"Migrated request","request":{"url":"{{ base }}/{{fallback}}?marker=private-query","header":[{"key":"X-Example","value":"{{headerOnly}}"}]}},{"name":"Other request","request":{"url":"{{base}}/{{activeOnly}}"}}]}]}`

func TestImportEnvironmentHandoffConfirmAndRetry(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		t.Run(language, func(t *testing.T) {
			var sends atomic.Int32
			routes := make(chan string, 4)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sends.Add(1)
				routes <- r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			svc, dir := importRegressionServices(t)
			active, err := svc.Environments.Create("Prior environment")
			if err != nil {
				t.Fatal(err)
			}
			active.Variables = []model.KV{{Key: "activeOnly", Value: "private-active"}, {Key: "fallback", Value: "disabled-active", Disabled: true}}
			if err := svc.Environments.Save(active); err != nil {
				t.Fatal(err)
			}
			settings := svc.Settings.Get()
			settings.ActiveEnvID, settings.Language = active.ID, language
			if language == "en" {
				settings.Theme = "dark"
			}
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			if err := h.D.Resize(960, 600); err != nil {
				t.Fatal(err)
			}
			h.typeInto(h.App.S.Request.URL, "https://example.test/draft")
			draft := h.App.Active().Request.Clone()
			text, state := h.App.S.Import, &h.App.importer
			before := importRegressionDisk(t, dir)
			importRegressionReview(h, handoffCollectionFixture)
			importSmokeLayout(t, h)
			h.click(text.HandoffOptIn)
			h.click(text.Submit)
			if !importSmokeHit(t, h, text.Submit).Disabled || !reflect.DeepEqual(importRegressionDisk(t, dir), before) || state.environmentHandoff != nil {
				t.Fatal("collection review or opt-in bypassed its confirmation gate")
			}
			h.click(text.Acknowledge)
			restore := blockHandoffSave(t, dir, "collections")
			h.click(text.Submit)
			restore()
			if !state.open || state.environmentHandoff != nil || !state.continueWithEnvironment || state.input != handoffCollectionFixture || !h.shows(text.SaveError) || !reflect.DeepEqual(importRegressionDisk(t, dir), before) {
				t.Fatal("collection save failure advanced the handoff or lost the review")
			}
			h.click(text.Submit)
			h.pump()
			importSmokeLayout(t, h)
			if !state.open || !h.shows(text.HandoffTitle) || !h.shows(text.HandoffActiveScope+" · base") || state.input != "" || state.preview != nil || state.acknowledged || len(svc.Collections.List()) != 1 || len(svc.Environments.List()) != 1 {
				t.Fatal("collection confirmation did not open a fresh environment setup")
			}
			for _, hit := range h.hitLabels() {
				if hit == text.SwaggerURLSource || hit == text.CurrentCollection || hit == text.NewCollection {
					t.Fatal("environment setup exposed general import source or destination controls")
				}
			}
			afterCollection := importRegressionDisk(t, dir)
			h.typeInto(text.Input, handoffCollectionFixture)
			reviewImportForTest(h)
			if state.preview != nil || !h.shows(text.HandoffEnvironmentOnly) || !reflect.DeepEqual(importRegressionDisk(t, dir), afterCollection) {
				t.Fatal("handoff accepted a second collection or changed storage")
			}
			var envInput string
			if language == "en" {
				envInput = fmt.Sprintf(`{"_postman_variable_scope":"environment","name":"New environment","values":[{"key":"base","value":%q},{"key":"fallback","value":"disabled-candidate","enabled":false},{"key":"unused","value":"{{$dynamic}}"}]}`, server.URL)
				path := filepath.Join(t.TempDir(), "environment.json")
				if err := os.WriteFile(path, []byte(envInput), 0600); err != nil {
					t.Fatal(err)
				}
				if err := h.D.QueueOpenFiles([]string{path}, nil); err != nil {
					t.Fatal(err)
				}
				h.click(text.LoadFile)
				h.waitFor("environment file", func() bool { return !state.loading && state.input == envInput })
			} else {
				envInput = fmt.Sprintf(`{"version":1,"id":%q,"name":"New environment","variables":[{"key":"base","value":%q},{"key":"fallback","value":"disabled-candidate","disabled":true}]}`, active.ID, server.URL)
				h.click(text.Input)
				if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
					t.Fatal(err)
				}
				if err := h.D.Type(envInput); err != nil {
					t.Fatal(err)
				}
			}
			reviewImportForTest(h)
			importSmokeLayout(t, h)
			if state.preview == nil || state.preview.Environment == nil || !h.shows(text.HandoffCandidateScope+" · activeOnly") || !h.shows(text.HandoffSelectionHelp) || !reflect.DeepEqual(importRegressionDisk(t, dir), afterCollection) {
				t.Fatal("environment review missed the candidate scope or changed storage")
			}
			painted := previewLabels(h)
			for _, private := range []string{server.URL, "private-active", "collection-path", "private-query", "headerOnly", "script-marker", "disabled-candidate"} {
				if strings.Contains(painted, private) {
					t.Fatal("handoff review exposed a value or request context")
				}
			}
			if language == "en" {
				h.click(text.Submit)
				if state.acknowledged || !importSmokeHit(t, h, text.Submit).Disabled || len(svc.Environments.List()) != 1 {
					t.Fatal("environment reused the collection's acknowledgement")
				}
				h.click(text.Acknowledge)
			}
			preview, fileName := state.preview, state.fileName
			restore = blockHandoffSave(t, dir, "environments")
			h.click(text.Submit)
			restore()
			importSmokeLayout(t, h)
			if !state.open || state.preview != preview || state.input != envInput || state.fileName != fileName || !h.shows(text.SaveError) || !reflect.DeepEqual(importRegressionDisk(t, dir), afterCollection) {
				t.Fatal("environment save failure lost its source or touched an existing entity")
			}
			h.click(text.Submit)
			h.pump()
			envs, collections := svc.Environments.List(), svc.Collections.List()
			if state.open || state.environmentHandoff != nil || state.input != "" || len(envs) != 2 || len(collections) != 1 || svc.Settings.Get() != settings || len(h.App.tabs) != 1 || !reflect.DeepEqual(h.App.Active().Request, draft) || sends.Load() != 0 || len(svc.History.List()) != 0 {
				t.Fatal("environment confirmation wrote unintended data, opened a tab or sent a request")
			}
			existing, _ := svc.Environments.Get(active.ID)
			if !reflect.DeepEqual(existing, active) || envs[0].Name != "New environment" || envs[0].ID == active.ID {
				t.Fatal("environment import did not create a standalone entity with a fresh ID")
			}
			afterEnvironment := importRegressionDisk(t, dir)
			for path, data := range afterCollection {
				if afterEnvironment[path] != data {
					t.Fatal("environment confirmation rewrote existing bytes")
				}
			}
			request := collections[0].Items[0].Items[0].Request
			if request.URL != "{{ base }}/{{fallback}}" || len(request.Params) != 1 || request.Params[0].Key != "marker" || request.Params[0].Value != "private-query" || request.PreScript != "" {
				t.Fatal("import rewrote the URL or retained a Postman script")
			}
			h.click(h.App.S.Env.Title)
			collectionsOption(h, "New environment")
			h.click("Migrated request")
			h.click(h.App.S.Request.Preview)
			if !h.shows(h.App.S.Request.PreviewTitle) || svc.Settings.Get().ActiveEnvID != envs[0].ID || sends.Load() != 0 {
				t.Fatal("manual environment selection or pre-send preview did not behave independently")
			}
			h.click(h.App.S.Request.PreviewClose)
			h.click(h.App.S.Request.Send)
			tab := h.App.Active()
			h.waitFor("explicit loopback request", func() bool { return !tab.Sending })
			if tab.Response == nil || tab.Response.StatusCode != http.StatusNoContent || sends.Load() != 1 {
				t.Fatal("explicit send did not use the selected environment")
			}
			if route := <-routes; route != "/collection-path" || tab.Request.URL != request.URL {
				t.Fatal("disabled environment row did not fall back or send modified the draft")
			}
		})
	}
}

func blockHandoffSave(t *testing.T, dir, folder string) func() {
	t.Helper()
	path, retained := filepath.Join(dir, folder), filepath.Join(dir, folder+"-retained")
	if err := os.Rename(path, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("local blocker"), 0600); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(retained, path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestImportEnvironmentHandoffDismissAndResumeOrdinaryImport(t *testing.T) {
	for _, method := range []string{"cancel", "escape", "close"} {
		t.Run(method, func(t *testing.T) {
			svc, dir := importRegressionServices(t)
			h := newHarness(t, svc)
			if err := h.D.Resize(960, 600); err != nil {
				t.Fatal(err)
			}
			text, state := h.App.S.Import, &h.App.importer
			importRegressionReview(h, smokeCleanCollection)
			h.click(text.HandoffOptIn)
			h.click(text.Submit)
			h.pump()
			before := importRegressionDisk(t, dir)
			h.typeInto(text.Input, `{"version":1,"name":"Cancelled environment","variables":[]}`)
			reviewImportForTest(h)
			switch method {
			case "cancel":
				h.click(text.Cancel)
			case "escape":
				if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
			case "close":
				h.click("Close dialog")
			}
			if state.open || state.environmentHandoff != nil || state.continueWithEnvironment || state.input != "" || state.preview != nil || len(svc.Collections.List()) != 1 || !reflect.DeepEqual(importRegressionDisk(t, dir), before) {
				t.Fatal("dismissing an environment review lost the collection or retained the handoff")
			}
			collection := svc.Collections.List()[0]
			h.click(text.Title)
			h.click(text.CurrentCollection)
			importTypeInto(h, `{"info":{"name":"Appended"},"item":[]}`)
			reviewImportForTest(h)
			for _, label := range h.hitLabels() {
				if label == text.HandoffOptIn {
					t.Fatal("existing collection append offered the environment handoff")
				}
			}
			importRegressionConfirm(h)
			stored, _ := svc.Collections.Get(collection.ID)
			if len(svc.Collections.List()) != 1 || len(stored.Items) != len(collection.Items)+1 || len(svc.Environments.List()) != 0 || svc.Settings.Get().ActiveEnvID != "" {
				t.Fatal("ordinary append after cancellation did not use the requested destination")
			}
		})
	}
}

func TestImportEnvironmentHandoffBoundsMissingURLNames(t *testing.T) {
	collection := model.Collection{Variables: []model.KV{{Key: "fallback", Value: "{{nested}}"}, {Key: "disabled", Disabled: true}}, Items: []model.Item{
		{Type: model.ItemRequest, Request: &model.Request{URL: "{{ available }}{{fallback}}{{disabled}}{{disabled}}{{http://private/path}}{{" + strings.Repeat("x", 41) + "}}{{second}}"}},
		{Type: model.ItemFolder, Items: []model.Item{{Type: model.ItemRequest, Request: &model.Request{URL: "{{second}}{{third}}{{fourth}}{{fifth}}{{sixth}}"}}}},
	}}
	environment := model.Environment{Variables: []model.KV{{Key: "available", Value: ""}, {Key: "fallback", Disabled: true}}}
	got := importMissingURLNames(collection, environment)
	if !got.omitted || !reflect.DeepEqual(got.names, []string{"disabled", "second", "third", "fourth", "fifth"}) {
		t.Fatalf("unexpected bounded advisory %#v", got)
	}
}
