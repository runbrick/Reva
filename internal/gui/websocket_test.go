package gui

import (
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
)

func TestWebSocketCreationMenuAndPersistence(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		t.Run(string(lang), func(t *testing.T) {
			h := newHarness(t)
			settings := h.Svc.Settings.Get()
			settings.Language = string(lang)
			h.App.ApplySettings(settings)
			collectionsPaint(h)
			col, err := h.Svc.Collections.Create("Streams")
			if err != nil {
				t.Fatal(err)
			}
			collectionsPaint(h)
			_, err = h.Svc.Collections.AddFolder(col.ID, "", "Events")
			if err != nil {
				t.Fatal(err)
			}
			collectionsPaint(h)
			h.pump()
			h.click("Events")
			h.click(h.App.S.Collections.Add)
			collectionsOption(h, h.App.S.Collections.NewWebSocket)
			h.pump()
			tab := h.App.Active()
			if tab.Request.Method != webSocketMethod || tab.CollectionID != col.ID || !h.shows(h.App.S.WebSocket.Disconnected) {
				t.Fatalf("new WebSocket did not open: %+v", tab)
			}
			h.typeInto(h.App.S.WebSocket.URL, "ws://localhost/events")
			h.pump()
			h.click(h.App.S.WebSocket.Save)
			h.pump()
			stored, _ := h.Svc.Collections.Get(col.ID)
			if len(stored.Items) != 1 || len(stored.Items[0].Items) != 1 {
				t.Fatalf("WebSocket not stored in selected folder: %+v", stored.Items)
			}
			r := stored.Items[0].Items[0].Request
			if r == nil || r.Method != webSocketMethod || r.URL != "ws://localhost/events" || tab.Dirty {
				t.Fatalf("saved WebSocket lost edits: %+v", r)
			}
			h.App.CloseTab(len(h.App.Tabs()) - 1)
			collectionsPaint(h)
			h.click(h.App.S.Collections.NewWebSocket)
			if h.App.Active().Request.URL != r.URL || h.App.Active().Request.Method != webSocketMethod {
				t.Fatal("stored WebSocket did not reopen")
			}
		})
	}
}

func TestWebSocketConnectReportsHandshakeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	h := newHarness(t)
	h.click(h.App.S.Collections.Add)
	collectionsOption(h, h.App.S.Collections.NewWebSocket)
	h.pump()
	h.typeInto(h.App.S.WebSocket.URL, "ws"+strings.TrimPrefix(server.URL, "http"))
	h.click(h.App.S.WebSocket.Connect)
	h.waitFor("handshake failure", func() bool { return h.shows("403") })
	if !h.shows(h.App.S.WebSocket.Disconnected) || h.App.Active().Request.Method != model.Method("WS") {
		t.Fatal("failed connection did not return to disconnected state")
	}
}

func TestWebSocketConnectSendReceiveAndDisconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		hash := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(hash[:]) + "\r\n\r\n")
		_ = rw.Flush()
		// Read the masked text frame, then echo a simple unmasked server frame.
		head := make([]byte, 2)
		if _, err := io.ReadFull(rw, head); err != nil {
			return
		}
		mask := make([]byte, 4)
		_, _ = io.ReadFull(rw, mask)
		data := make([]byte, int(head[1]&127))
		_, _ = io.ReadFull(rw, data)
		for i := range data {
			data[i] ^= mask[i%4]
		}
		_, _ = rw.Write(append([]byte{0x81, byte(len(data))}, data...))
		_ = rw.Flush()
		// Wait for disconnect without closing the socket prematurely.
		_, _ = io.Copy(io.Discard, rw)
	}))
	defer server.Close()
	h := newHarness(t)
	h.click(h.App.S.Collections.Add)
	collectionsOption(h, h.App.S.Collections.NewWebSocket)
	h.pump()
	h.typeInto(h.App.S.WebSocket.URL, "ws"+strings.TrimPrefix(server.URL, "http"))
	h.click(h.App.S.WebSocket.Connect)
	h.waitFor("connection", func() bool { return h.shows(h.App.S.WebSocket.Connected) })
	h.typeInto(h.App.S.WebSocket.Message, "hello")
	h.click(h.App.S.WebSocket.Send)
	h.waitFor("echo", func() bool {
		return len(h.App.websocket.tabs[h.App.Active().ID].log) == 2
	})
	log := h.App.websocket.tabs[h.App.Active().ID].log
	if log[0].text != "hello" || log[0].received || log[1].text != "hello" || !log[1].received {
		t.Fatalf("message directions were lost: %+v", log)
	}
	h.click(h.App.S.WebSocket.Disconnect)
	if !h.shows(h.App.S.WebSocket.Disconnected) {
		t.Fatal("disconnect did not update status")
	}
}
