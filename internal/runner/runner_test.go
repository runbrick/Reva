package runner_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/execution"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service"
	requestservice "github.com/runbrick/reva/internal/service/request"
	"github.com/runbrick/reva/internal/service/store"
)

type requestFunc func(context.Context, model.Request, []model.KV) (model.Response, error)

func (f requestFunc) Send(ctx context.Context, req model.Request, values []model.KV) (model.Response, error) {
	return f(ctx, req, values)
}

func item(id, url string) model.Item {
	req := model.NewRequest("stale-embedded-id")
	req.Name, req.URL = "same name", url
	return model.Item{ID: id, Name: "same name", Type: model.ItemRequest, Request: &req}
}

func collection(items ...model.Item) model.Collection {
	return model.Collection{ID: "collection", Name: "collection", Version: model.SchemaVersion, Items: items}
}

func run(ctx context.Context, col model.Collection, options runner.Options) runner.Result {
	return runner.Run(ctx, requestservice.New(), col, model.DefaultSettings(), nil, options)
}

func jsonBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestNestedDepthFirstOrderAuthAndEnvironmentPrecedence(t *testing.T) {
	type received struct{ path, auth, value, fallback string }
	got := make(chan received, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got <- received{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-Value"), r.Header.Get("X-Fallback")}
	}))
	defer srv.Close()
	first, second, third := item("one", "{{host}}/one"), item("two", "{{host}}/two"), item("three", "{{host}}/three")
	for _, it := range []*model.Item{&first, &second, &third} {
		it.Request.Auth.Type = model.AuthInherit
		it.Request.Headers = []model.KV{{Key: "X-Value", Value: "{{value}}"}, {Key: "X-Fallback", Value: "{{fallback}}"}}
	}
	col := collection(model.Item{ID: "folder", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, Items: []model.Item{
		first, {ID: "barrier", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthNone}, Items: []model.Item{second}},
	}}, third)
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "root-token"}
	col.Variables = []model.KV{{Key: "host", Value: srv.URL}, {Key: "value", Value: "collection"}, {Key: "fallback", Value: "collection-fallback"}, {Key: "token", Value: "collection-token"}}
	env := []model.KV{{Key: "value", Value: "old-env"}, {Key: "value", Value: "environment"}, {Key: "fallback", Value: "disabled-env", Disabled: true}, {Key: "token", Value: "environment-token"}}
	beforeCol, beforeEnv := string(jsonBytes(t, col)), string(jsonBytes(t, env))
	r := runner.Run(context.Background(), requestservice.New(), col, model.DefaultSettings(), env, runner.Options{})
	if r.Outcome != runner.Passed || r.Err != nil || len(r.Requests) != 3 {
		t.Fatalf("run = %+v", r)
	}
	want := []received{{"/one", "Bearer environment-token", "environment", "collection-fallback"}, {"/two", "", "environment", "collection-fallback"}, {"/three", "Bearer root-token", "environment", "collection-fallback"}}
	for i := range want {
		if actual := <-got; actual != want[i] {
			t.Fatalf("request %d = %+v, want %+v", i+1, actual, want[i])
		}
		if r.Requests[i].Ordinal != i+1 || r.Requests[i].ItemID != []string{"one", "two", "three"}[i] || !r.Requests[i].Sent {
			t.Fatalf("lost stable item identity: %+v", r.Requests[i])
		}
	}
	if beforeCol != string(jsonBytes(t, col)) || beforeEnv != string(jsonBytes(t, env)) {
		t.Fatal("runner changed caller input")
	}
}

func TestAllInputsSnapshotBeforeFirstSend(t *testing.T) {
	paths := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		paths <- r.URL.Path + ":" + r.Header.Get("Authorization") + ":" + r.Header.Get("X-Value")
	}))
	defer srv.Close()
	col := collection(item("one", "{{host}}/one"), item("two", "{{host}}/two"))
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
	col.Variables = []model.KV{{Key: "host", Value: srv.URL}, {Key: "token", Value: "original"}}
	for i := range col.Items {
		col.Items[i].Request.Auth.Type = model.AuthInherit
		col.Items[i].Request.Headers = []model.KV{{Key: "X-Value", Value: "{{value}}"}}
	}
	env := []model.KV{{Key: "value", Value: "environment"}}
	settings := model.Settings{RequestTimeoutSec: 3, NoFollowRedirects: true, InsecureTLS: true}
	underlying, sends := requestservice.New(), 0
	sender := requestFunc(func(ctx context.Context, req model.Request, values []model.KV) (model.Response, error) {
		sends++
		if req.Options.TimeoutMs != 3000 || req.Options.FollowRedirects != model.ToggleOff || req.Options.VerifyTLS != model.ToggleOff || len(values) != 0 {
			t.Errorf("unsnapshotted options or extra expansion: %+v, %v", req.Options, values)
		}
		if sends == 1 {
			col.Items[1].Request.URL = "bad://changed"
			col.Items[1].Request.Headers[0].Value = "changed"
			col.Items[1].Request.Auth = model.Auth{Type: "changed"}
			col.Auth.Token, col.Variables[0].Value, env[0].Value = "changed", "changed", "changed"
			settings.RequestTimeoutSec, settings.InsecureTLS, settings.NoFollowRedirects = 9, false, false
		}
		return underlying.Send(ctx, req, values)
	})
	r := runner.Run(context.Background(), sender, col, settings, env, runner.Options{})
	if r.Outcome != runner.Passed || sends != 2 {
		t.Fatalf("run did not retain full snapshot: %+v", r)
	}
	for _, want := range []string{"/one:Bearer original:environment", "/two:Bearer original:environment"} {
		if actual := <-paths; actual != want {
			t.Fatalf("received %q, want %q", actual, want)
		}
	}
}

func TestAssertionFailureContinueAndStopStrategy(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	col := collection(item("one", srv.URL), item("two", srv.URL))
	col.Items[0].Request.TestScript = `pm.test("must be 200", function(){ pm.response.to.have.status(200); }); pm.test("503 is visible", function(){ pm.response.to.have.status(503); });`
	for _, stop := range []bool{false, true} {
		calls.Store(0)
		r := run(context.Background(), col, runner.Options{StopOnFailure: stop})
		if r.Outcome != runner.Failed || r.Err != nil || r.Requests[0].Code != runner.CodeAssertion || r.Requests[0].Execution.Response.StatusCode != 503 {
			t.Fatalf("assertion lost or status reclassified: %+v", r)
		}
		report := r.Report()
		if report.Assertions != (runner.Assertions{Passed: 1, Failed: 1}) || report.Counts.Failed != 1 {
			t.Fatalf("assertion counts = %+v", report)
		}
		if stop {
			if calls.Load() != 1 || r.Requests[1].Code != runner.CodeSkippedFailure || r.Requests[1].Execution != nil || report.Counts.Skipped != 1 {
				t.Fatalf("failure did not stop subsequent request: %+v", r)
			}
		} else if calls.Load() != 2 || r.Requests[1].Outcome != runner.Passed {
			t.Fatalf("default continue or HTTP status policy broken: %+v", r)
		}
	}
}

func TestPreScriptFailureNeverReachesEndpoint(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()
	col := collection(item("one", srv.URL), item("two", srv.URL))
	col.Items[0].Request.PreScript = `console.log("diagnostic"); throw new Error("pre failed");`
	col.Items[0].Request.TestScript = `throw new Error("must not run");`
	r := run(context.Background(), col, runner.Options{})
	first := r.Requests[0]
	if r.Outcome != runner.Failed || calls.Load() != 1 || first.Code != runner.CodePreScript || first.Sent || first.Execution.Pre.Err == nil || first.Execution.Tests.Err != nil || first.Execution.Response != nil || first.Execution.TransportStarted {
		t.Fatalf("pre-script failure sent or ran tests: %+v", r)
	}
	if first.Execution.Pre.Logs[0] != "diagnostic" || r.Requests[1].Outcome != runner.Passed {
		t.Fatal("detailed diagnostics or continuation missing")
	}
}

func TestFailureCategoriesAndPostScriptCapabilityChecks(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()
	for _, tc := range []struct {
		name string
		edit func(*model.Request)
		code runner.Code
		sent bool
	}{
		{"auth", func(r *model.Request) { r.Auth = model.Auth{Type: model.AuthBearer, Token: "invalid\r\nheader"} }, runner.CodeAuth, false},
		{"method", func(r *model.Request) { r.Method = "WS" }, runner.CodeUnsupportedMethod, false},
		{"grpc", func(r *model.Request) {
			r.Method = model.MethodGRPC
			r.PreScript = `throw new Error("gRPC must not run HTTP scripts");`
		}, runner.CodeUnsupportedProtocol, false},
		{"protocol", func(r *model.Request) { r.URL = "ws://localhost/socket" }, runner.CodeUnsupportedProtocol, false},
		{"script-method", func(r *model.Request) { r.PreScript = `pm.request.method = "TRACE";` }, runner.CodeUnsupportedMethod, false},
		{"script-protocol", func(r *model.Request) { r.PreScript = `pm.request.url = "ftp://localhost/data";` }, runner.CodeUnsupportedProtocol, false},
		{"test-script", func(r *model.Request) {
			r.TestScript = `pm.test("failed", function(){ pm.expect(false).to.equal(true); }); throw new Error("test exception");`
		}, runner.CodeTestScript, true},
		{"invalid-url", func(r *model.Request) { r.URL = "http://[invalid" }, runner.CodeTransport, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			col := collection(item("one", srv.URL))
			tc.edit(col.Items[0].Request)
			r := run(context.Background(), col, runner.Options{})
			if r.Outcome != runner.Failed || r.Requests[0].Code != tc.code || r.Requests[0].Sent != tc.sent || r.Err != nil {
				t.Fatalf("wrong category: %+v", r.Requests[0])
			}
			if tc.code == runner.CodeTestScript && r.Report().Assertions.Failed != 1 {
				t.Fatal("test exception erased preceding failed assertion")
			}
		})
	}
	if calls.Load() != 1 {
		t.Fatalf("unsupported/preflight requests reached endpoint: %d", calls.Load())
	}
	// Failed inherited-auth construction also gets a stable auth category.
	col := collection(item("", srv.URL))
	col.Items[0].Request.Auth.Type = model.AuthInherit
	r := run(context.Background(), col, runner.Options{})
	if r.Requests[0].Code != runner.CodeAuth || r.Requests[0].Stage != "prepare" || r.Requests[0].Err == nil || r.Requests[0].Execution != nil {
		t.Fatalf("construction failure = %+v", r.Requests[0])
	}
}

func TestCancellationStopsRemainingRequestsAndIsPrimary(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "continue", true: "stop"}[stop], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				cancel()
				<-r.Context().Done()
			}))
			defer srv.Close()
			col := collection(item("one", srv.URL), item("two", srv.URL), item("three", srv.URL))
			col.Items[0].Request.TestScript = `throw new Error("must not run");`
			r := run(ctx, col, runner.Options{StopOnFailure: stop})
			if r.Outcome != runner.Cancelled || !errors.Is(r.Err, context.Canceled) || calls.Load() != 1 || r.Requests[0].Code != runner.CodeCancelled || r.Requests[0].Execution.Tests.Err != nil {
				t.Fatalf("cancellation not primary: %+v", r)
			}
			for _, entry := range r.Requests[1:] {
				if entry.Code != runner.CodeSkippedCancellation || entry.Sent || entry.Execution != nil {
					t.Fatalf("subsequent request was attempted: %+v", entry)
				}
			}
			if r.Report().Counts != (runner.Counts{Total: 3, Cancelled: 1, Skipped: 2}) {
				t.Fatalf("counts = %+v", r.Report().Counts)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := runner.Run(ctx, requestFunc(func(context.Context, model.Request, []model.KV) (model.Response, error) {
		t.Fatal("pre-cancelled run called sender")
		return model.Response{}, nil
	}), collection(item("one", "http://localhost")), model.Settings{}, nil, runner.Options{})
	if r.Outcome != runner.Cancelled || r.Requests[0].Code != runner.CodeSkippedCancellation {
		t.Fatalf("pre-cancelled run = %+v", r)
	}
}

func TestRequestLocalScriptVariablesAndUnchangedInputs(t *testing.T) {
	got := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got <- r.URL.Path + ":" + r.Header.Get("X-Value")
	}))
	defer srv.Close()
	col := collection(item("one", "{{host}}/{{path}}"), item("two", "{{host}}/{{path}}"))
	col.Variables = []model.KV{{Key: "host", Value: srv.URL}, {Key: "path", Value: "collection"}}
	env := []model.KV{{Key: "path", Value: "environment"}}
	col.Items[0].Request.PreScript = `pm.environment.set("path", "runtime"); pm.request.headers.upsert({key:"X-Value", value:"{{path}}"});`
	col.Items[0].Request.TestScript = `pm.test("pre scope visible", function(){ pm.expect(pm.variables.get("path")).to.equal("runtime"); }); pm.variables.set("path", "response-script");`
	col.Items[1].Request.Headers = []model.KV{{Key: "X-Value", Value: "{{path}}"}}
	col.Items[1].Request.TestScript = `pm.test("original scope", function(){ pm.expect(pm.variables.get("path")).to.equal("environment"); });`
	beforeCol, beforeEnv := string(jsonBytes(t, col)), string(jsonBytes(t, env))
	r := runner.Run(context.Background(), requestservice.New(), col, model.Settings{}, env, runner.Options{})
	if r.Outcome != runner.Passed || <-got != "/runtime:runtime" || <-got != "/environment:environment" {
		t.Fatalf("script mutations crossed requests: %+v", r)
	}
	if beforeCol != string(jsonBytes(t, col)) || beforeEnv != string(jsonBytes(t, env)) {
		t.Fatal("script mutation changed original collection/environment")
	}
	if r.Requests[0].Execution.Variables[len(col.Variables)].Value != "runtime" {
		t.Fatal("response-script mutations persisted in runtime pre scope")
	}
}

func TestDeadlineAndTransportFailureRemainDistinct(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	col := collection(item("one", "http://localhost"), item("two", "http://localhost"))
	r := run(ctx, col, runner.Options{})
	if r.Outcome != runner.Cancelled || !errors.Is(r.Err, context.DeadlineExceeded) {
		t.Fatalf("parent deadline = %+v", r)
	}
	// A request timeout while the parent stays live is a transport failure,
	// so the default strategy continues to the next request.
	sends := 0
	r = runner.Run(context.Background(), requestFunc(func(context.Context, model.Request, []model.KV) (model.Response, error) {
		sends++
		return model.Response{}, context.DeadlineExceeded
	}), col, model.Settings{}, nil, runner.Options{})
	if r.Outcome != runner.Failed || r.Err != nil || sends != 2 || r.Requests[0].Code != runner.CodeTransport {
		t.Fatalf("request timeout cancelled the parent run: %+v", r)
	}
}

func TestCancellationDuringPreScriptOutranksScriptFailure(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()
	col := collection(item("one", srv.URL), item("two", srv.URL))
	col.Items[0].Request.PreScript = `while (true) {}`
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	r := run(ctx, col, runner.Options{StopOnFailure: true})
	if r.Outcome != runner.Cancelled || !errors.Is(r.Err, context.DeadlineExceeded) || calls.Load() != 0 || (r.Requests[0].Code != runner.CodeCancelled && r.Requests[0].Code != runner.CodeSkippedCancellation) || r.Requests[1].Code != runner.CodeSkippedCancellation {
		t.Fatalf("script timeout hid parent cancellation: %+v", r)
	}
}

func TestEmptyCollectionSemantics(t *testing.T) {
	col := collection(model.Item{Type: model.ItemFolder, ID: "empty"})
	r := run(context.Background(), col, runner.Options{})
	if r.Outcome != runner.Passed || len(r.Requests) != 0 || r.Err != nil || string(jsonBytes(t, r.Report().Requests)) != "[]" {
		t.Fatalf("empty run = %+v", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = run(ctx, col, runner.Options{})
	if r.Outcome != runner.Cancelled || !errors.Is(r.Err, context.Canceled) {
		t.Fatal("empty collection masked cancellation")
	}
}

func TestSafeVersionedReportOmitsAllUntrustedStrings(t *testing.T) {
	const secret = "secret-canary-NEVER-EXPORT"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Set-Cookie", "session="+secret)
		_, _ = w.Write([]byte(secret))
	}))
	defer srv.Close()
	col := collection(item(secret+"-request-id", "{{host}}/"+secret), item(secret+"-transport-id", "http://localhost"))
	col.ID, col.Name, col.Items[0].Name, col.Items[0].Request.Name = secret+"-collection-id", secret, secret, secret
	col.Items[0].Request.Auth = model.Auth{Type: model.AuthBearer, Token: secret}
	col.Items[0].Request.Headers = []model.KV{{Key: "X-Secret", Value: secret}}
	col.Items[0].Request.PreScript = `console.log("` + secret + `");`
	col.Items[0].Request.TestScript = `console.log(pm.response.text()); pm.test("` + secret + `", function(){ pm.expect("` + secret + `").to.equal("other"); }); throw new Error("` + secret + `");`
	col.Items = []model.Item{{ID: secret + "-folder-id", Name: secret, Type: model.ItemFolder, Items: col.Items}}
	env := []model.KV{{Key: "host", Value: srv.URL}, {Key: secret, Value: secret}}
	underlying := requestservice.New()
	sender := requestFunc(func(ctx context.Context, req model.Request, values []model.KV) (model.Response, error) {
		if strings.HasSuffix(req.ID, "transport-id") {
			return model.Response{}, errors.New("failed URL: " + secret)
		}
		return underlying.Send(ctx, req, values)
	})
	r := runner.Run(context.Background(), sender, col, model.Settings{ActiveEnvID: secret}, env, runner.Options{})
	if r.Requests[0].Execution.Tests.Err == nil || !strings.Contains(r.Requests[0].Execution.Tests.Assertions[0].Name, secret) || !strings.Contains(r.Requests[1].Err.Error(), secret) {
		t.Fatal("secret canaries did not reach detailed diagnostics")
	}
	for _, value := range []any{r, &r, r.Report(), r.Requests, r.Requests[0]} {
		data := string(jsonBytes(t, value))
		for _, forbidden := range []string{secret, srv.URL, "Authorization", "Set-Cookie", "PreScript", "HistoryRequest", "http://localhost", "other", "{{host}}"} {
			if strings.Contains(data, forbidden) {
				t.Fatalf("report contains %q: %s", forbidden, data)
			}
		}
	}
	var decoded runner.Report
	if err := json.Unmarshal(jsonBytes(t, r), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != runner.ReportVersion || decoded.Counts != (runner.Counts{Total: 2, Failed: 2}) || decoded.Assertions.Failed != 1 || decoded.Requests[0].StatusCode != 200 || decoded.Requests[1].Code != runner.CodeTransport {
		t.Fatalf("safe metadata missing: %+v", decoded)
	}
	// Exported enum fields are not a channel for arbitrary caller strings.
	r.Outcome, r.Requests[0].Outcome, r.Requests[0].Code, r.Requests[0].Stage = runner.Outcome(secret), runner.Outcome(secret), runner.Code(secret), execution.Stage(secret)
	if strings.Contains(string(jsonBytes(t, r)), secret) {
		t.Fatal("projection trusted caller-supplied enum text")
	}
}

func TestRealStoreBytesUnchangedAcrossRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"token":"response-secret"}`))
	}))
	defer srv.Close()
	col := collection(item("one", "{{host}}/{{path}}"), item("two", "{{host}}/{{path}}"))
	col.Variables = []model.KV{{Key: "host", Value: srv.URL}, {Key: "path", Value: "original"}}
	col.Items[0].Request.PreScript = `pm.environment.set("path", "runtime");`
	col.Items[0].Request.TestScript = `pm.environment.set("token", pm.response.json().token);`
	env := model.Environment{ID: "environment", Name: "environment", Variables: []model.KV{{Key: "path", Value: "env"}}}
	settings := model.DefaultSettings()
	settings.ActiveEnvID = env.ID
	for _, err := range []error{st.Collections().Save(col), st.Environments().Save(env), st.Settings().Save(settings), st.History().Add(model.HistoryEntry{Request: *col.Items[0].Request, StatusCode: 201})} {
		if err != nil {
			t.Fatal(err)
		}
	}
	before := directoryBytes(t, dir)
	savedCol, okCol := st.Collections().Get(col.ID)
	savedEnv, okEnv := st.Environments().Get(env.ID)
	if !okCol || !okEnv {
		t.Fatal("failed loading actual stored inputs")
	}
	r := runner.Run(context.Background(), requestservice.New(), savedCol, st.Settings().Get(), savedEnv.Variables, runner.Options{})
	if r.Outcome != runner.Passed {
		t.Fatalf("stored run = %+v", r)
	}
	if !reflect.DeepEqual(before, directoryBytes(t, dir)) {
		t.Fatal("run changed data files or created a new file")
	}
	if len(st.History().List()) != 1 {
		t.Fatal("run wrote history")
	}
}

func directoryBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	contents := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err == nil {
			contents[strings.TrimPrefix(path, dir)] = string(data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

var _ service.RequestService = requestFunc(nil)
