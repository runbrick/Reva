package request

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/runbrick/reva/internal/model"
)

func cookieSend(t *testing.T, svc *Service, ctx context.Context, url string, headers []model.KV, auth model.Auth, noRedirect bool) model.Response {
	t.Helper()
	req := model.NewRequest("cookie")
	req.URL, req.Headers, req.Auth = url, headers, auth
	if len(url) >= 6 && url[:6] == "https:" {
		req.Options.VerifyTLS = model.ToggleOff
	}
	if noRedirect {
		req.Options.FollowRedirects = model.ToggleOff
	}
	resp, err := svc.Send(ctx, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertCookieBody(t *testing.T, response model.Response, want string) {
	t.Helper()
	if got := string(response.Body); got != want {
		t.Fatalf("cookie body = %q, want %q", got, want)
	}
}

func assertHeaderSequence(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cookie headers = %#v, want %#v", got, want)
	}
}

func TestCookieSessionCarryAndExpiry(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "session-1", Path: "/api", Secure: true})
		case "/logout":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "deleted", Path: "/api", Secure: true, MaxAge: -1})
		default:
			_, _ = w.Write([]byte(r.Header.Get("Cookie")))
		}
	}))
	defer server.Close()
	svc := New()
	ctx := WithCookieSession(context.Background(), NewCookieSession())
	send := func(ctx context.Context, path string, noRedirect bool) model.Response {
		return cookieSend(t, svc, ctx, server.URL+path, nil, model.Auth{}, noRedirect)
	}
	send(ctx, "/login", false)
	assertCookieBody(t, send(ctx, "/api/me", false), "sid=session-1")
	assertCookieBody(t, send(ctx, "/other", false), "")
	assertCookieBody(t, send(context.Background(), "/api/me", false), "")
	assertCookieBody(t, send(WithCookieSession(context.Background(), NewCookieSession()), "/api/me", false), "")
	assertCookieBody(t, send(ctx, "/api/me", true), "sid=session-1")
	send(ctx, "/logout", false)
	assertCookieBody(t, send(ctx, "/api/me", false), "")
	if WithCookieSession(ctx, nil) != ctx {
		t.Fatal("nil session changed context")
	}
}

func TestCookieSessionRedirectIsolation(t *testing.T) {
	var a, b *httptest.Server
	var hops []string
	b = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/seed" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "from-b", Path: "/"})
			return
		}
		hops = append(hops, r.Header.Get("Cookie"))
		if r.URL.Path == "/back" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "from-cross", Path: "/"})
			http.Redirect(w, r, a.URL+"/end", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(r.Header.Get("Cookie")))
	}))
	defer b.Close()
	a = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops = append(hops, r.Header.Get("Cookie"))
		switch r.URL.Path {
		case "/same":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "from-a", Path: "/"})
			http.Redirect(w, r, a.URL+"/end", http.StatusFound)
		case "/cross":
			http.Redirect(w, r, b.URL+"/back", http.StatusFound)
		default:
			_, _ = w.Write([]byte(r.Header.Get("Cookie")))
		}
	}))
	defer a.Close()
	svc := New()
	ctx := WithCookieSession(context.Background(), NewCookieSession())
	sendA := func(path string, headers []model.KV, noRedirect bool) model.Response {
		return cookieSend(t, svc, ctx, a.URL+path, headers, model.Auth{}, noRedirect)
	}
	cookieSend(t, svc, ctx, b.URL+"/seed", nil, model.Auth{}, false)
	sendA("/same", nil, false)
	assertHeaderSequence(t, hops, []string{"", "sid=from-a"})
	hops = nil
	if resp := sendA("/same", nil, true); resp.StatusCode != http.StatusFound || len(resp.Cookies) != 1 {
		t.Fatalf("stopped redirect response = %#v", resp)
	}
	hops = nil
	sendA("/cross", nil, false)
	assertHeaderSequence(t, hops, []string{"sid=from-a", "", ""})
	hops = nil
	sendA("/cross", []model.KV{{Key: "Cookie", Value: "sid=manual"}}, false)
	assertHeaderSequence(t, hops, []string{"sid=manual", "", ""})
	assertCookieBody(t, cookieSend(t, svc, ctx, b.URL+"/probe", nil, model.Auth{}, false), "sid=from-b")

	var plain *httptest.Server
	plain = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops = append(hops, r.Header.Get("Cookie"))
	}))
	defer plain.Close()
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/seed" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "from-tls", Path: "/", Secure: true})
			return
		}
		hops = append(hops, r.Header.Get("Cookie"))
		http.Redirect(w, r, plain.URL, http.StatusFound)
	}))
	defer tls.Close()
	cookieSend(t, svc, ctx, tls.URL+"/seed", nil, model.Auth{}, false)
	hops = nil
	cookieSend(t, svc, ctx, tls.URL, nil, model.Auth{}, false)
	assertHeaderSequence(t, hops, []string{"sid=from-tls", ""})
}

func TestCookieSessionExplicitHeadersBypass(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/seed" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "jar", Path: "/"})
			return
		}
		seen = r.Header.Values("Cookie")
		if r.URL.Path == "/change" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "ignored", Path: "/"})
		}
		_, _ = w.Write([]byte(r.Header.Get("Cookie")))
	}))
	defer server.Close()
	svc := New()
	ctx := WithCookieSession(context.Background(), NewCookieSession())
	cookieSend(t, svc, ctx, server.URL+"/seed", nil, model.Auth{}, false)
	checks := []struct {
		name    string
		headers []model.KV
		auth    model.Auth
		want    []string
	}{
		{"manual", []model.KV{{Key: "cOoKiE", Value: "sid=manual"}}, model.Auth{}, []string{"sid=manual"}},
		{"empty", []model.KV{{Key: "Cookie", Value: ""}}, model.Auth{}, []string{""}},
		{"api key", nil, model.Auth{Type: model.AuthAPIKey, In: model.APIKeyInHeader, Key: "Cookie", Value: "sid=api"}, []string{"sid=api"}},
		{"host", []model.KV{{Key: "Host", Value: "custom.local"}}, model.Auth{}, nil},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			cookieSend(t, svc, ctx, server.URL+"/change", check.headers, check.auth, false)
			assertHeaderSequence(t, seen, check.want)
			assertCookieBody(t, cookieSend(t, svc, ctx, server.URL+"/probe", nil, model.Auth{}, false), "sid=jar")
		})
	}
	cookieSend(t, svc, ctx, server.URL+"/probe", []model.KV{{Key: "Cookie", Value: "off", Disabled: true}}, model.Auth{}, false)
	assertHeaderSequence(t, seen, []string{"sid=jar"})
}
