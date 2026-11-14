package gui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	requestservice "github.com/runbrick/reva/internal/service/request"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/vivaui/viva/ui"
)

func importRegressionServices(t *testing.T) (service.Services, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	settings := s.Settings().Get()
	settings.Language, settings.OnboardingCompleted = "en", true
	if err := s.Settings().Save(settings); err != nil {
		t.Fatal(err)
	}
	return service.Services{Collections: s.Collections(), Environments: s.Environments(), Settings: s.Settings(), History: s.History(), Requests: requestservice.New()}, dir
}

func importRegressionReview(h *harness, input string) {
	h.t.Helper()
	h.click(h.App.S.Import.Title)
	h.typeInto(h.App.S.Import.Input, input)
	reviewImportForTest(h)
	if h.App.importer.preview == nil {
		h.t.Fatal("supported fixture did not reach review")
	}
}

func importRegressionConfirm(h *harness) {
	h.t.Helper()
	if len(h.App.importer.preview.Warnings) > 0 {
		h.click(h.App.S.Import.Acknowledge)
	}
	h.click(h.App.S.Import.Submit)
	if h.App.importer.open {
		h.t.Fatal("confirmed import did not close")
	}
	h.pump()
}

func importRegressionDisk(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".tmp") {
			t.Error("orphan temporary import file")
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		files[rel] = string(data)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestImportDiskFailureRetryAndReload(t *testing.T) {
	for _, kind := range []string{"collection", "environment"} {
		t.Run(kind, func(t *testing.T) {
			svc, dir := importRegressionServices(t)
			existingCol, err := svc.Collections.Create("Collision")
			if err != nil {
				t.Fatal(err)
			}
			existingEnv, err := svc.Environments.Create("Collision")
			if err != nil {
				t.Fatal(err)
			}
			existingEnv.Variables = []model.KV{{Key: "prior", Value: "must remain"}}
			if err := svc.Environments.Save(existingEnv); err != nil {
				t.Fatal(err)
			}
			settings := svc.Settings.Get()
			settings.ActiveEnvID, settings.Theme, settings.HistoryLimit = existingEnv.ID, "dark", 37
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			if err := svc.History.Add(model.HistoryEntry{Request: model.NewRequest("prior")}); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			h.typeInto(h.App.S.Request.URL, "https://example.test/unsaved-draft")
			draft := h.App.Active().Request.Clone()
			beforeCols, beforeEnvs, beforeHistory := svc.Collections.List(), svc.Environments.List(), svc.History.List()
			before := importRegressionDisk(t, dir)
			input := `{"version":1,"id":"` + existingCol.ID + `","name":"Collision","items":[{"type":"folder","id":"source","name":"Empty"}],"variables":[{"key":"v","value":"complete payload","description":"kept","disabled":true}]}`
			sub := "collections"
			if kind == "environment" {
				sub = "environments"
				input = `{"version":1,"id":"` + existingEnv.ID + `","name":"Collision","variables":[{"key":"v","value":"complete payload","description":"kept","disabled":true}]}`
			}
			importRegressionReview(h, input)
			preview := h.App.importer.preview
			if !reflect.DeepEqual(importRegressionDisk(t, dir), before) || !reflect.DeepEqual(h.App.Active().Request, draft) {
				t.Fatal("review wrote files or changed an existing draft")
			}
			// A file in place of the target directory makes a real atomic save
			// fail on Windows and Unix, independently of permission/ACL behavior.
			path, retained := filepath.Join(dir, sub), filepath.Join(dir, sub+"-retained")
			if err := os.Rename(path, retained); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("local blocker"), 0600); err != nil {
				t.Fatal(err)
			}
			h.click(h.App.S.Import.Submit)
			if !h.App.importer.open || h.App.importer.preview != preview || h.App.importer.input != input || !h.shows(h.App.S.Import.SaveError) {
				t.Fatal("actual disk failure discarded review/input or prevented retry")
			}
			if !reflect.DeepEqual(svc.Collections.List(), beforeCols) || !reflect.DeepEqual(svc.Environments.List(), beforeEnvs) || svc.Settings.Get() != settings || !reflect.DeepEqual(svc.History.List(), beforeHistory) {
				t.Fatal("failed import created partial/empty data or changed existing services")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(retained, path); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(importRegressionDisk(t, dir), before) {
				t.Fatal("failed import left orphan data or changed prior bytes")
			}
			h.click(h.App.S.Import.Submit)
			h.pump()
			if h.App.importer.open || len(svc.Collections.List())+len(svc.Environments.List()) != 3 || len(h.App.tabs) != 1 || !reflect.DeepEqual(h.App.Active().Request, draft) || svc.Settings.Get() != settings || !reflect.DeepEqual(svc.History.List(), beforeHistory) {
				t.Fatal("retry did not save one entity while preserving draft/preferences/history")
			}
			if kind == "environment" {
				for _, imported := range svc.Environments.List() {
					if imported.ID != existingEnv.ID && (imported.ID == preview.Environment.ID || imported.ID == "" || imported.Version != 1 || !reflect.DeepEqual(imported.Variables, preview.Environment.Variables)) {
						t.Fatal("environment save did not assign a fresh identity and retain full reviewed content")
					}
				}
			} else {
				for _, imported := range svc.Collections.List() {
					if imported.ID != existingCol.ID && (imported.ID == preview.Collection.ID || imported.ID == "" || len(imported.Items) != 1 || !reflect.DeepEqual(imported.Variables, preview.Collection.Variables)) {
						t.Fatal("collection retry did not assign a fresh identity and retain full reviewed content")
					}
				}
			}
			// Reusing the old confirmation callback must not import twice.
			h.App.performImport()
			after := importRegressionDisk(t, dir)
			if len(after) != len(before)+1 {
				t.Fatal("retry or duplicate submit left an extra empty/orphan entity")
			}
			for path, data := range before {
				if after[path] != data {
					t.Fatal("retry overwrote an existing entity/settings/history")
				}
			}
			reloaded, err := store.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(reloaded.LoadErrors()) != 0 || !reflect.DeepEqual(reloaded.Collections().List(), svc.Collections.List()) || !reflect.DeepEqual(reloaded.Environments().List(), svc.Environments.List()) || reloaded.Settings().Get() != settings {
				t.Fatal("successful import did not survive storage reload")
			}
			svc.Collections, svc.Environments, svc.Settings, svc.History = reloaded.Collections(), reloaded.Environments(), reloaded.Settings(), reloaded.History()
			reopened := newHarness(t, svc)
			if reopened.App.Svc.Settings.Get().ActiveEnvID != existingEnv.ID || len(reopened.App.Svc.Collections.List())+len(reopened.App.Svc.Environments.List()) != 3 {
				t.Fatal("app recreation lost imported data or activated an imported environment")
			}
		})
	}
}

func TestImportCancelledOldParseCannotReplaceNewReview(t *testing.T) {
	h := newHarness(t)
	oldInput := `{"info":{"name":"Old"},"item":[]}`
	newInput := `{"info":{"name":"New"},"item":[],"event":[{}]}`
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	h.App.importer.parse = ui.NewEffectKeyed(func(ctx context.Context, job *importParseJob) (importer.Result, error) {
		if job.input == oldInput {
			close(started)
			<-release // deliberately finish after cancellation and the replacement
			defer close(finished)
		}
		return importer.Parse(job.input)
	})
	h.click(h.App.S.Import.Title)
	h.typeInto(h.App.S.Import.Input, oldInput)
	h.click(h.App.S.Import.Review)
	h.waitFor("old parse start", func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	h.click(h.App.S.Import.Cancel)
	h.click(h.App.S.Import.Title)
	// Replace retained input through the actual editor selection shortcut.
	h.click(h.App.S.Import.Input)
	if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Type(newInput); err != nil {
		t.Fatal(err)
	}
	reviewImportForTest(h)
	if h.App.importer.preview == nil || h.App.importer.preview.Collection.Name != "New" || h.App.importer.acknowledged {
		t.Fatal("replacement input did not get an independent unacknowledged review")
	}
	preview := h.App.importer.preview
	close(release)
	h.waitFor("old parse finish", func() bool {
		select {
		case <-finished:
			return true
		default:
			return false
		}
	})
	requestRepaint(h)
	if h.App.importer.preview != preview || h.App.importer.input != newInput || len(h.Svc.Collections.List()) != 0 || len(h.App.tabs) != 1 || !importSmokeHit(t, h, h.App.S.Import.Submit).Disabled {
		t.Fatal("cancelled stale parse replaced/accepted/saved the newer review")
	}
	h.click(h.App.S.Import.Acknowledge)
	h.click(h.App.S.Import.Back)
	reviewImportForTest(h)
	if h.App.importer.acknowledged || !importSmokeHit(t, h, h.App.S.Import.Submit).Disabled {
		t.Fatal("editing reused acknowledgement from the previous review")
	}
	h.click(h.App.S.Import.Cancel)
}

func TestImportSameFileRepairAfterBlockingError(t *testing.T) {
	svc, dir := importRegressionServices(t)
	h := newHarness(t, svc)
	path := filepath.Join(t.TempDir(), "synthetic-postman.json")
	inputs := []string{
		`{"info":{"name":"Repair"},"item":[{"name":"R","request":{"url":"https://example.test","auth":{"type":"oauth2","oauth2":[{"key":"token","value":"source-secret-regression"}]}}}]}`,
		`{"info":{"name":"Repair"},"item":[{"name":"R","request":{"url":"https://example.test","auth":{"type":"noauth"}}}]}`,
	}
	before := importRegressionDisk(t, dir)
	h.click(h.App.S.Import.Title)
	for i, input := range inputs {
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		if err := h.D.QueueOpenFiles([]string{path}, nil); err != nil {
			t.Fatal(err)
		}
		h.click(h.App.S.Import.LoadFile)
		h.waitFor("same path reread", func() bool { return !h.App.importer.loading && h.App.importer.input == input })
		reviewImportForTest(h)
		if i == 0 && (h.App.importer.preview != nil || !strings.Contains(h.App.importer.err, h.App.S.Import.UnsupportedAuth) || strings.Contains(h.App.importer.err, "source-secret-regression")) {
			t.Fatal("blocking error offered review or leaked its source credential")
		}
		if !reflect.DeepEqual(importRegressionDisk(t, dir), before) || len(h.App.tabs) != 1 {
			t.Fatal("file load or review mutated application data")
		}
	}
	importRegressionConfirm(h)
	if len(svc.Collections.List()) != 1 || len(svc.Environments.List()) != 0 || len(svc.History.List()) != 0 {
		t.Fatal("same-file repair did not import exactly one collection")
	}
}

func TestImportFileByteBoundaryAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bounded-fixture.txt")
	base := " \n\ufeff" + `{"version":1,"name":"中文环境","variables":[]}`
	// Count original UTF-8 bytes, including BOM and padding. The picker path
	// and paste parser must accept the same exact maximum without truncation.
	input := base + strings.Repeat(" ", (32<<20)-len(base))
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	text, err := readImportFile(context.Background(), &importFile{path: path})
	if err != nil || text != input {
		t.Fatal("exactly 32 MiB UTF-8/BOM file was rejected or truncated")
	}
	if result, err := importer.Parse(text); err != nil || result.Environment == nil || result.Environment.Name != "中文环境" {
		t.Fatal("file/paste input boundaries disagree")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := f.WriteString(" ")
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatal("could not extend local boundary fixture")
	}
	if text, err := readImportFile(context.Background(), &importFile{path: path}); err == nil || text != "" {
		t.Fatal("32 MiB plus one byte yielded accepted/partial file content")
	} else {
		var failure *importer.ParseError
		if !errors.As(err, &failure) || failure.Code != "input_too_large" {
			t.Fatal("oversized file did not return the localized size diagnostic code")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if text, err := readImportFile(ctx, &importFile{path: path}); text != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled file read returned input or attempted work")
	}
}

func TestImportedAuthSendPreservesHierarchy(t *testing.T) {
	type observed struct {
		authorization, apiKey []string
		fallback              string
		query                 url.Values
	}
	requests := make(chan observed, 20)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		requests <- observed{authorization: r.Header.Values("Authorization"), apiKey: r.Header.Values("X-Api-Key"), fallback: r.Header.Get("X-Fallback"), query: r.URL.Query()}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	svc, _ := importRegressionServices(t)
	h := newHarness(t, svc)
	// Synthetic Postman-compatible data: no claim of a certified vendor export.
	input := `{"info":{"name":"Migrated auth","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"auth":{"type":"bearer","bearer":[{"key":"token","value":"{{token}}"}]},"variable":[{"key":"base","value":"` + server.URL + `"},{"key":"token","value":"collection-token"},{"key":"fallback","value":"collection-fallback"}],"item":[{"name":"Shared parent","item":[{"name":"Live inherited","request":{"url":{"raw":"{{base}}/live?key=old&key=older&q=a&q=a&q=b&skip=off","query":[{"key":"key","value":"old"},{"key":"key","value":"older"},{"key":"q","value":"a"},{"key":"q","value":"a"},{"key":"q","value":"b"},{"key":"skip","value":"off","disabled":true}]},"header":[{"key":"Authorization","value":"disabled","disabled":true},{"key":"X-Api-Key","value":"disabled","disabled":true},{"key":"X-Fallback","value":"{{fallback}}"}]}},{"name":"Explicit headers","request":{"url":"{{base}}/headers","header":[{"key":"authorization","value":""},{"key":"AUTHORIZATION","value":"manual"},{"key":"Authorization","value":"disabled","disabled":true},{"key":"x-api-key","value":""},{"key":"X-API-KEY","value":"manual-key"},{"key":"X-Api-Key","value":"disabled","disabled":true}]}},{"name":"Explicit none","request":{"url":"{{base}}/none","auth":{"type":"noauth"}}}]},{"name":"None barrier","auth":{"type":"noauth"},"item":[{"name":"Barrier request","request":{"url":"{{base}}/barrier"}}]}]}`
	importRegressionReview(h, input)
	if calls.Load() != 0 || len(svc.Collections.List()) != 0 {
		t.Fatal("review sent or saved an imported request")
	}
	importRegressionConfirm(h)
	envInput := `{"_postman_variable_scope":"environment","name":"Imported environment","values":[{"key":"token","value":"old-env-token"},{"key":"token","value":"env-token","description":"last enabled wins"},{"key":"token","value":"disabled-token","enabled":false},{"key":"fallback","value":"disabled-fallback","enabled":false},{"key":"user","value":"env-user"},{"key":"password","value":"env-password"}]}`
	importRegressionReview(h, envInput)
	importRegressionConfirm(h)
	if svc.Settings.Get().ActiveEnvID != "" || calls.Load() != 0 || len(svc.History.List()) != 0 {
		t.Fatal("import activated an environment or performed a send")
	}
	h.click(h.App.S.Env.Title)
	collectionsOption(h, "Imported environment")
	if svc.Settings.Get().ActiveEnvID != svc.Environments.List()[0].ID {
		t.Fatal("explicit GUI environment selection failed")
	}
	h.click("Live inherited")
	live := h.App.Active()
	beforeLive := live.Request.Clone()
	send := func() observed {
		t.Helper()
		tab := h.App.Active()
		before := tab.Request.Clone()
		h.click(h.App.S.Request.Send)
		h.waitFor("local imported HTTP response", func() bool { return !tab.Sending })
		if tab.Response == nil || tab.Response.StatusCode != http.StatusNoContent || tab.Err != "" {
			t.Fatal("imported request failed to reach the local HTTP server")
		}
		if !reflect.DeepEqual(tab.Request, before) {
			t.Fatal("sending resolved credentials/variables into the saved editor draft")
		}
		select {
		case got := <-requests:
			return got
		default:
			t.Fatal("real HTTP service did not send the request")
			return observed{}
		}
	}
	got := send()
	if !reflect.DeepEqual(got.authorization, []string{"Bearer env-token"}) || got.fallback != "collection-fallback" {
		t.Fatal("imported inheritance/environment duplicates/disabled-variable precedence changed on wire")
	}
	editParent := func(kind, first, second string) {
		t.Helper()
		collectionsContext(h, "Shared parent", h.App.S.Collections.Authentication)
		collectionsOption(h, kind)
		switch kind {
		case h.App.S.Request.Basic:
			requestType(h, h.App.S.Request.Username, first, true)
			requestType(h, h.App.S.Request.Password, second, true)
		case h.App.S.Request.Bearer:
			requestType(h, h.App.S.Request.Token, first, true)
		case h.App.S.Request.APIKey:
			requestType(h, h.App.S.Request.APIKeyName, first, true)
			requestType(h, h.App.S.Request.APIKeyValue, second, true)
		}
		collectionsOption(h, h.App.S.Collections.Save)
		if h.App.collections.authOpen {
			t.Fatal("parent auth edit did not save")
		}
	}
	editParent(h.App.S.Request.Basic, "{{user}}", "{{password}}")
	got = send()
	if !reflect.DeepEqual(got.authorization, []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("env-user:env-password"))}) {
		t.Fatal("open imported request did not observe live Basic parent auth")
	}
	editParent(h.App.S.Request.Bearer, "parent-{{token}}", "")
	if got = send(); !reflect.DeepEqual(got.authorization, []string{"Bearer parent-env-token"}) {
		t.Fatal("open imported request did not observe live Bearer parent edit")
	}
	h.click("Explicit headers")
	if got = send(); !reflect.DeepEqual(got.authorization, []string{"", "manual"}) || !reflect.DeepEqual(got.apiKey, []string{"", "manual-key"}) {
		t.Fatal("explicit empty/repeated headers failed to override generated auth")
	}
	for _, name := range []string{"Explicit none", "Barrier request"} {
		h.click(name)
		if got = send(); len(got.authorization) != 0 || len(got.apiKey) != 0 {
			t.Fatal("imported explicit None or folder None barrier gained credentials")
		}
	}
	h.click("Live inherited")
	editParent(h.App.S.Request.APIKey, "X-Api-Key", "{{token}}")
	if got = send(); !reflect.DeepEqual(got.apiKey, []string{"env-token"}) || len(got.authorization) != 0 {
		t.Fatal("disabled explicit header suppressed imported shared API-key generation")
	}
	h.click("Explicit headers")
	if got = send(); !reflect.DeepEqual(got.apiKey, []string{"", "manual-key"}) {
		t.Fatal("explicit empty/repeated API-key headers did not win over shared auth")
	}
	h.click("Live inherited")
	collectionsContext(h, "Shared parent", h.App.S.Collections.Authentication)
	requestType(h, h.App.S.Request.APIKeyName, "key", true)
	collectionsOption(h, h.App.S.Request.InQuery)
	collectionsOption(h, h.App.S.Collections.Save)
	got = send()
	if !reflect.DeepEqual(got.query["key"], []string{"env-token"}) || !reflect.DeepEqual(got.query["q"], []string{"a", "a", "b"}) || len(got.query["skip"]) != 0 || len(got.authorization) != 0 || len(got.apiKey) != 0 {
		t.Fatal("query API-key replacement damaged repeated query rows or sent disabled rows")
	}
	if !reflect.DeepEqual(live.Request, beforeLive) || live.Request.Auth.Type != model.AuthInherit {
		t.Fatal("live parent edits flattened or mutated the imported request")
	}
	// History retains placeholders and resolved scope, not exported/env values.
	for _, entry := range svc.History.List() {
		encoded, err := json.Marshal(entry.Request)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "env-token") || strings.Contains(string(encoded), "env-password") {
			t.Fatal("history persisted expanded imported credentials")
		}
	}
}
