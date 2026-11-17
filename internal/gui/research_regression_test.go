package gui

import (
	"context"
	"encoding/base64"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service"
	requestservice "github.com/runbrick/reva/internal/service/request"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/vivaui/viva/gfx"
)

// Include directory names as well as bytes: a read-only operation must not
// create empty persistence directories or temporary files either.
func researchDisk(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[rel+"/"] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		files[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func researchStore(t *testing.T, dir string, col model.Collection, env model.Environment) *store.Store {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	settings := model.DefaultSettings()
	settings.Language, settings.ActiveEnvID = "en", env.ID
	settings.OnboardingCompleted = true
	settings.NoFollowRedirects = true
	for _, err := range []error{
		st.Collections().Save(col), st.Environments().Save(env), st.Settings().Save(settings),
		st.History().Add(model.HistoryEntry{Request: model.NewRequest("existing-history"), StatusCode: 204}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	// Reopen so the test exercises native deserialization rather than the
	// same in-memory model that wrote the fixture.
	st, err = store.Open(dir)
	if err != nil || len(st.LoadErrors()) != 0 {
		t.Fatal("cannot reload native fixture", err)
	}
	return st
}

func TestResearchGUIAndRunnerAgreeOnStoredSendSemantics(t *testing.T) {
	type received struct {
		Method, Path, Body, Authorization, Key, Value, Fallback string
		Query                                                   url.Values
	}
	for _, tc := range []struct {
		name string
		auth model.Auth
		want string
		pre  string
	}{
		{"bearer-after-pre", model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, "Bearer runtime-token", `pm.variables.set("token", "runtime-token");`},
		{"basic", model.Auth{Type: model.AuthBasic, Username: "{{user}}", Password: "{{token}}"}, "Basic " + base64.StdEncoding.EncodeToString([]byte("env-user:env-token")), ""},
		{"query-key", model.Auth{Type: model.AuthAPIKey, Key: "api-key", Value: "{{token}}", In: model.APIKeyInQuery}, "", ""},
		{"divergent-bearer", model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, "Bearer env-token", ""},
		{"divergent-query-key", model.Auth{Type: model.AuthAPIKey, Key: "api-key", Value: "{{token}}", In: model.APIKeyInQuery}, "", ""},
		{"expanded-value-equality", model.Auth{Type: model.AuthNone}, "", ""},
		{"expanded-key-delimiters", model.Auth{Type: model.AuthNone}, "", ""},
		{"explicit-empty-header", model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, "", ""},
		{"redirect-override", model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, "Bearer env-token", ""},
		{"pre-abort", model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, "", `console.log("abort-evidence"); throw new Error("abort-before-http");`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := make(chan received, 8)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.name == "redirect-override" && r.URL.Path == "/wire" {
					http.Redirect(w, r, "/final?"+r.URL.RawQuery, http.StatusTemporaryRedirect)
					return
				}
				body, _ := io.ReadAll(r.Body)
				got <- received{r.Method, r.URL.Path, string(body), r.Header.Get("Authorization"), r.URL.Query().Get("api-key"), r.Header.Get("X-Value"), r.Header.Get("X-Fallback"), r.URL.Query()}
				w.WriteHeader(202)
			}))
			defer srv.Close()
			dir := t.TempDir()
			t.Setenv("REVA_HOME", dir)
			req := model.NewRequest("request")
			req.Name, req.Method, req.URL = "Stored request", model.MethodPost, "{{host}}/wire?tag=a&tag=a&tag=b&api-key=old&api-key=older"
			req.Auth.Type = model.AuthInherit
			req.Params = []model.KV{{Key: "tag", Value: "a"}, {Key: "tag", Value: "a"}, {Key: "tag", Value: "b"}, {Key: "api-key", Value: "old"}, {Key: "api-key", Value: "older"}}
			req.Headers = []model.KV{{Key: "X-Value", Value: "{{value}}"}, {Key: "X-Fallback", Value: "{{fallback}}"}}
			if strings.HasPrefix(tc.name, "divergent-") {
				// Valid native files can have URL-only and Params-only pairs.
				// Opening one must preserve the sender's multiset merge, including
				// duplicate values and disabled rows, before auth is generated.
				req.URL = "{{host}}/wire?tag=a&tag=a&url-only=kept&api-key=old&api-key=older"
				req.Params = []model.KV{{Key: "tag", Value: "a"}, {Key: "tag", Value: "a"}, {Key: "tag", Value: "b"}, {Key: "disabled", Value: "never-send", Disabled: true}}
			}
			if tc.name == "explicit-empty-header" {
				req.Headers = append(req.Headers, model.KV{Key: "authorization", Value: ""})
			}
			if tc.name == "expanded-value-equality" {
				req.URL = "{{host}}/wire?x={{query-value}}"
				req.Params = []model.KV{{Key: "x", Value: "a"}}
			}
			if tc.name == "expanded-key-delimiters" {
				req.URL = "{{host}}/wire"
				req.Params = []model.KV{{Key: "{{query-key}}", Value: "a"}}
			}
			if tc.name == "redirect-override" {
				req.Options.FollowRedirects = model.ToggleOn
			}
			req.Body = model.Body{Mode: model.BodyRaw, Text: "{{outer}}"}
			req.PreScript = tc.pre
			req.TestScript = `pm.test("same response", function(){ pm.response.to.have.status(202); });`
			col := model.Collection{ID: "collection", Name: "Stored collection", Auth: model.Auth{Type: model.AuthNone}, Variables: []model.KV{
				{Key: "host", Value: srv.URL}, {Key: "token", Value: "collection-token"}, {Key: "value", Value: "collection-value"},
				{Key: "fallback", Value: "collection-fallback"}, {Key: "outer", Value: "{{inner}}"}, {Key: "inner", Value: "must-not-expand-twice"},
				{Key: "query-value", Value: "a"}, {Key: "query-key", Value: "x&y"},
			}, Items: []model.Item{{ID: "folder", Name: "Stored folder", Type: model.ItemFolder, Auth: tc.auth, Items: []model.Item{{ID: req.ID, Name: req.Name, Type: model.ItemRequest, Request: &req}}}}}
			env := model.Environment{ID: "environment", Name: "QA environment", Variables: []model.KV{
				{Key: "user", Value: "env-user"}, {Key: "token", Value: "env-token"}, {Key: "value", Value: "old-value"},
				{Key: "value", Value: "env-value"}, {Key: "fallback", Value: "disabled-value", Disabled: true},
			}}
			st := researchStore(t, dir, col, env)
			col, _ = st.Collections().Get(col.ID)
			env, _ = st.Environments().Get(env.ID)
			before := researchDisk(t, dir)
			result := runner.Run(context.Background(), requestservice.New(), col, st.Settings().Get(), env.Variables, runner.Options{})
			if !reflect.DeepEqual(before, researchDisk(t, dir)) {
				t.Fatal("runner changed persisted GUI data")
			}
			h := newHarness(t, service.Services{Requests: requestservice.New(), Collections: st.Collections(), Environments: st.Environments(), Settings: st.Settings(), History: st.History()})
			h.App.OpenRequest(*col.Items[0].Items[0].Request, col.ID)
			tab := h.App.Active()
			requestRepaint(h)
			if !reflect.DeepEqual(*col.Items[0].Items[0].Request, tab.Request) {
				t.Fatal("opening a native request rewrote authored URL/Params before an explicit edit")
			}
			draft := tab.Request.Clone()
			h.click(h.App.S.Request.Send)
			h.waitFor("stored GUI send", func() bool { return !tab.Sending })
			if tc.name == "pre-abort" {
				if len(got) != 0 || result.Requests[0].Code != runner.CodePreScript || result.Requests[0].Sent || tab.Response != nil || !h.shows("abort-evidence") || !h.shows("abort-before-http") || !reflect.DeepEqual(before, researchDisk(t, dir)) {
					t.Fatal("GUI/runner disagree on pre failure or pre failure mutated disk")
				}
			} else {
				if result.Outcome != runner.Passed || result.Report().Assertions.Passed != 1 || tab.Err != "" || tab.Response == nil || tab.Response.StatusCode != 202 || len(tab.Tests) != 1 || !tab.Tests[0].Passed || len(got) != 2 {
					t.Fatal("stored GUI/runner execution or response assertions differ")
				}
				fromRunner, fromGUI := <-got, <-got
				wantPath := "/wire"
				if tc.name == "redirect-override" {
					wantPath = "/final"
				}
				wantQuery := url.Values{"tag": {"a", "a", "b"}, "api-key": {"old", "older"}}
				if tc.auth.Type == model.AuthAPIKey {
					wantQuery["api-key"] = []string{"env-token"}
				}
				if strings.HasPrefix(tc.name, "divergent-") {
					wantQuery["url-only"] = []string{"kept"}
				}
				if tc.name == "expanded-value-equality" {
					wantQuery = url.Values{"x": {"a"}}
				}
				if tc.name == "expanded-key-delimiters" {
					wantQuery = url.Values{"x&y": {"a"}}
				}
				if !reflect.DeepEqual(fromRunner, fromGUI) || fromGUI.Method != "POST" || fromGUI.Path != wantPath || fromGUI.Authorization != tc.want || fromGUI.Body != "{{inner}}" || fromGUI.Value != "env-value" || fromGUI.Fallback != "collection-fallback" || !reflect.DeepEqual(fromGUI.Query, wantQuery) {
					t.Fatalf("wire semantics differ: runner=%+v GUI=%+v", fromRunner, fromGUI)
				}
				if tc.auth.Type == model.AuthAPIKey && !reflect.DeepEqual(fromGUI.Query["api-key"], []string{"env-token"}) {
					t.Fatal("inherited query authentication did not replace duplicate keys")
				}
				if strings.HasPrefix(tc.name, "divergent-") && (fromGUI.Query.Get("url-only") != "kept" || fromGUI.Query.Has("disabled")) {
					t.Fatal("editor load discarded URL-only query pairs or enabled disabled rows")
				}
				after := researchDisk(t, dir)
				after["history.json"] = before["history.json"]
				if !reflect.DeepEqual(before, after) || len(st.History().List()) != 2 {
					t.Fatal("GUI send changed files beyond its one expected history entry")
				}
				entry := st.History().List()[0].Request
				wantHistory := draft.Clone()
				wantHistory.Auth = tc.auth
				if !reflect.DeepEqual(entry, wantHistory) {
					t.Fatal("GUI history persisted expanded or script-modified values")
				}
			}
			if !reflect.DeepEqual(draft, tab.Request) || tab.Dirty {
				t.Fatal("send changed the saved editor draft")
			}
		})
	}
}

func TestResearchPreviewUsesPersistedEnvironmentWithoutExecuting(t *testing.T) {
	const canary = "QAPRIV17"
	const responseCanary = "QARESP17"
	const logCanary = "QALOG17"
	got := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("Authorization")
		_, _ = io.WriteString(w, responseCanary)
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	req := model.NewRequest("preview-request")
	req.Name, req.URL, req.Auth.Type = canary+" request", "{{host}}/route?q={{value}}", model.AuthInherit
	req.Body = model.Body{Mode: model.BodyRaw, Text: canary}
	req.PreScript = `pm.variables.set("token", "after-pre-token"); console.log("` + logCanary + `");`
	req.TestScript = `console.log(pm.response.text());`
	col := model.Collection{ID: "collection", Name: canary + " collection", Auth: model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, Variables: []model.KV{{Key: "host", Value: srv.URL}, {Key: "value", Value: "collection-private"}}, Items: []model.Item{{ID: req.ID, Name: req.Name, Type: model.ItemRequest, Request: &req}}}
	env := model.Environment{ID: "environment", Name: canary + " Persisted QA environment", Variables: []model.KV{{Key: "value", Value: "shadowed-private"}, {Key: "value", Value: canary}, {Key: "token", Value: "before-pre-private"}}}
	st := researchStore(t, dir, col, env)
	h := newHarness(t, service.Services{Requests: requestservice.New(), Collections: st.Collections(), Environments: st.Environments(), Settings: st.Settings(), History: st.History()})
	h.App.OpenRequest(req, col.ID)
	requestRepaint(h)
	h.click(h.App.S.Request.Send)
	h.waitFor("response before preview", func() bool { return !h.App.Active().Sending })
	if len(got) != 1 || <-got != "Bearer after-pre-token" || h.App.Active().Response == nil || string(h.App.Active().Response.Body) != responseCanary {
		t.Fatal("could not establish real prior response and script logs")
	}
	// A preview must preserve prior results without painting them behind its
	// modal, and must not execute this newly edited, throwing pre script.
	h.App.Active().Request.PreScript = `throw new Error("preview-script-must-not-run");`
	requestRepaint(h)
	priorResponse := h.App.Active().Response
	priorLogs := append([]string(nil), h.App.scriptTab(h.App.Active()).logs...)
	if !reflect.DeepEqual(priorLogs, []string{logCanary, responseCanary}) {
		t.Fatal("prior send did not retain expected script diagnostics")
	}
	before := researchDisk(t, dir)
	draft := h.App.Active().Request.Clone()
	h.click(h.App.S.Request.Preview)
	seen := ""
	for i := 0; i < 7; i++ {
		snapshot := h.D.Snapshot()
		for _, label := range snapshot.Labels {
			seen += label.Value + "\n"
		}
		for _, hit := range snapshot.Hits {
			seen += hit.Label + "\n"
		}
		if err := h.D.Scroll(gfx.ScrollEvent{X: 640, Y: 380, DeltaY: 150, Precise: true}); err != nil {
			t.Fatal(err)
		}
	}
	for _, secret := range []string{canary, responseCanary, logCanary, srv.URL, "shadowed-private", "collection-private", "before-pre-private", "after-pre-token", "preview-script-must-not-run"} {
		if strings.Contains(seen, secret) {
			t.Fatalf("preview painted private data %q", secret)
		}
	}
	for _, visible := range []string{"Persisted QA environment", "Source: Collection", "Environment row 2", "Shadowed by a later enabled row", "[masked]", "BEFORE pre-request scripts"} {
		if !strings.Contains(seen, visible) {
			t.Fatalf("preview omitted persisted provenance %q", visible)
		}
	}
	if len(got) != 0 || !reflect.DeepEqual(h.App.scriptTab(h.App.Active()).logs, priorLogs) || h.App.Active().Response != priorResponse || h.App.Active().Err != "" || !reflect.DeepEqual(before, researchDisk(t, dir)) || !reflect.DeepEqual(draft, h.App.Active().Request) {
		t.Fatal("preview executed or changed native data/draft")
	}
	h.click(h.App.S.Request.PreviewClose)
	if !h.shows(canary+" collection") || h.App.requestTabTitle(h.App.Active()) != req.Name {
		t.Fatal("closing preview did not restore stored display names")
	}
	if !h.shows(responseCanary) {
		t.Fatal("closing preview did not restore the previous response")
	}
	h.click(h.App.S.Request.Send)
	h.waitFor("send after preview", func() bool { return !h.App.Active().Sending })
	if len(got) != 0 || h.App.Active().Response != nil || !h.shows("preview-script-must-not-run") || !reflect.DeepEqual(before, researchDisk(t, dir)) {
		t.Fatal("explicit send after preview did not abort at its newly edited pre script")
	}
}
