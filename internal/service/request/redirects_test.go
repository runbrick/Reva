package request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
)

func TestRedirectDiagnosticsObserveMethodAndAuthentication(t *testing.T) {
	var crossCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		crossCalls.Add(1)
		if req.Method != http.MethodGet || req.URL.Path != "/secret-cross-path" || req.URL.RawQuery != "state=secret-cross-query" {
			t.Error("unexpected request on the cross-origin hop")
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("X-API-Key") != "" {
			t.Error("cross-origin request carried authentication")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()
	var sameCalls atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer secret-bearer" || req.Header.Get("X-API-Key") != "secret-key" {
			t.Error("same-origin request lost authentication")
		}
		switch req.URL.Path {
		case "/start":
			if req.Method != http.MethodPost {
				t.Error("initial request was not POST")
			}
			w.Header().Set("Location", "/secret-same-path?secret-query-key=secret-query-value#secret-fragment")
			w.WriteHeader(http.StatusFound)
		case "/secret-same-path":
			sameCalls.Add(1)
			if req.Method != http.MethodGet {
				t.Error("302 did not rewrite POST to GET")
			}
			w.Header().Set("Location", strings.Replace(destination.URL, "://", "://secret-user:secret-password@", 1)+"/secret-cross-path?state=secret-cross-query#secret-cross-fragment")
			w.WriteHeader(http.StatusTemporaryRedirect)
		default:
			t.Error("unexpected origin path")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer origin.Close()
	req := model.Request{
		Method: model.MethodPost, URL: origin.URL + "/start",
		Auth:    model.Auth{Type: model.AuthAPIKey, Key: "X-API-Key", Value: "secret-key"},
		Headers: []model.KV{{Key: "Authorization", Value: "Bearer secret-bearer"}},
	}
	resp, err := New().Send(context.Background(), req, nil)
	if err != nil || resp.StatusCode != http.StatusNoContent || sameCalls.Load() != 1 || crossCalls.Load() != 1 {
		t.Fatalf("redirect chain failed: %d %v", resp.StatusCode, err)
	}
	if len(resp.Redirects) != 2 || len(resp.Timeline) == 0 {
		t.Fatalf("redirects or final timeline missing: %+v %+v", resp.Redirects, resp.Timeline)
	}
	want := []model.RedirectHop{
		{Method: model.MethodPost, NextMethod: model.MethodGet, StatusCode: 302, Target: origin.URL, Followed: true, Reason: model.RedirectFollowed},
		{Method: model.MethodGet, NextMethod: model.MethodGet, StatusCode: 307, Target: destination.URL, Followed: true, Reason: model.RedirectFollowed, AuthRemoved: true},
	}
	for index, hop := range resp.Redirects {
		if hop.Duration <= 0 || hop.Duration > resp.Duration {
			t.Errorf("hop %d has invalid duration %v", index, hop.Duration)
		}
		hop.Duration = 0
		if hop != want[index] {
			t.Errorf("hop %d = %+v, want %+v", index, hop, want[index])
		}
	}
	serialized, err := json.Marshal(model.Response{Redirects: resp.Redirects})
	if err != nil || strings.Contains(string(serialized), origin.URL) || strings.Contains(string(serialized), destination.URL) {
		t.Fatalf("redirect diagnostics should stay in memory: %s %v", serialized, err)
	}
}

func TestRedirectDiagnosticsStopReasons(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		switch req.URL.Path {
		case "/malformed":
			w.Header().Set("Location", "/%zz?token=secret-malformed")
		case "/missing":
		default:
			w.Header().Set("Location", "/loop?token=secret-loop")
		}
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	svc := New()
	for _, tc := range []struct {
		name, path string
		options    model.RequestOptions
		reason     model.RedirectReason
		hops       int
		nextMethod model.Method
		wantError  bool
	}{
		{"disabled", "/loop", model.RequestOptions{FollowRedirects: model.ToggleOff}, model.RedirectDisabled, 1, model.MethodGet, false},
		{"limit", "/loop", model.RequestOptions{}, model.RedirectLimit, 11, model.MethodGet, true},
		{"missing", "/missing", model.RequestOptions{}, model.RedirectMissingTarget, 1, "", false},
		{"malformed", "/malformed", model.RequestOptions{}, model.RedirectMalformedTarget, 1, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls.Store(0)
			resp, err := svc.Send(context.Background(), model.Request{URL: server.URL + tc.path, Options: tc.options}, nil)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected send error: %v", err)
			}
			if len(resp.Redirects) != tc.hops || int(calls.Load()) != tc.hops {
				t.Fatalf("redirect log and actual calls differ: got %d records and %d calls, want %d", len(resp.Redirects), calls.Load(), tc.hops)
			}
			for _, hop := range resp.Redirects[:len(resp.Redirects)-1] {
				if !hop.Followed || hop.Reason != model.RedirectFollowed {
					t.Errorf("successful hop marked as not followed: %+v", hop)
				}
			}
			last := resp.Redirects[len(resp.Redirects)-1]
			if last.Reason != tc.reason || last.Followed || last.AuthRemoved || last.StatusCode != 302 || last.NextMethod != tc.nextMethod {
				t.Errorf("unexpected final redirect record: %+v", last)
			}
			if strings.Contains(fmt.Sprint(resp.Redirects), "secret-") {
				t.Fatal("redirect trace retained an unredacted URL")
			}
		})
	}
}

type diagnosticTransportFunc func(*http.Request) (*http.Response, error)

func (transport diagnosticTransportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func TestRedirectDiagnosticsDistinguishAttemptFromPolicyApproval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Location", "/secret-next-path?token=secret-query")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	transportErr := errors.New("secret-transport-error")
	policyErr := errors.New("secret-policy-error")
	for _, tc := range []struct {
		name   string
		policy func(*http.Request, []*http.Request) error
		reason model.RedirectReason
		cause  error
	}{
		{"transport failure", nil, model.RedirectTransportError, transportErr},
		{"client rejection before transport", func(req *http.Request, _ []*http.Request) error {
			req.RequestURI = "/secret-invalid-uri"
			return nil
		}, model.RedirectNotSent, nil},
		{"policy refusal", func(*http.Request, []*http.Request) error { return policyErr }, model.RedirectPolicy, policyErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var attempts int
			svc := &Service{Transport: diagnosticTransportFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				if req.URL.Path == "/secret-next-path" {
					return nil, transportErr
				}
				return server.Client().Transport.RoundTrip(req)
			})}
			if tc.policy != nil {
				svc.client(model.RequestOptions{}).CheckRedirect = tc.policy
			}
			resp, err := svc.Send(context.Background(), model.Request{Method: model.MethodPost, URL: server.URL}, nil)
			if err == nil || tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("unexpected send error: %v", err)
			}
			if len(resp.Redirects) != 1 {
				t.Fatalf("unexpected chain: %+v", resp.Redirects)
			}
			if strings.Contains(fmt.Sprint(resp.Redirects), "secret-") {
				t.Fatal("redirect trace retained an unredacted transport or policy error")
			}
			if hop := resp.Redirects[0]; hop.Followed || hop.Reason != tc.reason || hop.Method != model.MethodPost || hop.NextMethod != model.MethodGet || hop.Target != server.URL {
				t.Errorf("unexpected hop: %+v", hop)
			}
			wantAttempts := 1
			if tc.reason == model.RedirectTransportError {
				wantAttempts = 2
			}
			if attempts != wantAttempts {
				t.Errorf("transport attempts = %d, want %d", attempts, wantAttempts)
			}
		})
	}
}

func TestRedirectDiagnosticsIsolateConcurrentSends(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/redirect" {
			w.Header().Set("Location", "/done")
			w.WriteHeader(http.StatusSeeOther)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	svc := New()
	for index := range 8 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			path, redirectCount := "/done", 0
			if index%2 == 0 {
				path, redirectCount = "/redirect", 1
			}
			resp, err := svc.Send(context.Background(), model.Request{URL: server.URL + path}, nil)
			if err != nil || resp.StatusCode != 204 || len(resp.Redirects) != redirectCount {
				t.Fatalf("send state mixed across requests: %d %+v %v", resp.StatusCode, resp.Redirects, err)
			}
		})
	}
}
