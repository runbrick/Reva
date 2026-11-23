package gui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/request"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
)

func newSSEHarness(t *testing.T, handler http.HandlerFunc) (*harness, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	services := fake.New()
	services.Requests = request.New()
	h := newHarness(t, services)
	requestEnglish(h)
	return h, server.URL
}

func flushSSE(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	io.WriteString(w, body)
	w.(http.Flusher).Flush()
}

func TestNewSSERequestStreamsAndDisconnects(t *testing.T) {
	closed := make(chan struct{})
	h, url := newSSEHarness(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(closed)
		if r.Method != http.MethodGet || r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("request = %s, Accept = %q", r.Method, r.Header.Get("Accept"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		flushSSE(w, "data: subscribed from menu\n\n")
		<-r.Context().Done()
	})
	h.click(h.App.S.Collections.Add)
	collectionsOption(h, h.App.S.Collections.NewSSE)
	tab := h.App.Active()
	if len(h.App.Tabs()) != 2 || tab.Request.Name != h.App.S.Collections.NewSSE || tab.CollectionID != "" || !tab.Dirty {
		t.Fatalf("SSE draft = %+v", tab)
	}
	h.typeInto(h.App.S.Request.URL, url)
	h.click(h.App.S.Request.Send)
	h.waitFor("live SSE event", func() bool { return tab.Sending && h.shows("subscribed from menu") })
	h.click(h.App.S.Response.SSEDisconnect)
	h.waitFor("stream disconnected", func() bool { return !tab.Sending && h.shows(h.App.S.Response.SSEDisconnected) })
	assertSSEClosed(t, closed)
}

func TestNewSSERequestInFolderSavesAndReopens(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	disk, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := fake.New()
	svc.Collections = disk.Collections()
	collection, err := svc.Collections.Create("API")
	if err != nil {
		t.Fatal(err)
	}
	folder, err := svc.Collections.AddFolder(collection.ID, "", "Streams")
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	label := h.App.S.Collections.NewSSE
	collectionsContext(h, folder.Name, label)
	first := h.App.Active()
	if first.CollectionID != collection.ID || first.Request.Auth.Type != model.AuthInherit {
		t.Fatalf("collection SSE request = %+v", first)
	}
	const url = "https://example.test/events"
	h.typeInto(h.App.S.Request.URL, url)
	h.click(h.App.S.Request.Save)
	h.click(label)
	h.click(h.App.S.Collections.Add)
	collectionsOption(h, label)
	second := h.App.Active()
	if second.Request.ID == first.Request.ID {
		t.Fatal("creating an SSE request with a sibling selected reused the sibling")
	}
	h.D.Close()
	disk, err = store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc.Collections = disk.Collections()
	stored, ok := svc.Collections.Get(collection.ID)
	parent := model.FindItem(stored.Items, folder.ID)
	if !ok || parent == nil || len(parent.Items) != 2 {
		t.Fatalf("saved folder = %+v", parent)
	}
	h = newHarness(t, svc)
	h.click(label)
	reopened := h.App.Active().Request
	if reopened.ID != first.Request.ID || reopened.URL != url || reopened.Method != model.MethodGet || len(reopened.Headers) != 1 || reopened.Headers[0] != (model.KV{Key: "Accept", Value: "text/event-stream"}) {
		t.Fatalf("reopened SSE request = %+v", reopened)
	}
}

func TestResponseSSEEarlyThenFinalThroughExecution(t *testing.T) {
	more := make(chan struct{})
	finish := make(chan struct{})
	const first = "id: alpha\r\nevent: update\r\ndata: private-first\r\ndata: multiline\r\n\r\n"
	const second = "data: second-event\n\n"
	h, url := newSSEHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/events", http.StatusTemporaryRedirect)
			return
		}
		if r.Header.Get("Authorization") != "Bearer runtime-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("X-Stream", "from-server")
		flushSSE(w, first)
		select {
		case <-more:
			io.WriteString(w, second)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
		select {
		case <-finish:
		case <-r.Context().Done():
		}
	})
	tab := h.App.Active()
	tab.Request.URL = url + "/draft"
	tab.Request.Auth = model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
	tab.Request.PreScript = `pm.request.url = "` + url + `/redirect"; pm.variables.set("token", "runtime-token");`
	tab.Request.TestScript = `pm.test("stream response", function(){ pm.response.to.have.status(200); });`
	requestRepaint(h)
	h.click("Send")
	h.waitFor("early stream event", func() bool { return h.shows("private-first") && h.shows("multiline") })
	if !tab.Sending || tab.Response != nil || !h.shows("Type: update") || !h.shows("ID: alpha") || len(h.Svc.History.List()) != 0 || len(tab.Tests) != 0 {
		t.Fatal("live event or execution timing was incorrect")
	}
	close(more)
	h.waitFor("second live batch", func() bool { return len(tab.sse.events) == 2 && tab.Sending })
	h.click("Event 2")
	if !h.shows("second-event") || !h.shows("ID: alpha") {
		t.Fatal("second event and effective ID are unavailable")
	}
	close(finish)
	h.waitFor("final HTTP response", func() bool { return !tab.Sending && tab.Response != nil && h.shows("Stream ended") })
	if len(tab.Tests) != 1 || !tab.Tests[0].Passed || string(tab.Response.Body) != first+second || tab.Response.URL != url+"/events" {
		t.Fatalf("final body, URL, or tests incorrect: %#v, %#v", tab.Response, tab.Tests)
	}
	responseClickTab(h, h.App.S.Response.Headers)
	if !h.shows("from-server") {
		t.Fatal("final HTTP headers are unavailable")
	}
	history := h.Svc.History.List()
	if len(history) != 1 || history[0].StatusCode != 200 || history[0].Duration <= 0 || history[0].Error != "" ||
		history[0].Request.URL != url+"/draft" || history[0].Request.Auth.Token != "{{token}}" {
		t.Fatalf("history = %#v", history)
	}
}

func TestResponseSSETabOwnershipAndDisconnect(t *testing.T) {
	oldClosed, newClosed, otherClosed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	h, url := newSSEHarness(t, func(w http.ResponseWriter, r *http.Request) {
		closed := map[string]chan struct{}{"/old": oldClosed, "/new": newClosed, "/other": otherClosed}[r.URL.Path]
		defer close(closed)
		flushSSE(w, "data: marker"+r.URL.Path+"\n\n")
		<-r.Context().Done()
	})
	first := h.App.Active()
	first.Request.URL = url + "/old"
	h.App.Send(first)
	requestRepaint(h)
	h.waitFor("first tab event", func() bool { return h.shows("marker/old") })
	second := h.App.NewTab()
	second.Request.URL = url + "/other"
	h.App.Send(second)
	requestRepaint(h)
	h.waitFor("second tab event", func() bool { return h.shows("marker/other") })
	if h.shows("marker/old") || !first.Sending {
		t.Fatal("switching tabs moved or disconnected the old stream")
	}
	h.App.active = 0
	requestRepaint(h)
	if !h.shows("marker/old") || h.shows("marker/other") {
		t.Fatal("first tab did not retain its own event")
	}
	first.Request.URL = url + "/new"
	h.App.Send(first)
	requestRepaint(h)
	h.waitFor("new generation event", func() bool { return h.shows("marker/new") })
	assertSSEClosed(t, oldClosed)
	if h.shows("marker/old") || len(first.sse.events) != 1 || !second.Sending {
		t.Fatal("resend retained stale events or changed the other tab")
	}
	h.click("Disconnect")
	h.waitFor("disconnected state", func() bool { return h.shows("Disconnected") })
	assertSSEClosed(t, newClosed)
	if first.Sending || !h.shows("marker/new") || !second.Sending {
		t.Fatal("disconnect reset inspected events or changed the other tab")
	}
	h.App.CloseTab(1)
	requestRepaint(h)
	assertSSEClosed(t, otherClosed)
	if len(h.Svc.History.List()) != 0 {
		t.Fatal("canceled streams entered history")
	}
}

func assertSSEClosed(t *testing.T, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("underlying HTTP request did not close")
	}
}

func TestResponseSSELimitedStreams(t *testing.T) {
	for _, test := range []struct {
		name, tail string
		wantCount  int
		wantError  bool
	}{
		{"high rate", strings.Repeat("data: next\n\n", 10000), maxSSEEvents, false},
		{"oversized line", "data:" + strings.Repeat("a", 32<<10), 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h, url := newSSEHarness(t, func(w http.ResponseWriter, _ *http.Request) {
				flushSSE(w, "data: first-kept\n\n"+test.tail)
			})
			tab := h.App.Active()
			tab.Request.URL = url
			h.App.Send(tab)
			requestRepaint(h)
			h.waitFor("bounded stream", func() bool { return !tab.Sending && h.shows("inspection limits") })
			if len(tab.sse.events) != test.wantCount || tab.sse.mailbox != nil || !h.shows("first-kept") || tab.Response == nil || (tab.Err != "") != test.wantError {
				t.Fatalf("stream count = %d, mailbox present = %t, response present = %t, error = %q", len(tab.sse.events), tab.sse.mailbox != nil, tab.Response != nil, tab.Err)
			}
		})
	}
}

func TestResponseSSETimeoutKeepsPartialHTTPResponse(t *testing.T) {
	closed := make(chan struct{})
	h, url := newSSEHarness(t, func(w http.ResponseWriter, r *http.Request) {
		defer close(closed)
		flushSSE(w, "data: before-timeout\n\n")
		<-r.Context().Done()
	})
	tab := h.App.Active()
	tab.Request.URL = url
	tab.Request.Options.TimeoutMs = 500
	tab.Request.TestScript = `pm.test("requires successful completion", function(){});`
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("timed out stream", func() bool { return !tab.Sending && h.shows("Stream timed out") })
	assertSSEClosed(t, closed)
	history := h.Svc.History.List()
	if !h.shows("before-timeout") || tab.Response == nil || tab.Response.StatusCode != 200 || len(tab.Tests) != 0 ||
		len(history) != 1 || history[0].Error == "" || history[0].StatusCode != 200 {
		t.Fatal("timeout did not preserve available events and HTTP metadata or ran response tests")
	}
}

func TestSSEMailboxBoundAcrossDrains(t *testing.T) {
	state := newSSEState()
	mailbox := state.mailbox
	value := strings.Repeat("x", maxSSEBytes/8)
	for range 4 {
		mailbox.push(request.SSEEvent{Type: value, ID: value})
	}
	if len(mailbox.ready) != 1 {
		t.Fatal("pending items did not coalesce to one notification")
	}
	state.adopt(mailbox.drain(false))
	for range 10000 {
		mailbox.push(request.SSEEvent{Data: "unretained"})
	}
	state.adopt(mailbox.drain(false))
	if len(state.events) != 4 || !state.overflow || mailbox.bytes != maxSSEBytes {
		t.Fatalf("metadata budget failed, kept = %d, bytes = %d, overflow = %t", len(state.events), mailbox.bytes, state.overflow)
	}
	state.release()
	mailbox.push(request.SSEEvent{Data: value})
	if len(mailbox.ready) != 0 || len(mailbox.pending.events) != 0 {
		t.Fatal("closed mailbox kept a publication")
	}
}

func TestResponseSSEPreservesOrdinarySendAndDownload(t *testing.T) {
	finish := make(chan struct{})
	started := make(chan struct{})
	body := "data:" + strings.Repeat("a", 32<<10) + "\n\n"
	h, url := newSSEHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ordinary" {
			w.Header().Set("Content-Type", "text/plain")
			io.WriteString(w, "ordinary-response")
			return
		}
		flushSSE(w, body)
		close(started)
		select {
		case <-finish:
		case <-r.Context().Done():
		}
	})
	tab := h.App.Active()
	tab.Request.URL = url + "/ordinary"
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("ordinary body", func() bool { return h.shows("ordinary-response") })
	if tab.sse != nil || h.shows("SSE events") {
		t.Fatal("ordinary send adopted an SSE view")
	}
	tab.Request.URL = url + "/download"
	path := filepath.Join(t.TempDir(), "events.txt")
	if err := h.D.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	requestRepaint(h)
	h.click("Send options")
	h.click("Send and download")
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("Send and download did not start")
	}
	h.pump()
	if !tab.Sending || tab.sse != nil || h.shows("SSE events") {
		t.Fatal("Send and download subscribed to a stream")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("download wrote before EOF: %v", err)
	}
	close(finish)
	h.waitFor("downloaded raw body", func() bool { data, err := os.ReadFile(path); return err == nil && string(data) == body })
}
