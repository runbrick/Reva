package grpcclient

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
)

const rpcPath = "/example.Echo/Call"

func testHTTP2(t *testing.T, secure bool, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	if secure {
		server.EnableHTTP2 = true
		server.StartTLS()
	} else {
		server.Config.Protocols = new(http.Protocols)
		server.Config.Protocols.SetUnencryptedHTTP2(true)
		server.Start()
	}
	t.Cleanup(server.Close)
	return server
}

func testFrame(payload ...byte) []byte {
	return append(binary.BigEndian.AppendUint32([]byte{0}, uint32(len(payload))), payload...)
}

func TestUnaryHTTP2(t *testing.T) {
	for _, test := range []struct {
		name   string
		secure bool
	}{{"h2c", false}, {"TLS", true}} {
		t.Run(test.name, func(t *testing.T) {
			server := testHTTP2(t, test.secure, func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(body, testFrame(8, 42)) || r.ProtoMajor != 2 || r.Method != "POST" || r.RequestURI != rpcPath || r.Header.Get("Content-Type") != "application/grpc" || r.Header.Get("TE") != "trailers" || r.Header.Get("Grpc-Timeout") == "" {
					t.Errorf("invalid request: %s %s %s %v %x %v", r.Proto, r.Method, r.RequestURI, r.Header, body, err)
				}
				if strings.Join(r.Header.Values("x-test"), ",") != "first,second" || r.Header.Get("x-skip") != "" || r.Header.Get("x-data-bin") != "/w" {
					t.Errorf("invalid metadata: %v", r.Header)
				}
				w.Header().Set("Content-Type", "application/grpc+proto")
				w.Header().Set("Trailer", "Grpc-Status")
				_, _ = w.Write(testFrame(8, 43))
				w.Header().Set("Grpc-Status", "0")
			})
			result, err := Call(context.Background(), Request{
				URL: server.URL + rpcPath, Hex: " 08\n2A ", InsecureTLS: test.secure,
				Metadata: []model.KV{{Key: "X-Test", Value: "first"}, {Key: "x-test", Value: "second"}, {Key: "x-skip", Value: "unused", Disabled: true}, {Key: "x-data-bin", Value: "/w=="}},
			})
			if err != nil || result.Status != 0 || result.HTTPStatus != 200 || result.Duration <= 0 || !bytes.Equal(result.Message, []byte{8, 43}) {
				t.Fatalf("Call = %+v, %v", result, err)
			}
			if test.secure {
				if _, err := Call(context.Background(), Request{URL: server.URL + rpcPath}); !errors.Is(err, ErrTransport) {
					t.Fatalf("untrusted certificate error = %v", err)
				}
			}
		})
	}
}

func TestUnaryStatusAndMalformedResponses(t *testing.T) {
	for _, test := range []struct {
		name, status string
		initial      bool
		frame        []byte
		wantStatus   int
		wantErr      error
	}{
		{"trailers only", "7", true, nil, 7, nil},
		{"error trailers", "3", false, nil, 3, nil},
		{"zero-byte message", "0", false, testFrame(), 0, nil},
		{"missing message", "0", false, nil, 0, ErrProtocol},
		{"missing status", "", false, testFrame(), 0, ErrProtocol},
		{"invalid status", "-1", false, testFrame(), 0, ErrProtocol},
		{"misplaced status", "0", true, testFrame(), 0, ErrProtocol},
		{"short header", "0", false, []byte{0}, 0, ErrProtocol},
		{"short payload", "0", false, testFrame(42)[:5], 0, ErrProtocol},
		{"multiple frames", "0", false, append(testFrame(), testFrame()...), 0, ErrProtocol},
		{"oversize", "0", false, binary.BigEndian.AppendUint32([]byte{0}, MaxMessageBytes+1), 0, ErrLimit},
		{"compression", "0", false, []byte{1, 0, 0, 0, 0}, 0, ErrCompression},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := testHTTP2(t, false, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/grpc")
				if test.initial {
					w.Header().Set("Grpc-Status", test.status)
					w.Header().Set("Grpc-Message", "No%20access%2B%E4%BD%A0")
					_, _ = w.Write(test.frame)
					return
				}
				w.Header().Set("Trailer", "Grpc-Status")
				_, _ = w.Write(test.frame)
				if test.status != "" {
					w.Header().Set("Grpc-Status", test.status)
				}
			})
			result, err := Call(context.Background(), Request{URL: server.URL + rpcPath})
			if !errors.Is(err, test.wantErr) || err == nil && result.Status != test.wantStatus {
				t.Fatalf("Call = %+v, %v; want status %d, %v", result, err, test.wantStatus, test.wantErr)
			}
			if test.initial && err == nil && result.StatusMessage != "No access+\u4f60" {
				t.Fatalf("decoded grpc-message = %q", result.StatusMessage)
			}
		})
	}
}

func TestUnaryValidationAndRedirect(t *testing.T) {
	server := testHTTP2(t, false, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			t.Error("redirect followed")
		}
		w.Header().Set("Location", "/redirected")
		w.WriteHeader(http.StatusTemporaryRedirect)
	})
	const clue = "redaction-test-value"
	for _, test := range []struct {
		request Request
		want    error
	}{
		{Request{URL: "https://fixture@localhost" + rpcPath}, ErrURL},
		{Request{URL: server.URL + "/invalid"}, ErrURL},
		{Request{URL: server.URL + rpcPath + "?key=" + clue}, ErrURL},
		{Request{URL: server.URL + rpcPath, Hex: "123"}, ErrHex},
		{Request{URL: server.URL + rpcPath, Hex: strings.Repeat("00", MaxMessageBytes+1)}, ErrLimit},
		{Request{URL: server.URL + rpcPath, Metadata: []model.KV{{Key: "Content-Type", Value: "text/plain"}}}, ErrMetadata},
		{Request{URL: server.URL + rpcPath, Metadata: []model.KV{{Key: "x-bad", Value: clue + "\r\n"}}}, ErrMetadata},
		{Request{URL: server.URL + rpcPath, Metadata: []model.KV{{Key: "x-data-bin", Value: clue + "!"}}}, ErrMetadata},
	} {
		_, err := Call(context.Background(), test.request)
		if !errors.Is(err, test.want) || strings.Contains(err.Error(), clue) {
			t.Errorf("error = %v, want %v without input content", err, test.want)
		}
	}
	result, err := Call(context.Background(), Request{URL: server.URL + rpcPath})
	if !errors.Is(err, ErrHTTPStatus) || result.HTTPStatus != 307 {
		t.Fatalf("redirect = %+v, %v", result, err)
	}
}

func TestUnaryCancellationAndDeadline(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		entered, finished := make(chan struct{}), make(chan struct{})
		server := testHTTP2(t, false, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/grpc")
			w.(http.Flusher).Flush()
			close(entered)
			<-r.Context().Done()
			close(finished)
		})
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		request := Request{URL: server.URL + rpcPath, Timeout: 4 * time.Second}
		want := context.Canceled
		if timeout {
			request.Timeout = 500 * time.Millisecond
			want = context.DeadlineExceeded
		}
		done := make(chan error, 1)
		go func() {
			_, err := Call(ctx, request)
			done <- err
		}()
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("server did not receive call")
		}
		if !timeout {
			cancel()
		}
		select {
		case err := <-done:
			if !errors.Is(err, want) {
				t.Fatalf("Call error = %v, want %v", err, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("call did not stop")
		}
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
			t.Fatal("server did not observe cancellation")
		}
	}
}
