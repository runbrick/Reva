package gui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	requestservice "github.com/runbrick/reva/internal/service/request"
)

func TestExecutionGUIAbortsPreErrorBeforeHTTPAndResetsDiagnostics(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(201)
	}))
	defer srv.Close()
	svc := fake.New()
	svc.Requests = requestservice.New()
	h := newHarness(t, svc)
	tab := h.App.Active()
	tab.Request.URL = srv.URL
	tab.Request.PreScript = `console.log("before abort"); throw new Error("pre abort");`
	tab.Request.TestScript = `throw new Error("must not run");`
	h.pump()
	h.click("Send")
	h.waitFor("aborted script", func() bool { return !tab.Sending })
	if calls.Load() != 0 || tab.Response != nil || len(svc.History.List()) != 0 || !h.shows("before abort") || !h.shows("pre abort") || h.shows("must not run") {
		t.Fatal("pre error sent HTTP, lost diagnostics, or ran response tests")
	}
	tab.Request.PreScript, tab.Request.TestScript = "", ""
	h.click("Send")
	h.waitFor("successful retry", func() bool { return !tab.Sending })
	if calls.Load() != 1 || tab.Response == nil || tab.Response.StatusCode != 201 || len(tab.Tests) != 0 || h.shows("before abort") || tab.Err != "" || len(svc.History.List()) != 1 {
		t.Fatalf("retry did not reset aborted state: %+v", tab)
	}
}

func TestExecutionGUIHistoryKeepsOriginalSendStartDraft(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	received := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.URL.Path + "|" + r.Header.Get("X-Secret")
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(202)
	}))
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		srv.Close()
	}()
	svc := fake.New()
	svc.Requests = requestservice.New()
	h := newHarness(t, svc)
	tab := h.App.Active()
	tab.Request.URL = srv.URL + "/draft"
	tab.Request.Headers = []model.KV{{Key: "X-Original", Value: "placeholder"}}
	tab.Request.PreScript = `pm.request.url = "` + srv.URL + `/runtime";
pm.variables.set("secret", "runtime-secret"); pm.request.headers.add({key:"X-Secret", value:"{{secret}}"});`
	tab.Request.TestScript = `pm.test("response", function(){ pm.response.to.have.status(202); });`
	original := tab.Request.Clone()
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("HTTP endpoint", func() bool { return len(received) > 0 })
	<-started
	if got := <-received; got != "/runtime|runtime-secret" {
		t.Fatalf("HTTP received %q", got)
	}
	// Edits during a send must not become its history record or test source.
	tab.Request.URL, tab.Request.TestScript = srv.URL+"/later", `throw new Error("later editor");`
	close(release)
	h.waitFor("completed send", func() bool { return !tab.Sending })
	entries := svc.History.List()
	if len(entries) != 1 || !reflect.DeepEqual(entries[0].Request, original) || entries[0].StatusCode != 202 || len(tab.Tests) != 1 || !tab.Tests[0].Passed {
		t.Fatalf("history/test source was not send-start snapshot: entries=%+v tests=%+v", entries, tab.Tests)
	}
}

func TestExecutionGUICancelledOldJobCannotReplaceRetry(t *testing.T) {
	h := newHarness(t)
	oldStarted, oldRelease, oldReturned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-oldRelease:
		default:
			close(oldRelease)
		}
	})
	h.Svc.Requests.(*fake.Requests).Fn = func(_ context.Context, req model.Request) (model.Response, error) {
		if strings.Contains(req.URL, "old") {
			close(oldStarted)
			<-oldRelease // Deliberately simulate a transport slow to notice cancellation.
			close(oldReturned)
			return model.Response{StatusCode: 200, Body: []byte("stale")}, nil
		}
		return model.Response{StatusCode: 201, Body: []byte("current")}, nil
	}
	tab := h.App.Active()
	tab.Request.URL = "https://example.test/old"
	tab.Request.PreScript = `console.log("old log");`
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("old send", func() bool {
		select {
		case <-oldStarted:
			return true
		default:
			return false
		}
	})
	h.App.Cancel(tab)
	// Retry immediately, without a paint between cancel and new send.
	tab.Request.URL, tab.Request.PreScript = "https://example.test/new", `console.log("new log");`
	tab.Request.TestScript = `pm.test("new response", function(){ pm.response.to.have.status(201); });`
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("retry", func() bool { return !tab.Sending })
	close(oldRelease)
	h.waitFor("late old result", func() bool {
		select {
		case <-oldReturned:
			return true
		default:
			return false
		}
	})
	h.pump()
	if tab.Response == nil || string(tab.Response.Body) != "current" || len(tab.Tests) != 1 || !tab.Tests[0].Passed || tab.Err != "" || len(h.Svc.History.List()) != 1 {
		t.Fatalf("stale result replaced retry: %+v", tab)
	}
	logs := h.App.scriptTab(tab).logs
	if len(logs) != 1 || logs[0] != "new log" {
		t.Fatalf("stale script logs: %v", logs)
	}
}

func TestExecutionGUIScriptsDoNotBlockSendOrCancel(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	tab.Request.PreScript = `while (true) {}`
	h.App.Send(tab)
	// No frame has declared the effect: scripts cannot have run yet.
	if !tab.Sending || h.App.scriptTab(tab).preError != "" {
		t.Fatal("Send executed the sandbox on the UI thread")
	}
	requestRepaint(h)
	h.App.Cancel(tab)
	requestRepaint(h)
	if tab.Sending || tab.Err != context.Canceled.Error() || tab.Response != nil || len(h.Svc.History.List()) != 0 {
		t.Fatal("cancel did not leave an aborted, responsive tab")
	}
}
