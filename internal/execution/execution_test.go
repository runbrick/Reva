package execution_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/execution"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/script"
	"github.com/runbrick/reva/internal/service"
	requestservice "github.com/runbrick/reva/internal/service/request"
)

func newPlan(t *testing.T, in execution.Input) *execution.Plan {
	t.Helper()
	p, err := execution.New(in)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type requestFunc func(context.Context, model.Request, []model.KV) (model.Response, error)

func (f requestFunc) Send(ctx context.Context, req model.Request, values []model.KV) (model.Response, error) {
	return f(ctx, req, values)
}

func TestScriptedRequestAuthOptionsAndRawHistory(t *testing.T) {
	type received struct{ method, path, body, token, header, literal string }
	requests := make(chan received, 1)
	var redirected atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			redirected.Add(1)
			return
		}
		body, _ := io.ReadAll(r.Body)
		requests <- received{r.Method, r.URL.RequestURI(), string(body), r.Header.Get("Authorization"), r.Header.Get("X-Script"), r.Header.Get("X-Literal")}
		w.Header().Set("Location", "/redirected")
		w.WriteHeader(http.StatusFound)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	req := model.NewRequest("real-id")
	req.Name, req.URL = "same name", "{{host}}/original"
	req.Auth = model.Auth{Type: model.AuthInherit}
	req.Headers = []model.KV{{Key: "X-Literal", Value: "{{outer}}"}}
	req.Body = model.Body{Mode: model.BodyRaw, Text: "{{payload}}"}
	req.PreScript = `pm.request.url = "{{host}}/scripted?x={{path}}";
pm.request.method = "POST";
pm.variables.set("path", "runtime"); pm.variables.set("token", "runtime-secret");
pm.request.headers.add({key:"X-Script", value:"{{path}}"}); console.log("pre");`
	req.TestScript = `pm.test("actual context", function() {
pm.expect(pm.request.url).to.equal(pm.variables.get("host") + "/scripted?x=runtime");
pm.expect(pm.variables.get("path")).to.equal("runtime");
pm.expect(pm.response.code).to.equal(302);
}); console.log("post");`
	col := model.Collection{ID: "col", Auth: model.Auth{Type: model.AuthBearer, Token: "wrong-root"}, Items: []model.Item{
		{ID: "decoy", Type: model.ItemRequest, Name: req.Name, Request: &model.Request{ID: "decoy"}},
		{ID: "folder", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, Items: []model.Item{{ID: req.ID, Type: model.ItemRequest, Name: req.Name, Request: &req}}},
	}}
	values := []model.KV{{Key: "host", Value: srv.URL}, {Key: "token", Value: "original-secret"}, {Key: "path", Value: "original"}, {Key: "payload", Value: "body"}, {Key: "outer", Value: "{{inner}}"}, {Key: "inner", Value: "must-stay-literal"}}
	original, originalValues := req.Clone(), append([]model.KV(nil), values...)
	settings := model.Settings{InsecureTLS: true, NoFollowRedirects: true, RequestTimeoutSec: 1}
	p := newPlan(t, execution.Input{Request: req, Collection: col, Variables: values, Settings: settings})
	r := p.Run(context.Background(), requestservice.New())
	if r.Failed() || r.Response == nil || r.Response.StatusCode != 302 || len(r.Tests.Assertions) != 1 || !r.Tests.Assertions[0].Passed {
		t.Fatalf("execution = %+v, tests = %+v", r, r.Tests)
	}
	got := <-requests
	if got != (received{"POST", "/scripted?x=runtime", "body", "Bearer runtime-secret", "runtime", "{{inner}}"}) || redirected.Load() != 0 {
		t.Fatalf("server received %+v; redirects = %d", got, redirected.Load())
	}
	wantHistory := original.Clone()
	wantHistory.Auth = col.Items[1].Auth
	if !reflect.DeepEqual(r.HistoryRequest, wantHistory) || !reflect.DeepEqual(req, original) || !reflect.DeepEqual(values, originalValues) {
		t.Fatal("runtime mutations leaked into history or input")
	}
	if !strings.Contains(r.Prepared.URL, "{{host}}") || len(r.Pre.Logs) != 1 || len(r.Tests.Logs) != 1 {
		t.Fatalf("missing raw prepared context/logs: %+v", r)
	}
	// Returned snapshots and later caller edits cannot contaminate another run.
	r.Prepared.Headers[0].Value = "changed-result"
	r.Variables[0].Value = "changed-result"
	req.Headers[0].Value, values[0].Value = "changed-input", "changed-input"
	col.Items[1].Auth.Token = "changed-parent"
	r = p.Run(context.Background(), requestservice.New())
	if r.Failed() || (<-requests).token != "Bearer runtime-secret" {
		t.Fatalf("plan was not isolated: %+v", r)
	}
}

func TestPreScriptErrorsAbortBeforeEndpoint(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()
	for _, source := range []string{`console.log("before"); throw new Error("stop");`, `while (true) {}`} {
		req := model.NewRequest("r")
		req.URL, req.PreScript, req.TestScript = srv.URL, source, `throw new Error("must not run");`
		r := newPlan(t, execution.Input{Request: req}).Run(context.Background(), requestservice.New())
		if !r.Failed() || r.Pre.Err == nil || r.Err != nil || r.TransportStarted || r.Response != nil || r.Tests.Err != nil || calls.Load() != 0 {
			t.Fatalf("pre failure sent or ran tests: %+v", r)
		}
		if strings.Contains(source, "console") && (len(r.Pre.Logs) != 1 || r.Pre.Logs[0] != "before") {
			t.Fatalf("lost logs: %+v", r.Pre)
		}
		if strings.Contains(source, "while") && !errors.Is(r.Pre.Err, script.ErrTimeout) {
			t.Fatalf("timeout = %v", r.Pre.Err)
		}
	}
}

func TestResponseAssertionsAndScriptErrorsAreSeparateFromHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }))
	defer srv.Close()
	for _, source := range []string{"", `pm.test("unavailable", function(){ pm.response.to.have.status(503); });`, `pm.test("wrong", function(){ pm.response.to.have.status(200); }); console.log("before error"); throw new Error("test error");`} {
		req := model.NewRequest("r")
		req.URL, req.TestScript = srv.URL, source
		r := newPlan(t, execution.Input{Request: req}).Run(context.Background(), requestservice.New())
		if r.Err != nil || r.Response == nil || r.Response.StatusCode != 503 {
			t.Fatalf("HTTP status became operational failure: %+v", r)
		}
		if strings.Contains(source, "wrong") {
			if !r.Failed() || r.Tests.Err == nil || len(r.Tests.Assertions) != 1 || r.Tests.Assertions[0].Passed || len(r.Tests.Logs) != 1 {
				t.Fatalf("missing test diagnostics: %+v", r.Tests)
			}
		} else if r.Failed() {
			t.Fatalf("503 alone failed execution: %+v", r)
		}
	}
}

func TestSingleExpansionAlsoMatchesResponseScriptContext(t *testing.T) {
	paths := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { paths <- r.URL.Path }))
	defer srv.Close()
	req := model.NewRequest("r")
	req.URL = srv.URL + "/{{outer}}"
	req.TestScript = `pm.test("single expansion", function(){ pm.expect(pm.request.url).to.equal(pm.variables.get("host") + "/{{inner}}"); });`
	values := []model.KV{{Key: "host", Value: srv.URL}, {Key: "outer", Value: "{{inner}}"}, {Key: "inner", Value: "wrong"}}
	r := newPlan(t, execution.Input{Request: req, Variables: values}).Run(context.Background(), requestservice.New())
	if r.Failed() || len(r.Tests.Assertions) != 1 || !r.Tests.Assertions[0].Passed || (<-paths) != "/{{inner}}" {
		t.Fatalf("transport and script expanded differently: %+v", r)
	}
}

func TestAuthValidationUsesPostScriptVariables(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer fixed" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
	}))
	defer srv.Close()
	req := model.NewRequest("r")
	req.URL, req.Auth = srv.URL, model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
	in := execution.Input{Request: req, Variables: []model.KV{{Key: "token", Value: "bad\r\nvalue"}}}
	r := newPlan(t, in).Run(context.Background(), requestservice.New())
	if r.Err == nil || r.Stage != execution.StageValidation || r.TransportStarted || calls.Load() != 0 {
		t.Fatalf("invalid auth reached transport: %+v", r)
	}
	in.Request.PreScript = `pm.variables.set("token", "fixed");`
	r = newPlan(t, in).Run(context.Background(), requestservice.New())
	if r.Failed() || calls.Load() != 1 {
		t.Fatalf("pre-script auth correction failed: %+v", r)
	}
}

func TestInheritedAuthRequiresMembershipAndHonorsNoneBarrier(t *testing.T) {
	req := model.NewRequest("actual")
	req.Auth.Type = model.AuthInherit
	col := model.Collection{ID: "c", Auth: model.Auth{Type: model.AuthBearer, Token: "root"}, Items: []model.Item{{ID: "folder", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthNone}, Items: []model.Item{{ID: req.ID, Type: model.ItemRequest, Request: &req}}}}}
	p := newPlan(t, execution.Input{Request: req, Collection: col})
	if p.Request().Auth.Type != model.AuthNone {
		t.Fatal("none barrier lost")
	}
	for _, id := range []string{"", "same-name-but-not-member"} {
		req.ID = id
		if _, err := execution.New(execution.Input{Request: req, Collection: col}); err == nil {
			t.Fatalf("missing member %q accepted", id)
		}
	}
	req.Auth.Type = model.AuthNone
	if _, err := execution.New(execution.Input{Request: req}); err != nil {
		t.Fatal("explicit no-auth needs no collection", err)
	}
}

func TestCancellationAndTimeoutPreventLaterStages(t *testing.T) {
	t.Run("before pre", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req := model.NewRequest("r")
		req.PreScript = `console.log("must not run");`
		r := newPlan(t, execution.Input{Request: req}).Run(ctx, requestFunc(func(context.Context, model.Request, []model.KV) (model.Response, error) {
			t.Fatal("sent cancelled request")
			return model.Response{}, nil
		}))
		if !errors.Is(r.Err, context.Canceled) || len(r.Pre.Logs) != 0 || r.TransportStarted {
			t.Fatalf("cancellation = %+v", r)
		}
	})
	t.Run("during bounded pre", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		req := model.NewRequest("r")
		req.PreScript = `while (true) {}`
		r := newPlan(t, execution.Input{Request: req}).Run(ctx, requestFunc(func(context.Context, model.Request, []model.KV) (model.Response, error) {
			t.Fatal("sent after pre-script cancellation")
			return model.Response{}, nil
		}))
		if !errors.Is(r.Err, context.DeadlineExceeded) || !errors.Is(r.Pre.Err, script.ErrTimeout) || r.TransportStarted {
			t.Fatalf("cancelled pre-script = %+v", r)
		}
	})
	t.Run("during transport", func(t *testing.T) {
		started := make(chan struct{})
		finished := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
			close(finished)
		}))
		defer srv.Close()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := model.NewRequest("r")
		req.URL, req.TestScript = srv.URL, `throw new Error("must not run");`
		result := make(chan execution.Result, 1)
		plan := newPlan(t, execution.Input{Request: req})
		go func() { result <- plan.Run(ctx, requestservice.New()) }()
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("endpoint did not start")
		}
		cancel()
		select {
		case r := <-result:
			if !errors.Is(r.Err, context.Canceled) || !r.TransportStarted || r.Tests.Err != nil || r.Response != nil {
				t.Fatalf("cancellation = %+v", r)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("cancellation did not stop send")
		}
		<-finished
	})
	t.Run("after response before tests", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req := model.NewRequest("r")
		req.TestScript = `throw new Error("must not run");`
		r := newPlan(t, execution.Input{Request: req}).Run(ctx, requestFunc(func(context.Context, model.Request, []model.KV) (model.Response, error) {
			cancel()
			return model.Response{StatusCode: 200}, nil
		}))
		if !errors.Is(r.Err, context.Canceled) || r.Tests.Err != nil {
			t.Fatalf("tests ran after cancellation: %+v", r)
		}
	})
	t.Run("request timeout", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer srv.Close()
		req := model.NewRequest("r")
		req.URL, req.Options.TimeoutMs = srv.URL, 30
		r := newPlan(t, execution.Input{Request: req}).Run(context.Background(), requestservice.New())
		if !errors.Is(r.Err, context.DeadlineExceeded) || !r.TransportStarted || r.Response != nil {
			t.Fatalf("timeout = %+v", r)
		}
	})
}

var _ service.RequestService = requestFunc(nil)
