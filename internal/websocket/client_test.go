package websocket

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDialEchoAndMaskedFrames(t *testing.T) {
	messages := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local" || r.Header.Get("Sec-WebSocket-Version") != "13" {
			t.Errorf("missing handshake headers: %v", r.Header)
		}
		key := r.Header.Get("Sec-WebSocket-Key")
		if _, err := base64.StdEncoding.DecodeString(key); err != nil {
			t.Errorf("invalid nonce: %v", err)
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		hash := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(hash[:]) + "\r\n\r\n")
		_ = rw.Flush()
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		head := make([]byte, 2)
		if _, err := io.ReadFull(rw, head); err != nil {
			t.Error(err)
			return
		}
		if head[0] != 0x81 || head[1]&0x80 == 0 {
			t.Errorf("unmasked or wrong opcode: %x", head)
			return
		}
		n := int(head[1] & 127)
		if n == 126 {
			var size [2]byte
			_, _ = io.ReadFull(rw, size[:])
			n = int(binary.BigEndian.Uint16(size[:]))
		}
		mask := make([]byte, 4)
		_, _ = io.ReadFull(rw, mask)
		data := make([]byte, n)
		_, _ = io.ReadFull(rw, data)
		for i := range data {
			data[i] ^= mask[i%4]
		}
		messages <- string(data)
		// Ping followed by fragmented text tests control frames during reads.
		_, _ = rw.Write([]byte{0x89, 1, 'x', 0x01, 2, 'o', 'k', 0x80, 1, '!'})
		_ = rw.Flush()
		// The client responds to ping with a masked pong.
		if _, err := io.ReadFull(bufio.NewReader(conn), head); err != nil {
			t.Error(err)
		} else if head[0] != 0x8a || head[1]&0x80 == 0 {
			t.Errorf("missing masked pong: %x", head)
		}
	}))
	defer server.Close()
	client, err := Dial(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http")+"/echo", http.Header{"Authorization": {"Bearer local"}}, time.Second, false)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	text := strings.Repeat("a", 130)
	if err := client.WriteText(text); err != nil {
		t.Fatal(err)
	}
	if got := <-messages; got != text {
		t.Fatalf("received %q, want %q", got, text)
	}
	if opcode, data, err := client.ReadMessage(); err != nil || opcode != 1 || string(data) != "ok!" {
		t.Fatalf("fragmented message = %d %q %v", opcode, data, err)
	}
}

func TestDialRejectsInvalidHandshakeAndURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	for _, raw := range []string{"https://example.test", "ws://", "ws" + strings.TrimPrefix(server.URL, "http"), "ws" + strings.TrimPrefix(server.URL, "http") + "/#fragment"} {
		if conn, err := Dial(context.Background(), raw, nil, time.Second, false); err == nil {
			conn.Close()
			t.Errorf("accepted %q", raw)
		}
	}
}
