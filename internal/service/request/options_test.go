package request

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runbrick/reqo/internal/model"
)

func TestSendRequestTimeoutIncludesBody(t *testing.T) {
	for _, stage := range []string{"handler", "body", "discard"} {
		t.Run(stage, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if stage != "handler" {
					if stage == "discard" {
						io.WriteString(w, "more than the body cap")
					}
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer server.Close()
			svc := New()
			svc.MaxBodyBytes = 4
			start := time.Now()
			_, err := svc.Send(context.Background(), model.Request{URL: server.URL, Options: model.RequestOptions{TimeoutMs: 100}}, nil)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want deadline exceeded", err)
			}
			if elapsed := time.Since(start); elapsed > 2*time.Second {
				t.Fatalf("request timeout took %v", elapsed)
			}
		})
	}
}

func TestSendFollowRedirectsOff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Error("followed disabled redirect")
		}
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer server.Close()
	resp, err := New().Send(context.Background(), model.Request{URL: server.URL, Options: model.RequestOptions{FollowRedirects: model.ToggleOff}}, nil)
	if err != nil || resp.StatusCode != http.StatusFound || resp.Header("Location") != "/final" || resp.URL != server.URL {
		t.Fatalf("response = %+v, error = %v", resp, err)
	}
}

func TestSendVerifyTLSIsPerOptionSet(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "secure") }))
	defer server.Close()
	svc := New() // Deliberately do not use server.Client() or its certificate pool.
	for _, verify := range []model.Toggle{model.ToggleOn, model.ToggleOff, model.ToggleOn, model.ToggleInherit, model.ToggleOff} {
		resp, err := svc.Send(context.Background(), model.Request{URL: server.URL, Options: model.RequestOptions{VerifyTLS: verify}}, nil)
		if verify == model.ToggleOff {
			if err != nil || string(resp.Body) != "secure" {
				t.Fatalf("VerifyTLS off: response=%+v error=%v", resp, err)
			}
		} else {
			var certErr x509.UnknownAuthorityError
			if !errors.As(err, &certErr) {
				t.Fatalf("VerifyTLS %q: error=%v, want untrusted certificate", verify, err)
			}
		}
	}
	if len(svc.clients) != 2 {
		t.Fatalf("cache size = %d, want 2", len(svc.clients))
	}
}

func TestSendCookiesWithAttributes(t *testing.T) {
	expires := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "earlier", Value: "excluded"})
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc", Domain: "example.com", Path: "/api", Expires: expires,
			MaxAge: 3600, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
		http.SetCookie(w, &http.Cookie{Name: "strict", Value: "s", SameSite: http.SameSiteStrictMode})
		http.SetCookie(w, &http.Cookie{Name: "none", Value: "n", SameSite: http.SameSiteNoneMode})
		http.SetCookie(w, &http.Cookie{Name: "default", Value: "d", MaxAge: -1})
		w.Header().Add("Set-Cookie", "invalid")
	}))
	defer server.Close()
	resp, err := New().Send(context.Background(), model.Request{URL: server.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.Cookie{
		{Name: "session", Value: "abc", Domain: "example.com", Path: "/api", Expires: expires, MaxAge: 3600, HTTPOnly: true, Secure: true, SameSite: "Lax"},
		{Name: "strict", Value: "s", SameSite: "Strict"},
		{Name: "none", Value: "n", SameSite: "None"},
		{Name: "default", Value: "d", MaxAge: -1},
	}
	if !reflect.DeepEqual(resp.Cookies, want) {
		t.Fatalf("cookies = %+v, want %+v", resp.Cookies, want)
	}
}

func checkTimeline(t *testing.T, response model.Response, required ...model.PhaseKind) map[model.PhaseKind]model.Phase {
	t.Helper()
	phases := make(map[model.PhaseKind]model.Phase)
	var total, previousEnd time.Duration
	for _, phase := range response.Timeline {
		if _, duplicate := phases[phase.Kind]; duplicate {
			t.Errorf("duplicate phase %s", phase.Kind)
		}
		phases[phase.Kind] = phase
		if phase.Start < previousEnd || phase.Duration < 0 || phase.Start+phase.Duration > response.Duration+time.Millisecond {
			t.Errorf("invalid phase %+v (previous end %v, duration %v)", phase, previousEnd, response.Duration)
		}
		total += phase.Duration
		previousEnd = phase.Start + phase.Duration
	}
	if total > response.Duration+time.Millisecond {
		t.Errorf("phases sum to %v, duration %v", total, response.Duration)
	}
	for _, kind := range required {
		if _, ok := phases[kind]; !ok {
			t.Errorf("missing %s in %+v", kind, response.Timeline)
		}
	}
	return phases
}

func TestSendTimelineFreshAndReusedConnections(t *testing.T) {
	for _, tls := range []bool{false, true} {
		t.Run(fmt.Sprint("tls=", tls), func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(5 * time.Millisecond)
				io.WriteString(w, "first")
				w.(http.Flusher).Flush()
				time.Sleep(5 * time.Millisecond)
				io.WriteString(w, "last")
			})
			server := httptest.NewUnstartedServer(handler)
			if tls {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			svc := New()
			req := model.Request{URL: server.URL, Options: model.RequestOptions{VerifyTLS: model.ToggleOff}}
			for i := range 2 {
				response, err := svc.Send(context.Background(), req, nil)
				if err != nil {
					t.Fatal(err)
				}
				phases := checkTimeline(t, response, model.PhaseSend, model.PhaseWait, model.PhaseDownload)
				_, connected := phases[model.PhaseConnect]
				_, handshook := phases[model.PhaseTLS]
				if connected != (i == 0) || handshook != (tls && i == 0) {
					t.Errorf("send %d timeline = %+v", i, response.Timeline)
				}
			}
		})
	}
}

func TestSendTimelineRedirectFinalHop(t *testing.T) {
	var finalStarted time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			time.Sleep(25 * time.Millisecond)
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		finalStarted = time.Now()
		io.WriteString(w, "final")
	}))
	defer server.Close()
	start := time.Now()
	resp, err := New().Send(context.Background(), model.Request{URL: server.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	phases := checkTimeline(t, resp, model.PhaseSend, model.PhaseWait, model.PhaseDownload)
	if _, ok := phases[model.PhaseConnect]; ok {
		t.Error("final hop reused connection but includes earlier connect")
	}
	if resp.Duration < 25*time.Millisecond || phases[model.PhaseSend].Start < 25*time.Millisecond || finalStarted.Sub(start) < 25*time.Millisecond {
		t.Fatalf("redirect chain timing = %+v", resp)
	}
}

func TestSendOptionSetsReuseSeparateTransports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, r.RemoteAddr) }))
	defer server.Close()
	svc := New()
	options := []model.RequestOptions{{}, {FollowRedirects: model.ToggleOff}}
	addresses := make([]string, 2)
	for iteration := range 3 {
		for i, option := range options {
			resp, err := svc.Send(context.Background(), model.Request{URL: server.URL, Options: option}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if iteration == 0 {
				addresses[i] = string(resp.Body)
			} else if string(resp.Body) != addresses[i] {
				t.Errorf("option set %d opened another connection", i)
			}
		}
	}
	if addresses[0] == addresses[1] || len(svc.clients) != 2 {
		t.Fatalf("connections=%v cache size=%d", addresses, len(svc.clients))
	}
	// Exercise shared cache initialization and use from concurrent sends.
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			_, err := svc.Send(context.Background(), model.Request{URL: server.URL, Options: model.RequestOptions{TimeoutMs: 2000 + i%2}}, nil)
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if len(svc.clients) != 4 {
		t.Fatalf("cache size after concurrent sends=%d, want 4", len(svc.clients))
	}
}

func TestSendDescriptionNeverReachesWire(t *testing.T) {
	const description = "private-description-sentinel"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		wire := fmt.Sprint(r.URL, r.Header, string(body))
		if strings.Contains(wire, description) {
			t.Errorf("description reached wire: %s", wire)
		}
		if r.URL.Query().Get("q") != "query-value" || r.Header.Get("X-Test") != "header-value" || string(body) != "field=form-value" {
			t.Errorf("missing request data: %s", wire)
		}
	}))
	defer server.Close()
	req := model.Request{URL: server.URL, Method: model.MethodPost,
		Params:  []model.KV{{Key: "q", Value: "query-value", Description: description}},
		Headers: []model.KV{{Key: "X-Test", Value: "header-value", Description: description}},
		Body:    model.Body{Mode: model.BodyURLEncoded, URLEncoded: []model.KV{{Key: "field", Value: "form-value", Description: description}}}}
	if _, err := New().Send(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
}
