package request_test

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/request"
)

func uploadFile(t *testing.T, name, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSendBodyModesAndContentTypes(t *testing.T) {
	path := uploadFile(t, "payload.bin", "\x00binary\xff")
	tests := []struct {
		name        string
		body        model.Body
		header      string
		wantBody    string
		wantContent string
	}{
		{"none", model.Body{Mode: model.BodyNone, Text: "ignored"}, "", "", ""},
		{"empty mode", model.Body{}, "", "", ""},
		{"json", model.Body{Mode: model.BodyJSON, Text: `{"ok":true}`}, "", `{"ok":true}`, "application/json"},
		{"json override", model.Body{Mode: model.BodyJSON, Text: "{}"}, "application/custom", "{}", "application/custom"},
		{"raw default", model.Body{Mode: model.BodyRaw, Text: "hello"}, "", "hello", "text/plain"},
		{"raw content type", model.Body{Mode: model.BodyRaw, Text: "<a/>", ContentType: "application/xml"}, "", "<a/>", "application/xml"},
		{"raw override", model.Body{Mode: model.BodyRaw, Text: "hello", ContentType: "text/plain"}, "text/custom", "hello", "text/custom"},
		{"urlencoded", model.Body{Mode: model.BodyURLEncoded, URLEncoded: []model.KV{
			{Key: "x", Value: "one & two"}, {Key: "x", Value: "three"}, {Key: "off", Value: "ignored", Disabled: true},
		}}, "", "x=one+%26+two&x=three", "application/x-www-form-urlencoded"},
		{"urlencoded override", model.Body{Mode: model.BodyURLEncoded}, "application/custom", "", "application/custom"},
		{"binary", model.Body{Mode: model.BodyBinary, FilePath: path}, "", "\x00binary\xff", "application/octet-stream"},
		{"binary override", model.Body{Mode: model.BodyBinary, FilePath: path}, "image/png", "\x00binary\xff", "image/png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if string(body) != tt.wantBody || r.Header.Get("Content-Type") != tt.wantContent {
					t.Errorf("body = %q, content type = %q; want %q, %q", body, r.Header.Get("Content-Type"), tt.wantBody, tt.wantContent)
				}
				if r.Method != "POST" {
					t.Errorf("method = %q", r.Method)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			req := model.Request{Method: model.MethodPost, URL: server.URL, Body: tt.body}
			if tt.header != "" {
				req.Headers = []model.KV{{Key: "content-type", Value: tt.header}}
			}
			if _, err := request.New().Send(context.Background(), req, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSendMultipart(t *testing.T) {
	first := uploadFile(t, "first.txt", "first file contents")
	second := uploadFile(t, "second.bin", "\x00second\xff")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("content type = %q, error = %v", mediaType, err)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		var got []string
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Error(err)
				return
			}
			body, err := io.ReadAll(part)
			if err != nil {
				t.Error(err)
			}
			got = append(got, part.FormName()+"|"+part.FileName()+"|"+string(body))
		}
		want := []string{"before||a", "upload|first.txt|first file contents", "between||b", "upload|second.bin|\x00second\xff", "after||c"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parts = %q, want %q", got, want)
		}
	}))
	defer server.Close()
	req := model.Request{Method: model.MethodPost, URL: server.URL, Body: model.Body{Mode: model.BodyFormData, Form: []model.FormField{
		{Key: "before", Value: "a"}, {Key: "upload", Value: first, File: true},
		{Key: "between", Value: "b"}, {Key: "upload", Value: second, File: true}, {Key: "after", Value: "c"},
		{Key: "off", Value: "ignored", Disabled: true}, {Key: "offfile", Value: "missing", File: true, Disabled: true},
	}}}
	if _, err := request.New().Send(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSendParamsHeadersAndScheme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := url.Values{"x": {"existing", "added"}, "keep": {"yes"}, "space": {"a & b"}}
		if !reflect.DeepEqual(r.URL.Query(), want) {
			t.Errorf("query = %v, want %v", r.URL.Query(), want)
		}
		if got := r.Header.Values("X-Multi"); !slices.Equal(got, []string{"one", "two"}) {
			t.Errorf("multi header = %v", got)
		}
		if r.Header.Get("X-Off") != "" || r.Header.Get("User-Agent") != "Reva/0.1" {
			t.Errorf("headers = %v", r.Header)
		}
		if r.Host != "custom.local" {
			t.Errorf("host = %q", r.Host)
		}
	}))
	defer server.Close()
	req := model.Request{URL: strings.TrimPrefix(server.URL, "http://") + "/?x=existing&keep=yes", Params: []model.KV{
		{Key: "x", Value: "added"}, {Key: "space", Value: "a & b"}, {Key: "off", Value: "no", Disabled: true},
	}, Headers: []model.KV{
		{Key: "X-Multi", Value: "one"}, {Key: "x-multi", Value: "two"}, {Key: "X-Off", Value: "no", Disabled: true},
		{Key: "User-Agent", Value: "no", Disabled: true}, {Key: "host", Value: "custom.local"},
	}}
	if _, err := request.New().Send(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSendParamsMultisetMerge(t *testing.T) {
	tests := []struct {
		name  string
		query string
		rows  []model.KV
		want  url.Values
	}{
		{
			name: "synced URL and params", query: "limit=10",
			rows: []model.KV{{Key: "limit", Value: "10"}},
			want: url.Values{"limit": {"10"}},
		},
		{
			name: "params without URL query",
			rows: []model.KV{{Key: "limit", Value: "10"}, {Key: "page", Value: "2"}},
			want: url.Values{"limit": {"10"}, "page": {"2"}},
		},
		{
			name: "partial overlap", query: "limit=10&keep=yes",
			rows: []model.KV{{Key: "limit", Value: "10"}, {Key: "page", Value: "2"}},
			want: url.Values{"limit": {"10"}, "keep": {"yes"}, "page": {"2"}},
		},
		{
			name: "repeated key with distinct values", query: "a=1&a=2",
			rows: []model.KV{{Key: "a", Value: "1"}, {Key: "a", Value: "2"}, {Key: "a", Value: "3"}},
			want: url.Values{"a": {"1", "2", "3"}},
		},
		{
			name: "rows exceed identical URL occurrences", query: "a=1&a=1",
			rows: []model.KV{{Key: "a", Value: "1"}, {Key: "a", Value: "1"}, {Key: "a", Value: "1"}},
			want: url.Values{"a": {"1", "1", "1"}},
		},
		{
			name: "URL exceeds identical row occurrences", query: "a=1&a=1&a=1",
			rows: []model.KV{{Key: "a", Value: "1"}, {Key: "a", Value: "1"}},
			want: url.Values{"a": {"1", "1", "1"}},
		},
		{
			name: "identical params without URL query",
			rows: []model.KV{{Key: "a", Value: "1"}, {Key: "a", Value: "1"}},
			want: url.Values{"a": {"1", "1"}},
		},
		{
			name: "disabled rows neither add nor consume", query: "a=1",
			rows: []model.KV{{Key: "a", Value: "1", Disabled: true}, {Key: "a", Value: "1"}, {Key: "off", Value: "no", Disabled: true}},
			want: url.Values{"a": {"1"}},
		},
		{
			name: "comparison after expansion and URL decoding", query: "{{key}}={{value}}&space=a%20b",
			rows: []model.KV{{Key: "{{key}}", Value: "{{value}}"}, {Key: "space", Value: "a b"}},
			want: url.Values{"expanded": {"10"}, "space": {"a b"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query(); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("query = %v, want %v", got, tt.want)
				}
			}))
			defer server.Close()
			req := model.Request{URL: server.URL, Params: tt.rows}
			if tt.query != "" {
				req.URL += "?" + tt.query
			}
			variables := []model.KV{{Key: "key", Value: "expanded"}, {Key: "value", Value: "10"}}
			if _, err := request.New().Send(context.Background(), req, variables); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSendAuthAndHeaderOverrides(t *testing.T) {
	tests := []struct {
		name   string
		auth   model.Auth
		header model.KV
		want   string
	}{
		{"none", model.Auth{}, model.KV{}, ""},
		{"bearer", model.Auth{Type: model.AuthBearer, Token: "secret"}, model.KV{}, "Bearer secret"},
		{"basic", model.Auth{Type: model.AuthBasic, Username: "user", Password: "p:a:ss"}, model.KV{}, "Basic dXNlcjpwOmE6c3M="},
		{"api default", model.Auth{Type: model.AuthAPIKey, Key: "X-Key", Value: "secret"}, model.KV{}, "secret"},
		{"api header", model.Auth{Type: model.AuthAPIKey, In: model.APIKeyInHeader, Key: "X-Key", Value: "secret"}, model.KV{}, "secret"},
		{"api query", model.Auth{Type: model.AuthAPIKey, In: model.APIKeyInQuery, Key: "X-Key", Value: "a & b"}, model.KV{}, "a & b"},
		{"bearer override", model.Auth{Type: model.AuthBearer, Token: "secret"}, model.KV{Key: "authorization", Value: "custom"}, "custom"},
		{"basic override", model.Auth{Type: model.AuthBasic, Username: "u", Password: "p"}, model.KV{Key: "authorization", Value: "custom"}, "custom"},
		{"api override", model.Auth{Type: model.AuthAPIKey, Key: "X-Key", Value: "secret"}, model.KV{Key: "x-key", Value: "custom"}, "custom"},
		{"empty explicit header", model.Auth{Type: model.AuthBearer, Token: "secret"}, model.KV{Key: "authorization", Value: ""}, ""},
		{"disabled override", model.Auth{Type: model.AuthBearer, Token: "secret"}, model.KV{Key: "authorization", Value: "custom", Disabled: true}, "Bearer secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got := r.Header.Get("Authorization")
				if tt.auth.Type == model.AuthAPIKey {
					got = r.Header.Get("X-Key")
					if tt.auth.In == model.APIKeyInQuery {
						got = r.URL.Query().Get("X-Key")
						if r.Header.Get("X-Key") != "" {
							t.Error("query API key was also sent as a header")
						}
					}
				}
				if got != tt.want {
					t.Errorf("auth = %q, want %q", got, tt.want)
				}
				if r.Header.Get("User-Agent") != "custom-agent" {
					t.Errorf("user agent = %q", r.Header.Get("User-Agent"))
				}
			}))
			defer server.Close()
			req := model.Request{URL: server.URL, Auth: tt.auth, Headers: []model.KV{{Key: "user-agent", Value: "custom-agent"}}}
			if tt.header.Key != "" {
				req.Headers = append(req.Headers, tt.header)
			}
			if _, err := request.New().Send(context.Background(), req, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSendAuthHeadersBeforeDefaults(t *testing.T) {
	for _, key := range []string{"Content-Type", "User-Agent"} {
		t.Run(key, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get(key); got != "auth-value" {
					t.Errorf("%s = %q, want auth-value", key, got)
				}
			}))
			defer server.Close()
			req := model.Request{URL: server.URL, Body: model.Body{Mode: model.BodyRaw, Text: "body"},
				Auth: model.Auth{Type: model.AuthAPIKey, Key: key, Value: "auth-value"}}
			if _, err := request.New().Send(context.Background(), req, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSendVariableSubstitution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/expanded" || r.URL.Query().Get("expanded") != "expanded" || r.Header.Get("X-expanded") != "expanded" ||
			r.Header.Get("Authorization") != "Bearer expanded" || string(body) != "expanded" || r.Header.Get("Content-Type") != "text/expanded" {
			t.Errorf("unexpanded request: URL=%s headers=%v body=%q", r.URL, r.Header, body)
		}
	}))
	defer server.Close()
	req := model.Request{Method: model.MethodPost, URL: "{{host}}/{{value}}",
		Params: []model.KV{{Key: "{{value}}", Value: "{{value}}"}}, Headers: []model.KV{{Key: "X-{{value}}", Value: "{{value}}"}},
		Body: model.Body{Mode: model.BodyRaw, Text: "{{value}}", ContentType: "text/{{value}}"}, Auth: model.Auth{Type: model.AuthBearer, Token: "{{value}}"}}
	original := req.Clone()
	_, err := request.New().Send(context.Background(), req, []model.KV{{Key: "host", Value: server.URL}, {Key: "value", Value: "expanded"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(req, original) {
		t.Fatal("Send mutated the original request")
	}
}

func TestSendResponseMetadataAndTruncation(t *testing.T) {
	for _, size := range []int{0, 4, 5, 100000} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("X-Multi", "one")
				w.Header().Add("X-Multi", "two")
				w.Header().Set("A-First", "first")
				w.WriteHeader(http.StatusTeapot)
				io.WriteString(w, strings.Repeat("x", size))
			}))
			defer server.Close()
			svc := request.New()
			svc.MaxBodyBytes = 4
			resp, err := svc.Send(context.Background(), model.Request{URL: server.URL}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if string(resp.Body) != strings.Repeat("x", min(size, 4)) || resp.Size != int64(size) || resp.Truncated != (size > 4) {
				t.Errorf("body=%q size=%d truncated=%v", resp.Body, resp.Size, resp.Truncated)
			}
			if resp.StatusCode != 418 || resp.Status != "418 I'm a teapot" || resp.Proto != "HTTP/1.1" || resp.Duration <= 0 || resp.URL != server.URL {
				t.Errorf("response metadata: %+v", resp)
			}
			var keys, multi []string
			for _, header := range resp.Headers {
				keys = append(keys, header.Key)
				if header.Key == "X-Multi" {
					multi = append(multi, header.Value)
				}
			}
			if !slices.IsSorted(keys) || !slices.Equal(multi, []string{"one", "two"}) {
				t.Errorf("headers = %v", resp.Headers)
			}
		})
	}
}

func TestSendDefaultCapAndGzip(t *testing.T) {
	const size = request.DefaultMaxBodyBytes + 17
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			t.Error("transport did not request gzip")
		}
		w.Header().Set("Content-Encoding", "gzip")
		writer := gzip.NewWriter(w)
		io.WriteString(writer, strings.Repeat("z", size))
		writer.Close()
	}))
	defer server.Close()
	resp, err := new(request.Service).Send(context.Background(), model.Request{URL: server.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Body) != request.DefaultMaxBodyBytes || resp.Size != size || !resp.Truncated || resp.Body[0] != 'z' {
		t.Errorf("body length=%d size=%d truncated=%v", len(resp.Body), resp.Size, resp.Truncated)
	}
}

func TestSendCancellationAndTimeout(t *testing.T) {
	for _, stage := range []string{"headers", "body", "discard"} {
		for _, cancelMode := range []string{"cancel", "timeout"} {
			t.Run(stage+"/"+cancelMode, func(t *testing.T) {
				started := make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if stage != "headers" {
						w.WriteHeader(http.StatusOK)
						if stage == "discard" {
							io.WriteString(w, "more than the cap")
						}
						w.(http.Flusher).Flush()
					}
					close(started)
					<-r.Context().Done()
				}))
				defer server.Close()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				svc := request.New()
				svc.MaxBodyBytes = 4
				if cancelMode == "timeout" {
					svc.Timeout = 100 * time.Millisecond
				}
				done := make(chan error, 1)
				go func() {
					_, err := svc.Send(ctx, model.Request{URL: server.URL}, nil)
					done <- err
				}()
				select {
				case <-started:
				case <-time.After(2 * time.Second):
					t.Fatal("request did not reach server")
				}
				if cancelMode == "cancel" {
					cancel()
				}
				select {
				case err := <-done:
					want := context.Canceled
					if cancelMode == "timeout" {
						want = context.DeadlineExceeded
					}
					if !errors.Is(err, want) {
						t.Errorf("error = %v, want %v", err, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("Send did not return promptly")
				}
			})
		}
	}
}

func TestSendRedirectFinalURLAndLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var hops int
		fmt.Sscanf(r.URL.Path, "/%d", &hops)
		if hops > 0 {
			http.Redirect(w, r, fmt.Sprintf("/%d?final=yes", hops-1), http.StatusFound)
			return
		}
		io.WriteString(w, "arrived")
	}))
	defer server.Close()
	resp, err := request.New().Send(context.Background(), model.Request{URL: server.URL + "/10"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.URL != server.URL+"/0?final=yes" || string(resp.Body) != "arrived" {
		t.Errorf("response = %+v", resp)
	}
	_, err = request.New().Send(context.Background(), model.Request{URL: server.URL + "/11"}, nil)
	if err == nil || !strings.Contains(err.Error(), "10 redirects") {
		t.Errorf("redirect limit error = %v", err)
	}
}

func TestSendUploadRedirectReplay(t *testing.T) {
	path := uploadFile(t, "upload.txt", "replayed")
	for _, mode := range []model.BodyMode{model.BodyJSON, model.BodyBinary, model.BodyFormData} {
		t.Run(string(mode), func(t *testing.T) {
			var firstBody, firstContent string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if r.URL.Path == "/" {
					firstBody, firstContent = string(body), r.Header.Get("Content-Type")
					http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
					return
				}
				if string(body) != firstBody || r.Header.Get("Content-Type") != firstContent || r.Method != "POST" {
					t.Errorf("redirect changed upload: body=%q headers=%v method=%s", body, r.Header, r.Method)
				}
			}))
			defer server.Close()
			body := model.Body{Mode: mode, Text: "replayed", FilePath: path, Form: []model.FormField{{Key: "file", Value: path, File: true}}}
			resp, err := request.New().Send(context.Background(), model.Request{Method: model.MethodPost, URL: server.URL, Body: body}, nil)
			if err != nil || resp.URL != server.URL+"/final" {
				t.Fatalf("URL = %q, error = %v", resp.URL, err)
			}
		})
	}
}

func TestSendMissingFileError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-file.bin")
	for _, body := range []model.Body{
		{Mode: model.BodyBinary, FilePath: missing},
		{Mode: model.BodyFormData, Form: []model.FormField{{Key: "file", Value: missing, File: true}}},
	} {
		_, err := request.New().Send(context.Background(), model.Request{URL: "http://127.0.0.1", Body: body}, nil)
		if err == nil || !strings.Contains(err.Error(), missing) || !errors.Is(err, os.ErrNotExist) {
			t.Errorf("mode=%s error=%v, want missing path %q", body.Mode, err, missing)
		}
	}
}

func TestSendInvalidURL(t *testing.T) {
	for _, raw := range []string{"", "://bad", "http://", "http://[invalid", "http://host:bad", "ftp://127.0.0.1/file", "http://127.0.0.1/%zz", "http://127.0.0.1/?bad=%zz", "http://127.0.0.1/\n"} {
		t.Run(raw, func(t *testing.T) {
			_, err := request.New().Send(context.Background(), model.Request{URL: raw}, nil)
			if err == nil {
				t.Fatal("invalid URL accepted")
			}
		})
	}
}
