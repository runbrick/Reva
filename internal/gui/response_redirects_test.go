package gui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/request"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestResponseTimelineRedirectDiagnosticsAndClipboard(t *testing.T) {
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		destinationCalls.Add(1)
		if req.Method != http.MethodGet || req.Header.Get("Authorization") != "" {
			t.Error("destination did not receive a GET without authorization")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destination.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.Header.Get("Authorization") != "Bearer secret-authorization" {
			t.Error("origin did not receive an authenticated POST")
		}
		w.Header().Set("Location", strings.Replace(destination.URL, "://", "://secret-user:secret-password@", 1)+"/secret-path?key=secret-query#secret-fragment")
		w.WriteHeader(http.StatusFound)
	}))
	defer origin.Close()
	svc := fake.New()
	svc.Requests = request.New()
	h := newHarness(t, svc)
	h.App.S = i18n.For(i18n.En)
	tab := h.App.Active()
	tab.Request = model.Request{Method: model.MethodPost, URL: origin.URL, Auth: model.Auth{Type: model.AuthBearer, Token: "secret-authorization"}}
	responsePaint(h)
	send := func() {
		h.click(h.App.S.Request.Send)
		h.waitFor("request completion", func() bool { return !tab.Sending })
		if tab.Err != "" || tab.Response == nil {
			t.Fatalf("no HTTP response: %s", tab.Err)
		}
	}
	send()
	if h.shows(i18n.ResponseRedirectsFor(i18n.En).Title) {
		t.Fatal("redirect diagnostics appeared outside Timeline")
	}
	timeline := responseClickTab(h, h.App.S.Response.Timeline)
	assertDiagnostics := func(labels i18n.ResponseRedirects, method, outcome, auth string) {
		t.Helper()
		for _, value := range []string{labels.Title, method, "302", "ms", destination.URL, outcome, auth} {
			if !h.shows(value) {
				t.Errorf("timeline missing %q", value)
			}
		}
		h.click(labels.Copy)
		clipboard := h.D.Clipboard()
		for _, value := range []string{labels.Title, method, "302", "ms", destination.URL, outcome, auth} {
			if !strings.Contains(clipboard, value) {
				t.Errorf("clipboard missing %q in %q", value, clipboard)
			}
		}
		if h.shows("secret-") || strings.Contains(clipboard, "secret-") || strings.Contains(clipboard, "?key=") {
			t.Fatal("diagnostics exposed an unredacted target or credential")
		}
	}
	assertDiagnostics(i18n.ResponseRedirectsFor(i18n.En), "POST → GET", "Followed", "Auth removed")
	scroll := func(delta float32) {
		t.Helper()
		if err := h.D.Scroll(gfx.ScrollEvent{X: timeline.X + timeline.Width/2, Y: timeline.Y + timeline.Height + Space6*2, DeltaY: delta, Precise: true}); err != nil {
			t.Fatal(err)
		}
	}
	scroll(600)
	if !h.shows(h.App.S.Response.Total) || destinationCalls.Load() != 1 {
		t.Fatal("the redirect follow or final phase chart was missing")
	}
	scroll(-600)
	tab.Request.Options.FollowRedirects = model.ToggleOff
	send()
	if tab.Response.Header("Location") == "" {
		t.Fatal("fixture did not retain raw Location in the normal HTTP response")
	}
	h.App.S = i18n.For(i18n.Zh)
	responsePaint(h)
	assertDiagnostics(i18n.ResponseRedirectsFor(i18n.Zh), "POST → GET（计划）", "未跟随（已禁用重定向）", "未移除认证")
	if destinationCalls.Load() != 1 {
		t.Fatal("diagnostic UI caused a disabled redirect to be followed")
	}
}

func TestResponseTimelineWithoutRedirectsKeepsExistingView(t *testing.T) {
	h := responseHarness(t, model.Response{}, nil)
	responseClickTab(h, h.App.S.Response.Timeline)
	labels := i18n.ResponseRedirectsFor(h.App.S.Lang)
	if !h.shows(h.App.S.Response.NoTimeline) || h.shows(labels.Title) || h.shows(labels.Copy) {
		t.Fatal("timeline without redirects changed its empty state")
	}
}

func TestFailedRedirectsShowCopyableDiagnosticsAndClearOnRetry(t *testing.T) {
	labels := i18n.ResponseRedirectsFor(i18n.En)
	for _, scenario := range []struct {
		name, location, reason string
	}{
		{"limit", "/start?token=redirect-secret", labels.Limit},
		{"malformed location", "/%zz?token=redirect-secret", labels.MalformedTarget},
		{"transport error", "http://127.0.0.1:0/redirect-secret?token=redirect-secret", labels.TransportError},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
				if incoming.URL.Path == "/ok" {
					writer.WriteHeader(http.StatusNoContent)
					return
				}
				writer.Header().Set("Location", scenario.location)
				writer.WriteHeader(http.StatusFound)
			}))
			defer origin.Close()
			services := fake.New()
			services.Requests = request.New()
			harness := newHarness(t, services)
			harness.App.S = i18n.For(i18n.En)
			tab := harness.App.Active()
			tab.Request.URL = origin.URL + "/start"
			responsePaint(harness)
			harness.click(harness.App.S.Request.Send)
			harness.waitFor("failed redirect", func() bool { return tab.Err != "" && !tab.Sending })
			if tab.Response != nil || !harness.shows(labels.RequestFailed) || !harness.shows(labels.Title) {
				t.Fatal("failed send should display diagnostics without a successful response")
			}
			harness.click(labels.Copy)
			copied := harness.D.Clipboard()
			entries := services.History.List()
			if !strings.Contains(copied, scenario.reason) || strings.Contains(copied, "redirect-secret") || harness.shows("redirect-secret") {
				t.Fatalf("missing reason or sensitive target exposed in diagnostics: %q", copied)
			}
			if len(entries) != 1 || entries[0].Error != labels.RequestFailed || entries[0].StatusCode != 0 {
				t.Fatal("history should record a redacted failure without a response status")
			}
			tab.Request.URL = origin.URL + "/ok"
			responsePaint(harness)
			harness.click(harness.App.S.Request.Send)
			harness.waitFor("successful retry", func() bool { return tab.Response != nil && !tab.Sending })
			responseClickTab(harness, harness.App.S.Response.Timeline)
			if harness.shows(labels.Title) || harness.shows(labels.Copy) {
				t.Fatal("successful retry showed diagnostics from the earlier send")
			}
		})
	}
}

func BenchmarkResponseRedirectTimelineRepaint(b *testing.B) {
	application := New(fake.New())
	response := &model.Response{
		Duration: time.Millisecond,
		Timeline: []model.Phase{{Kind: model.PhaseWait, Duration: time.Millisecond}},
		Redirects: []model.RedirectHop{
			{Method: model.MethodPost, NextMethod: model.MethodGet, StatusCode: 302, Target: "https://example.test:8443", Duration: time.Millisecond, Followed: true, Reason: model.RedirectFollowed, AuthRemoved: true},
		},
	}
	view := &responseView{source: response, duration: "1 ms", phaseTimes: []string{"1.00 ms"}, phaseShares: []float32{1}}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { application.responseTimelineChart(c, view) }, Theme: &application.theme})
	config := ui.Configure(page, app.Config{Width: 900, Height: 500})
	driver, err := app.NewDriver(config, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(driver.Close)
	for range 5 {
		config.Invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		config.Invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			b.Fatal(err)
		}
	}
}
