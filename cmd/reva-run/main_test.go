package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/storage"
)

const secret = "credential"

func testItem(id, url string) model.Item {
	req := model.NewRequest(id)
	req.Name, req.URL = id, url
	return model.Item{ID: id, Name: id, Type: model.ItemRequest, Request: &req}
}

func testCollection(items ...model.Item) model.Collection {
	return model.Collection{Version: model.SchemaVersion, ID: "collection", Name: "collection", Items: items}
}

func writeJSON(t *testing.T, dir, relative string, value any) {
	t.Helper()
	if err := storage.WriteJSON(filepath.Join(dir, relative), value); err != nil {
		t.Fatal("write fixture failed")
	}
}

func writeCollection(t *testing.T, dir string, col model.Collection) {
	t.Helper()
	writeJSON(t, dir, filepath.Join("collections", col.ID+".json"), col)
}

func invoke(t *testing.T, ctx context.Context, dir string, extra ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	args := append([]string{"--data-dir", dir, "--collection", "collection"}, extra...)
	code := run(ctx, args, &stdout, &stderr)
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatal("credential leaked to CLI output")
	}
	return code, stdout.String(), stderr.String()
}

func decodeReport(t *testing.T, data string) runner.Report {
	t.Helper()
	var report runner.Report
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&report); err != nil {
		t.Fatal("stdout is not a report")
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		t.Fatal("stdout contains more than one report")
	}
	if report.Version != 1 || !strings.HasSuffix(data, "\n") || strings.Count(data, "\n") != 1 {
		t.Fatal("report version or compact output contract changed")
	}
	return report
}

func snapshot(t *testing.T, dir string) map[string]string {
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
		t.Fatal("snapshot failed")
	}
	return files
}

func TestCLIInheritedAuthVariablesAndNonmutation(t *testing.T) {
	type received struct{ auth, value, fallback, body, query string }
	got := make(chan received, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- received{r.Header.Get("Authorization"), r.Header.Get("X-Value"), r.Header.Get("X-Fallback"), string(body), r.URL.Query().Get("credential")}
		w.Header().Set("X-Credential", secret)
		_, _ = io.WriteString(w, secret)
	}))
	defer srv.Close()
	dir := t.TempDir()
	it := testItem("request", "{{host}}?credential={{token}}")
	it.Request.Method = model.MethodPost
	it.Request.Auth.Type = model.AuthInherit
	it.Request.Body = model.Body{Mode: model.BodyRaw, Text: "{{local}}"}
	it.Request.Headers = []model.KV{{Key: "X-Value", Value: "{{value}}"}, {Key: "X-Fallback", Value: "{{fallback}}"}}
	it.Request.PreScript = `pm.variables.set("local", pm.variables.get("token")); console.log(pm.variables.get("token"));`
	it.Request.TestScript = `pm.test(pm.variables.get("token"), function(){pm.response.to.have.status(200);});`
	col := testCollection(model.Item{ID: "folder", Name: secret, Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, Items: []model.Item{it}})
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "root-token"}
	col.Variables = []model.KV{{Key: "host", Value: srv.URL}, {Key: "token", Value: "wrong-collection"}, {Key: "value", Value: "collection"}, {Key: "fallback", Value: "fallback"}}
	writeCollection(t, dir, col)
	writeJSON(t, dir, "environments/env.json", model.Environment{Version: 1, ID: "env", Name: secret, Variables: []model.KV{{Key: "token", Value: secret}, {Key: "value", Value: "environment"}, {Key: "fallback", Value: "ignored", Disabled: true}}})
	settings := model.DefaultSettings()
	settings.ActiveEnvID = "irrelevant-active-env"
	writeJSON(t, dir, "settings.json", settings)
	writeJSON(t, dir, "history.json", []model.HistoryEntry{{ID: "untouched", Error: secret}})
	// Unselected files are deliberately irrelevant, even if malformed.
	if err := os.WriteFile(filepath.Join(dir, "environments", "irrelevant-active-env.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, dir)
	code, out, stderr := invoke(t, context.Background(), dir, "--environment", "env")
	if code != exitOK || stderr != "" {
		t.Fatal("successful run returned diagnostics or failed")
	}
	report := decodeReport(t, out)
	if report.Outcome != runner.Passed || report.Counts.Passed != 1 || report.Assertions.Passed != 1 || report.Requests[0].StatusCode != 200 {
		t.Fatalf("unexpected safe report: %+v", report)
	}
	if actual := <-got; actual != (received{"Bearer " + secret, "environment", "fallback", secret, secret}) {
		t.Fatal("HTTP request did not receive inherited auth and variables")
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("CLI mutated native store")
	}
}

func TestCLIDefaultEnvironmentAndDataDirectorySelection(t *testing.T) {
	got := make(chan string, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got <- r.Header.Get("X-Value") }))
	defer srv.Close()
	dir := t.TempDir()
	it := testItem("one", srv.URL)
	it.Request.Headers = []model.KV{{Key: "X-Value", Value: "{{value}}"}}
	col := testCollection(it)
	col.Variables = []model.KV{{Key: "value", Value: "collection"}}
	writeCollection(t, dir, col)
	settings := model.DefaultSettings()
	settings.ActiveEnvID = "active"
	writeJSON(t, dir, "settings.json", settings)
	writeJSON(t, dir, "environments/active.json", model.Environment{Version: 1, ID: "active", Name: "active", Variables: []model.KV{{Key: "value", Value: "active"}}})
	t.Setenv(storage.EnvHome, dir)
	var out, stderr bytes.Buffer
	if code := run(context.Background(), []string{"--collection", "collection"}, &out, &stderr); code != exitOK || stderr.Len() != 0 {
		t.Fatal("REVA_HOME run failed")
	}
	if <-got != "collection" {
		t.Fatal("CLI implicitly selected active environment")
	}
	other := t.TempDir()
	col.Variables[0].Value = "explicit-directory"
	writeCollection(t, other, col) // No settings file: safe model defaults.
	if code, _, _ := invoke(t, context.Background(), other); code != exitOK || <-got != "explicit-directory" {
		t.Fatal("explicit data directory did not override REVA_HOME")
	}
	// Resolve the platform default using an isolated config location.
	config := t.TempDir()
	t.Setenv(storage.EnvHome, "")
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", config)
	} else if runtime.GOOS == "darwin" {
		t.Setenv("HOME", config)
	} else {
		t.Setenv("XDG_CONFIG_HOME", config)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	defaultDir := filepath.Join(base, "Reva")
	col.Variables[0].Value = "platform-default"
	writeCollection(t, defaultDir, col)
	out.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"--collection", "collection"}, &out, &stderr); code != exitOK || <-got != "platform-default" {
		t.Fatal("platform data directory selection failed")
	}
}

func TestCLIFailureContinuationStopAndHTTPStatus(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(503) }))
	defer srv.Close()
	dir := t.TempDir()
	first := testItem("first", srv.URL)
	first.Request.TestScript = `pm.test("status assertion", function(){pm.response.to.have.status(200);});`
	col := testCollection(first, testItem("second", srv.URL))
	writeCollection(t, dir, col)
	for _, stop := range []bool{false, true} {
		calls.Store(0)
		code, out, stderr := invoke(t, context.Background(), dir, "--stop-on-failure="+map[bool]string{true: "true", false: "false"}[stop])
		report := decodeReport(t, out)
		if code != exitFailed || stderr != "" || report.Outcome != runner.Failed || report.Assertions.Failed != 1 || report.Requests[0].Code != runner.CodeAssertion {
			t.Fatal("assertion failure classification lost")
		}
		if stop {
			if calls.Load() != 1 || report.Counts.Skipped != 1 || report.Requests[1].Code != runner.CodeSkippedFailure {
				t.Fatal("stop-on-failure did not skip later request")
			}
		} else if calls.Load() != 2 || report.Counts.Passed != 1 || report.Requests[1].StatusCode != 503 {
			t.Fatal("default continuation or HTTP status semantics changed")
		}
	}
}

func TestCLISafeReportsForScriptAndTransportFailures(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Set-Cookie", "credential="+secret)
		_, _ = io.WriteString(w, secret)
	}))
	defer srv.Close()
	for _, tc := range []struct {
		name string
		edit func(*model.Request)
		code runner.Code
		sent bool
	}{
		{"pre", func(r *model.Request) { r.PreScript += `throw new Error("` + secret + `");` }, runner.CodePreScript, false},
		{"tests", func(r *model.Request) {
			r.TestScript = `console.log("` + secret + `"); pm.test("` + secret + `", function(){throw new Error("` + secret + `");}); throw new Error("` + secret + `");`
		}, runner.CodeTestScript, true},
		{"transport-url", func(r *model.Request) { r.URL = "http://%" + secret }, runner.CodeTransport, true},
		{"transport-file", func(r *model.Request) {
			r.Body = model.Body{Mode: model.BodyBinary, FilePath: filepath.Join(t.TempDir(), secret)}
		}, runner.CodeTransport, true},
		{"auth", func(r *model.Request) { r.Auth = model.Auth{Type: model.AuthType(secret)} }, runner.CodeAuth, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			it := testItem(secret, srv.URL+"/"+secret+"?credential="+secret)
			it.Request.Headers = []model.KV{{Key: "X-Credential", Value: secret}}
			it.Request.Body = model.Body{Mode: model.BodyRaw, Text: secret}
			it.Request.PreScript = `console.log("` + secret + `");`
			tc.edit(it.Request)
			col := testCollection(it)
			col.ID, col.Name = secret, secret
			col.Variables = []model.KV{{Key: secret, Value: secret}}
			writeCollection(t, dir, col)
			before := snapshot(t, dir)
			calls.Store(0)
			code, out, stderr := invoke(t, context.Background(), dir, "--collection", secret)
			report := decodeReport(t, out)
			if code != exitFailed || stderr != "" || report.Requests[0].Code != tc.code || report.Requests[0].Sent != tc.sent {
				t.Fatalf("unexpected safe failure report: %+v", report)
			}
			if tc.code == runner.CodeTestScript && (calls.Load() != 1 || report.Assertions.Failed != 1) {
				t.Fatal("failed tests lost response or assertion count")
			}
			if tc.code != runner.CodeTestScript && calls.Load() != 0 {
				t.Fatal("failed preparation unexpectedly called endpoint")
			}
			if !reflect.DeepEqual(before, snapshot(t, dir)) {
				t.Fatal("failed run mutated native data")
			}
		})
	}
}

func TestCLIUsageAndHelpNeverLoadOrCreateData(t *testing.T) {
	dir := filepath.Join(t.TempDir(), secret, "absent")
	t.Setenv(storage.EnvHome, dir)
	for _, args := range [][]string{nil, {"--collection"}, {"--collection", "../escape"}, {"--unknown-" + secret}, {"--collection", "collection", "extra-" + secret}, {"--collection", "collection", "--environment="}, {"--collection", "collection", "--data-dir="}, {"--collection", "collection", "--stop-on-failure=" + secret}} {
		var out, stderr bytes.Buffer
		if code := run(context.Background(), args, &out, &stderr); code != exitInput || out.Len() != 0 || !strings.HasPrefix(stderr.String(), "reva-run: ") || strings.Contains(stderr.String(), secret) {
			t.Fatal("usage failure violated safe output contract")
		}
	}
	for _, help := range []string{"-h", "--help"} {
		var out, stderr bytes.Buffer
		if code := run(context.Background(), []string{help}, &out, &stderr); code != exitOK || stderr.Len() != 0 || !strings.HasPrefix(out.String(), "用法：") || !strings.Contains(out.String(), "Usage:") {
			t.Fatal("help did not succeed without data")
		}
	}
	code, out, stderr := invoke(t, context.Background(), dir)
	if code != exitInput || out != "" || strings.Contains(stderr, dir) {
		t.Fatal("missing directory failure leaked path or succeeded")
	}
	if _, err := os.Stat(filepath.Dir(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("CLI created missing data directory")
	}
}

func TestCLIHelpIsBilingualAndIgnoresNativeData(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "settings.json", map[string]any{"version": secret})
	writeJSON(t, dir, "collections/collection.json", map[string]any{secret: secret})
	before := snapshot(t, dir)
	for _, help := range []string{"-h", "--help"} {
		code, out, stderr := invoke(t, context.Background(), dir, help)
		if code != exitOK || stderr != "" || !strings.HasPrefix(out, "用法：") || !strings.Contains(out, "Usage:") {
			t.Fatal("help must print Chinese then English without loading invalid native data")
		}
		if !strings.Contains(out, "默认不选择环境") || !strings.Contains(out, "No environment is selected by default") {
			t.Fatal("help must explain environment selection in both languages")
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("help mutated native data")
	}
}

func TestCLIInvalidNativeInputNeverSendsOrMutates(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()
	for _, tc := range []struct {
		name string
		edit func(*testing.T, string, *model.Collection) []string
	}{
		{"missing-collection", func(t *testing.T, d string, c *model.Collection) []string { return []string{"--collection", "missing"} }},
		{"missing-environment", func(t *testing.T, d string, c *model.Collection) []string {
			return []string{"--environment", "missing"}
		}},
		{"bad-settings", func(t *testing.T, d string, c *model.Collection) []string {
			writeJSON(t, d, "settings.json", map[string]any{"version": 1, "insecureTls": secret})
			return nil
		}},
		{"future-settings", func(t *testing.T, d string, c *model.Collection) []string {
			writeJSON(t, d, "settings.json", model.Settings{Version: 2})
			return nil
		}},
		{"unversioned-settings", func(t *testing.T, d string, c *model.Collection) []string {
			writeJSON(t, d, "settings.json", map[string]any{})
			return nil
		}},
		{"future-collection", func(t *testing.T, d string, c *model.Collection) []string { c.Version = 2; return nil }},
		{"id-mismatch", func(t *testing.T, d string, c *model.Collection) []string { c.ID = "other"; return nil }},
		{"duplicate-item", func(t *testing.T, d string, c *model.Collection) []string {
			c.Items = append(c.Items, c.Items[0])
			return nil
		}},
		{"missing-request", func(t *testing.T, d string, c *model.Collection) []string { c.Items[0].Request = nil; return nil }},
		{"bad-item-type", func(t *testing.T, d string, c *model.Collection) []string {
			c.Items[0].Type = model.ItemType(secret)
			return nil
		}},
		{"bad-option", func(t *testing.T, d string, c *model.Collection) []string {
			c.Items[0].Request.Options.VerifyTLS = model.Toggle(secret)
			return nil
		}},
		{"bad-timeout", func(t *testing.T, d string, c *model.Collection) []string {
			c.Items[0].Request.Options.TimeoutMs = -1
			return nil
		}},
		{"future-environment", func(t *testing.T, d string, c *model.Collection) []string {
			writeJSON(t, d, "environments/env.json", model.Environment{Version: 2, ID: "env", Name: "env"})
			return []string{"--environment", "env"}
		}},
		{"malformed-environment", func(t *testing.T, d string, c *model.Collection) []string {
			writeJSON(t, d, "environments/env.json", map[string]any{"version": 1, "id": "env", "variables": secret})
			return []string{"--environment", "env"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			col := testCollection(testItem("one", srv.URL))
			extra := tc.edit(t, dir, &col)
			writeJSON(t, dir, "collections/collection.json", col)
			before := snapshot(t, dir)
			code, out, stderr := invoke(t, context.Background(), dir, extra...)
			if code != exitInput || out != "" || stderr == "" {
				t.Fatal("invalid native input did not fail safely")
			}
			if !reflect.DeepEqual(before, snapshot(t, dir)) || calls.Load() != 0 {
				t.Fatal("invalid native input sent HTTP or mutated data")
			}
		})
	}
}

func TestCLICancellationProducesPartialSafeReport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/cancel" {
			cancel()
			<-r.Context().Done()
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	writeCollection(t, dir, testCollection(testItem("one", srv.URL), testItem("two", srv.URL+"/cancel"), testItem("three", srv.URL)))
	before := snapshot(t, dir)
	code, out, stderr := invoke(t, ctx, dir)
	report := decodeReport(t, out)
	if code != exitCancelled || stderr != "" || report.Outcome != runner.Cancelled || report.Counts != (runner.Counts{Total: 3, Passed: 1, Cancelled: 1, Skipped: 1}) || calls.Load() != 2 {
		t.Fatalf("partial cancellation report incorrect: %+v", report)
	}
	if report.Requests[2].Code != runner.CodeSkippedCancellation || !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("cancellation sent later request or mutated data")
	}
	code, out, _ = invoke(t, ctx, dir)
	report = decodeReport(t, out)
	if code != exitCancelled || report.Counts.Skipped != 3 || calls.Load() != 2 {
		t.Fatal("pre-cancelled command sent a request")
	}
}

func TestCLIEmptyCollectionAndReportWriterFailure(t *testing.T) {
	dir := t.TempDir()
	writeCollection(t, dir, testCollection())
	code, out, stderr := invoke(t, context.Background(), dir)
	report := decodeReport(t, out)
	if code != exitOK || stderr != "" || report.Counts.Total != 0 || report.Requests == nil {
		t.Fatal("empty collection contract changed")
	}
	var diagnostics bytes.Buffer
	if code := run(context.Background(), []string{"--data-dir", dir, "--collection", "collection"}, failingWriter{}, &diagnostics); code != exitFailed || diagnostics.String() != "reva-run: output_failed\n" {
		t.Fatal("output failure was not diagnosed safely")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New(secret) }

func TestCLIBinaryExitCodesAndGUIFreeDependencies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	deps, err := exec.CommandContext(ctx, "go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatal("dependency inspection failed")
	}
	for _, path := range strings.Fields(string(deps)) {
		if strings.HasPrefix(path, "github.com/vivaui/viva") || strings.HasPrefix(path, "github.com/runbrick/reva/internal/gui") {
			t.Fatal("CLI dependency closure includes GUI")
		}
	}
	binary := filepath.Join(t.TempDir(), "reva-run")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s", output)
	}
	dir := t.TempDir()
	first := testItem("one", "http://127.0.0.1:1")
	first.Request.PreScript = `throw new Error("` + secret + `");`
	writeCollection(t, dir, testCollection(first))
	for _, tc := range []struct {
		args []string
		exit int
	}{
		{[]string{"--help"}, exitOK},
		{nil, exitInput},
		{[]string{"--data-dir", dir, "--collection", "collection"}, exitFailed},
	} {
		cmd := exec.CommandContext(ctx, binary, tc.args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal("binary launch failed")
			}
			code = exit.ExitCode()
		}
		if code != tc.exit || strings.Contains(stdout.String()+stderr.String(), secret) {
			t.Fatal("binary exit code or credential safety violated")
		}
		if tc.exit == exitFailed {
			decodeReport(t, stdout.String())
		}
		if tc.exit == exitOK && (!strings.HasPrefix(stdout.String(), "用法：") || !strings.Contains(stdout.String(), "Usage:")) {
			t.Fatal("binary help must include Chinese and English")
		}
	}
}
