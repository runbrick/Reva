package gui

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
)

func TestGRPCCreationAndPersistence(t *testing.T) {
	for _, lang := range i18n.Langs {
		t.Run(string(lang), func(t *testing.T) {
			h := newHarness(t)
			settings := h.Svc.Settings.Get()
			settings.Language = string(lang)
			if lang == i18n.En {
				settings.Theme = "dark"
			}
			h.App.ApplySettings(settings)
			col, err := h.Svc.Collections.Create("Services")
			if err != nil {
				t.Fatal(err)
			}
			_, err = h.Svc.Collections.AddFolder(col.ID, "", "Echo")
			if err != nil {
				t.Fatal(err)
			}
			collectionsPaint(h)
			h.click("Echo")
			h.click(h.App.S.Collections.Add)
			labels := h.App.S.GRPC
			collectionsOption(h, labels.NewRequest)
			h.pump()
			tab := h.App.Active()
			if tab.Request.Method != model.MethodGRPC || tab.CollectionID != col.ID || !h.shows(labels.Response) {
				t.Fatalf("gRPC editor did not open: %+v", tab)
			}
			const endpoint = "http://localhost:50051/example.Echo/Call"
			h.typeInto(labels.URL, endpoint)
			h.typeInto(labels.Input, "082a")
			h.click(labels.Metadata)
			h.click(h.App.S.Request.BulkEdit)
			h.typeInto(h.App.S.Request.BulkContent, "x-request-id: fixture")
			h.click(labels.Save)
			h.pump()
			stored, _ := h.Svc.Collections.Get(col.ID)
			if len(stored.Items) != 1 || len(stored.Items[0].Items) != 1 {
				t.Fatalf("not saved in selected folder: %+v", stored.Items)
			}
			request := stored.Items[0].Items[0].Request
			if request == nil || request.Method != model.MethodGRPC || request.URL != endpoint || request.Body.Text != "082a" || len(request.Headers) != 1 || request.Headers[0].Value != "fixture" || tab.Dirty {
				t.Fatalf("saved gRPC request lost edits: %+v", request)
			}
			h.App.CloseTab(h.App.requestTabIndex(tab.ID))
			collectionsPaint(h)
			h.click(labels.NewRequest)
			if h.App.Active().Request.ID != request.ID || h.App.Active().Request.Body.Text != "082a" || !h.shows(labels.Response) {
				t.Fatal("stored gRPC request did not reopen in its editor")
			}
			h.click(fmt.Sprintf(h.App.S.Collections.SettingsFor, "Echo"))
			if h.App.Active().Kind != collectionSettingsTabKind {
				t.Fatal("folder settings did not open")
			}
			found := false
			for _, hit := range h.D.Snapshot().Hits {
				if hit.Label == labels.NewRequest && hit.Bounds.X > RailWidth+h.App.sideSplit.Pos && hit.Bounds.Width <= 2*Space6 {
					bounds := hit.Bounds
					if err := h.D.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
						t.Fatal(err)
					}
					found = true
					break
				}
			}
			if !found {
				t.Fatal("folder settings had no gRPC creation action")
			}
			if tab := h.App.Active(); tab.Request.Method != model.MethodGRPC || tab.Request.ID == request.ID || tab.CollectionID != col.ID {
				t.Fatalf("folder settings did not create a separate gRPC request: kind %v, method %v, same ID %t", tab.Kind, tab.Request.Method, tab.Request.ID == request.ID)
			}
		})
	}
}

func TestGRPCWorkbenchCallsAndCancels(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/grpc")
		if strings.HasSuffix(r.URL.Path, "/Wait") {
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
			close(cancelled)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || !bytes.Equal(body, []byte{0, 0, 0, 0, 2, 8, 42}) {
			t.Errorf("unexpected request body %x (%v)", body, err)
		}
		if strings.HasSuffix(r.URL.Path, "/Success") {
			w.Header().Set("Trailer", "Grpc-Status")
			_, _ = w.Write([]byte{0, 0, 0, 0, 2, 8, 43})
			w.Header().Set("Grpc-Status", "0")
			return
		}
		w.Header().Set("Grpc-Status", "7")
		w.Header().Set("Grpc-Message", "Access%20denied")
	}))
	server.Config.Protocols = new(http.Protocols)
	server.Config.Protocols.SetUnencryptedHTTP2(true)
	server.Start()
	t.Cleanup(server.Close)
	h := newHarness(t)
	labels := h.App.S.GRPC
	collectionsPaint(h)
	h.click(h.App.S.Collections.Add)
	collectionsOption(h, labels.NewRequest)
	h.pump()
	h.typeInto(labels.URL, server.URL+"/example.Echo/Call")
	h.typeInto(labels.Input, "082a")
	h.click(labels.Invoke)
	h.waitFor("gRPC error status and message", func() bool { return h.shows("Access denied") && h.shows(fmt.Sprintf(labels.Status, 7)) })
	h.App.Active().Request.URL = server.URL + "/example.Echo/Success"
	h.pump()
	h.click(labels.Invoke)
	h.waitFor("gRPC response hex", func() bool { return h.shows(labels.Output) && h.shows(fmt.Sprintf(labels.Status, 0)) })
	state := h.App.grpc.tabs[h.App.Active().ID]
	if state.output.snapshot.Value != "082b" {
		t.Fatalf("unexpected output hex %q", state.output.snapshot.Value)
	}
	h.App.Active().Request.URL = server.URL + "/example.Echo/Wait"
	h.pump()
	h.click(labels.Invoke)
	h.waitFor("gRPC server to start", func() bool { return grpcChannelClosed(started) })
	h.click(labels.Cancel)
	h.waitFor("gRPC cancellation", func() bool { return grpcChannelClosed(cancelled) && h.shows(labels.Cancelled) })
}

func grpcChannelClosed(signal <-chan struct{}) bool {
	select {
	case <-signal:
		return true
	default:
		return false
	}
}
