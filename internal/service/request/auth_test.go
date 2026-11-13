package request

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reqo/internal/model"
)

func TestAuthValidationBeforeNetworkAfterExpansion(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	for _, cfg := range []model.Auth{
		{Type: model.AuthInherit}, {Type: "unsupported"},
		{Type: model.AuthBearer, Token: "{{bad}}"},
		{Type: model.AuthAPIKey, Key: "{{bad}}", Value: "value"},
		{Type: model.AuthAPIKey, Key: "X-Key", Value: "{{bad}}"},
		{Type: model.AuthAPIKey, Key: "key", In: "cookie"},
	} {
		req := model.NewRequest("r")
		req.URL, req.Auth = server.URL, cfg
		before := req.Clone()
		_, err := New().Send(context.Background(), req, []model.KV{{Key: "bad", Value: "credential-marker\r\nInjected: yes"}})
		if err == nil || strings.Contains(err.Error(), "credential-marker") || !reflect.DeepEqual(req, before) {
			t.Fatalf("invalid auth/immutability: %v, %#v", err, req)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid auth reached network")
	}
}

func TestAuthHTTPRepeatedOverridesAndQueryReplacement(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled", true: "disabled"}[disabled], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				want := []string{"", "explicit"}
				if disabled {
					want = []string{"Bearer generated"}
				}
				if !reflect.DeepEqual(r.Header.Values("Authorization"), want) {
					t.Errorf("wire auth = %#v, want %#v", r.Header.Values("Authorization"), want)
				}
			}))
			defer server.Close()
			req := model.NewRequest("r")
			req.URL, req.Auth = server.URL, model.Auth{Type: model.AuthBearer, Token: "generated"}
			req.Headers = []model.KV{{Key: "aUtHoRiZaTiOn", Disabled: disabled}, {Key: "AUTHORIZATION", Value: "explicit", Disabled: disabled}}
			if _, err := New().Send(context.Background(), req, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := url.Values{"key": {"expanded & value"}, "keep": {"a", "a", "b"}}
		if !reflect.DeepEqual(r.URL.Query(), want) || r.Header.Get("key") != "" {
			t.Errorf("query wire = %#v", r.URL.Query())
		}
	}))
	defer server.Close()
	req := model.NewRequest("r")
	req.URL = server.URL + "?key=old&key=older&keep=a&keep=a"
	req.Params = []model.KV{{Key: "keep", Value: "a"}, {Key: "keep", Value: "a"}, {Key: "keep", Value: "b"}, {Key: "key", Value: "row"}}
	req.Auth = model.Auth{Type: model.AuthAPIKey, In: model.APIKeyInQuery, Key: "key", Value: "{{value}}"}
	before := req.Clone()
	if _, err := New().Send(context.Background(), req, []model.KV{{Key: "value", Value: "expanded & value"}}); err != nil || !reflect.DeepEqual(req, before) {
		t.Fatalf("query send changed caller or failed: %v", err)
	}
}

func TestAuthRedirectCrossPortAndReturnNeverRestoresCredentials(t *testing.T) {
	for _, cfg := range []model.Auth{{Type: model.AuthBearer, Token: "token"}, {Type: model.AuthAPIKey, Key: "X-Key", Value: "key"}, {Type: model.AuthAPIKey, Key: "key", Value: "query", In: model.APIKeyInQuery}} {
		t.Run(string(cfg.Type)+"/"+string(cfg.In), func(t *testing.T) {
			var origin *httptest.Server
			var paths []string // Handler calls are sequential within this redirect chain.
			check := func(r *http.Request, initial bool) {
				paths = append(paths, r.URL.Path)
				wantAuth, wantKey := "", ""
				if initial {
					wantAuth = "Bearer explicit"
					if cfg.Type == model.AuthAPIKey && cfg.In != model.APIKeyInQuery {
						wantKey = "key"
					}
				}
				if r.Header.Get("Authorization") != wantAuth || r.Header.Get("X-Key") != wantKey {
					t.Errorf("%s auth=%q key=%q, want %q %q", r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-Key"), wantAuth, wantKey)
				}
				if r.URL.Path != "/start" && r.URL.Query().Has("key") {
					t.Errorf("query auth reapplied to redirect %s", r.URL)
				}
			}
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				check(r, false)
				u, _ := url.Parse(origin.URL + "/return")
				u.User = url.UserPassword("redirect-user", "redirect-password")
				http.Redirect(w, r, u.String(), http.StatusFound)
			}))
			defer other.Close()
			origin = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				check(r, r.URL.Path == "/start" || r.URL.Path == "/same")
				switch r.URL.Path {
				case "/start":
					http.Redirect(w, r, "/same", http.StatusFound)
				case "/same":
					u, _ := url.Parse(other.URL + "/cross")
					u.User = url.UserPassword("destination-user", "destination-password")
					http.Redirect(w, r, u.String(), http.StatusFound)
				}
			}))
			defer origin.Close()
			req := model.NewRequest("r")
			req.URL, req.Auth = origin.URL+"/start", cfg
			req.Headers = []model.KV{{Key: "Authorization", Value: "Bearer explicit"}}
			res, err := New().Send(context.Background(), req, nil)
			if err != nil || res.StatusCode != 200 || !reflect.DeepEqual(paths, []string{"/start", "/same", "/cross", "/return"}) || strings.Contains(res.URL, "@") {
				t.Fatalf("redirect chain: %#v %v %v", res, paths, err)
			}
		})
	}
}

func TestAuthRedirectSchemeChanges(t *testing.T) {
	for _, downgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "downgrade"}[downgrade], func(t *testing.T) {
			destinationHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" || r.Header.Get("X-Key") != "" {
					t.Error("scheme change leaked credentials")
				}
			})
			var destination, origin *httptest.Server
			if downgrade {
				destination = httptest.NewServer(destinationHandler)
			} else {
				destination = httptest.NewTLSServer(destinationHandler)
			}
			defer destination.Close()
			originHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) })
			if downgrade {
				origin = httptest.NewTLSServer(originHandler)
			} else {
				origin = httptest.NewServer(originHandler)
			}
			defer origin.Close()
			req := model.NewRequest("r")
			req.URL, req.Auth = origin.URL, model.Auth{Type: model.AuthAPIKey, Key: "X-Key", Value: "secret"}
			req.Headers = []model.KV{{Key: "Authorization", Value: "explicit"}}
			req.Options.VerifyTLS = model.ToggleOff
			if _, err := New().Send(context.Background(), req, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAuthRedirectPreservesCustomPolicyAndNoFollow(t *testing.T) {
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		destinationCalls.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Key") != "" {
			t.Error("custom redirect policy restored credentials")
		}
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer origin.Close()
	req := model.NewRequest("r")
	req.URL, req.Auth = origin.URL, model.Auth{Type: model.AuthAPIKey, Key: "X-Key", Value: "initial"}
	s := New()
	var callbacks int
	s.client(req.Options).CheckRedirect = func(r *http.Request, via []*http.Request) error {
		callbacks++
		r.Header.Set("Authorization", "reintroduced")
		r.Header.Set("X-Key", "reintroduced")
		return nil
	}
	if _, err := s.Send(context.Background(), req, nil); err != nil || callbacks != 1 || destinationCalls.Load() != 1 {
		t.Fatalf("callback not chained: %v, %d", err, callbacks)
	}
	stop := errors.New("custom policy stopped")
	s.client(req.Options).CheckRedirect = func(r *http.Request, via []*http.Request) error { return stop }
	if _, err := s.Send(context.Background(), req, nil); !errors.Is(err, stop) {
		t.Fatalf("custom policy error lost: %v", err)
	}
	req.Options.FollowRedirects = model.ToggleOff
	res, err := s.Send(context.Background(), req, nil)
	if err != nil || res.StatusCode != http.StatusFound || destinationCalls.Load() != 1 {
		t.Fatalf("no-follow lost: %d %v", res.StatusCode, err)
	}
}

func TestAuthOriginEffectivePort(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		same bool
	}{
		{"https://EXAMPLE.test/a", "https://example.test:443/b", true},
		{"http://example.test/a", "http://example.test:80/b", true},
		{"http://example.test", "http://example.test:81", false},
		{"https://example.test", "http://example.test:443", false},
		{"https://example.test", "https://child.example.test", false},
	} {
		a, _ := url.Parse(tc.a)
		b, _ := url.Parse(tc.b)
		if sameOrigin(a, b) != tc.same {
			t.Errorf("origin %s -> %s: want same=%t", tc.a, tc.b, tc.same)
		}
	}
}

func TestAuthRedirectCustomDestinationRewriteIsProtected(t *testing.T) {
	var calls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Key") != "" {
			t.Error("callback destination rewrite leaked auth or URL userinfo generated Basic")
		}
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/same-origin", http.StatusFound)
			return
		}
		t.Error("callback destination rewrite ignored")
	}))
	defer origin.Close()
	req := model.NewRequest("r")
	req.URL, req.Auth = origin.URL+"/start", model.Auth{Type: model.AuthAPIKey, Key: "X-Key", Value: "key"}
	req.Headers = []model.KV{{Key: "Authorization", Value: "Bearer explicit"}}
	s := New()
	s.client(req.Options).CheckRedirect = func(r *http.Request, via []*http.Request) error {
		r.URL, _ = url.Parse(destination.URL)
		r.URL.User = url.UserPassword("callback-user", "callback-password")
		return nil
	}
	if _, err := s.Send(context.Background(), req, nil); err != nil || calls.Load() != 1 {
		t.Fatalf("redirect rewrite: %v, calls %d", err, calls.Load())
	}
}
