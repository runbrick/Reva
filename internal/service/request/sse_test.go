package request

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
)

func TestSSEFraming(t *testing.T) {
	input := "\uFEFF: heartbeat\r\nretry: 100\runknown: ignored\nid: 42\r\nevent: custom\ndata: first\rdata:  second: value\r\n\r\n" +
		"event: unused\nid: 43\n\nid: ignored\x00id\ndata\n\n" +
		"id:\nevent:\ndata: invalid \xe2\x82!\r\rdata: unterminated\n"
	var got []SSEEvent
	err := readSSE(context.Background(), strings.NewReader(input), func(evt SSEEvent) { got = append(got, evt) })
	want := []SSEEvent{
		{Type: "custom", ID: "42", Data: "first\n second: value"},
		{Type: "message", ID: "43", Data: ""},
		{Type: "message", Data: "invalid \uFFFD!"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %#v, error = %v, want %#v", got, err, want)
	}
}

func TestSSEByteLimits(t *testing.T) {
	for _, test := range []struct {
		name, input string
	}{
		{"line", ":" + strings.Repeat("a", maxSSELineBytes)},
		{"data", strings.Repeat("data:"+strings.Repeat("a", maxSSELineBytes-5)+"\n", 3)},
		{"metadata", "id:" + strings.Repeat("a", 16<<10-3) + "\nevent:" + strings.Repeat("\xff", 8<<10) + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			count := 0
			err := readSSE(context.Background(), strings.NewReader("data:ok\n\n"+test.input), func(SSEEvent) { count++ })
			if !errors.Is(err, ErrSSELimit) || count != 1 {
				t.Fatalf("error = %v, dispatched = %d", err, count)
			}
		})
	}
}

func TestSendSSEBeforeEOFAndCancel(t *testing.T) {
	closed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(closed)
		w.Header().Set("Content-Type", "Text/Event-Stream; charset=utf-8")
		io.WriteString(w, "data: early\r\r")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events := make(chan SSEEvent, 1)
	result := make(chan error, 1)
	go func() {
		_, err := New().SendSSE(ctx, model.Request{URL: server.URL}, nil, SSEObserver{
			Event: func(event SSEEvent) { events <- event },
		})
		result <- err
	}()
	select {
	case event := <-events:
		if event.Data != "early" {
			t.Fatalf("event = %#v", event)
		}
	case err := <-result:
		t.Fatalf("transport ended before the event: %v", err)
	case <-ctx.Done():
		t.Fatal("event did not arrive while the connection was open")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("send error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not return from SendSSE")
	}
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("server connection did not close")
	}
}

func TestSendSSEMIMEAndFinalBody(t *testing.T) {
	body := strings.Repeat("data:"+strings.Repeat("a", 250)+"\n\n", 1024)
	for _, test := range []struct {
		name, mimeType string
		interactive    bool
		wantEvents     int
	}{
		{"stream", "text/event-stream; charset=utf-8", true, 1024},
		{"lookalike", "text/event-stream-test", true, 0},
		{"invalid MIME", "text/event-stream; broken", true, 0},
		{"ordinary Send", "text/event-stream", false, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.mimeType)
				w.WriteHeader(http.StatusAccepted)
				io.WriteString(w, body)
			}))
			defer server.Close()
			service := New()
			req := model.Request{URL: server.URL, Headers: []model.KV{{Key: "Accept", Value: "text/event-stream"}}}
			var response model.Response
			var err error
			count, status := 0, 0
			if test.interactive {
				response, err = service.SendSSE(context.Background(), req, nil, SSEObserver{
					Opened: func(code int) { status = code },
					Event:  func(SSEEvent) { count++ },
				})
			} else {
				response, err = service.Send(context.Background(), req, nil)
			}
			wantLength, wantStatus := len(body), 0
			if test.wantEvents != 0 {
				wantLength, wantStatus = maxSSERawBytes, http.StatusAccepted
			}
			if err != nil || count != test.wantEvents || status != wantStatus || response.StatusCode != http.StatusAccepted || response.URL != server.URL ||
				response.Size != int64(len(body)) || response.Truncated != (wantLength < len(body)) || string(response.Body) != body[:wantLength] {
				t.Fatalf("events = %d, opened = %d, HTTP = %d, bytes = %d/%d, truncated = %t, error = %v", count, status, response.StatusCode, len(response.Body), response.Size, response.Truncated, err)
			}
		})
	}
}
