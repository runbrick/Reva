package gui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// A Pump applies only pending effect wakes. Resize explicitly repaints fixture
// changes made by a test, without requiring any change to the shared harness.
func responsePaint(h *harness) {
	h.t.Helper()
	if err := h.D.Resize(1280, 800); err != nil {
		h.t.Fatal(err)
	}
}

func responseHarness(t *testing.T, response model.Response, sendErr error) *harness {
	t.Helper()
	svc := fake.New()
	svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		return response, sendErr
	}
	h := newHarness(t, svc)
	h.App.S = i18n.For(i18n.En)
	h.App.Active().Request.URL = "https://example.test/response"
	responsePaint(h)
	h.click("Send")
	h.waitFor("send completion", func() bool { return !h.App.Active().Sending })
	h.pump()
	return h
}

func TestResponseStatusDurationAndSizeAfterSend(t *testing.T) {
	h := responseHarness(t, model.Response{StatusCode: 200, Status: "200 OK", Duration: 12 * time.Millisecond, Size: 1536}, nil)
	for _, label := range []string{"200 OK", "12 ms", "1.50 KB"} {
		if !h.shows(label) {
			t.Errorf("response does not show %q", label)
		}
	}
}

func TestResponseStatusColorsInBothThemes(t *testing.T) {
	h := newHarness(t)
	for _, theme := range []string{"light", "dark"} {
		h.App.ApplySettings(model.Settings{Language: "en", Theme: theme})
		for _, tt := range []struct {
			code   int
			status string
			color  ui.Color
		}{
			{200, "200 OK", h.App.theme.Success},
			{302, "302 Found", h.App.theme.Foreground},
			{404, "404 Not Found", h.App.theme.Danger},
			{500, "500 Internal Server Error", h.App.theme.Danger},
		} {
			h.App.Active().Response = &model.Response{StatusCode: tt.code, Status: tt.status, Duration: 1234 * time.Millisecond}
			responsePaint(h)
			found := false
			for _, label := range h.D.Snapshot().Labels {
				if label.Value == tt.status {
					found = true
					if label.Color != tt.color {
						t.Errorf("%s: %s color = %v, want %v", theme, tt.status, label.Color, tt.color)
					}
				}
			}
			if !found || !h.shows("1.23 s") {
				t.Errorf("%s: missing status %q or duration", theme, tt.status)
			}
		}
	}
}

func TestResponsePrettyRawAndHeaders(t *testing.T) {
	body := `{"z":1,"a":2}`
	h := responseHarness(t, model.Response{
		StatusCode: 200, Status: "200 OK", Body: []byte(body),
		Headers: []model.KV{{Key: "Content-Type", Value: "application/json"}, {Key: "X-Trace", Value: "trace-123"}},
	}, nil)
	if !h.shows(`  "z": 1,`) || !h.shows(`  "a": 2`) {
		t.Fatal("Pretty did not show indented JSON")
	}
	h.click("Copy body")
	if got := h.D.Clipboard(); got != body {
		t.Fatalf("Copy body = %q, want full original body %q", got, body)
	}
	h.click("Raw")
	if !h.shows(body) {
		t.Fatal("Raw did not show compact response")
	}
	h.click("Headers")
	if !h.shows("X-Trace") || !h.shows("trace-123") {
		t.Fatal("Headers did not show name and value")
	}
	h.click("Body")
	if !h.shows(body) {
		t.Fatal("switching sections lost Raw selection")
	}
	h.click("Pretty")
	if !h.shows(`  "z": 1,`) {
		t.Fatal("switching back to Pretty did not show indentation")
	}
}

func TestResponseErrorAfterSend(t *testing.T) {
	h := responseHarness(t, model.Response{}, errors.New("connection refused"))
	if !h.shows(h.App.S.Response.ErrorTitle) || !h.shows("connection refused") {
		t.Fatal("error title and transport error must be visible")
	}
	for _, label := range h.D.Snapshot().Labels {
		if label.Value == "connection refused" && label.Color != h.App.theme.Danger {
			t.Fatal("transport error must use the theme danger color")
		}
	}
}

func TestResponseEmptyAndSending(t *testing.T) {
	svc := fake.New()
	gate := make(chan struct{})
	svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, _ model.Request) (model.Response, error) {
		select {
		case <-gate:
			return model.Response{}, nil
		case <-ctx.Done():
			return model.Response{}, ctx.Err()
		}
	}
	h := newHarness(t, svc)
	defer close(gate)
	h.pump()
	if !h.shows(h.App.S.Response.EmptyHint) {
		t.Fatal("missing empty hint in default language")
	}
	h.click("Send")
	if !h.shows(h.App.S.Response.Sending) {
		t.Fatal("missing sending text")
	}
	h.click(h.App.S.Common.Cancel)
}

func TestResponsePrettyCacheAcrossPaintsAndTabs(t *testing.T) {
	h := responseHarness(t, model.Response{Status: "200 OK", Body: []byte(`{"cached":true}`)}, nil)
	first := h.App.Active()
	v := h.App.response.views[first.ID]
	pretty := v.pretty
	// The shell treats installed responses as immutable. Deliberately mutate
	// this fixture to detect any formatting of the same pointer on repaint.
	first.Response.Body = []byte(`{"changed":true}`)
	responsePaint(h)
	h.click("Headers")
	h.click("Body")
	h.App.NewTab().Response = &model.Response{Body: []byte(`{"second":true}`)}
	responsePaint(h)
	h.App.active = 0
	responsePaint(h)
	if v.pretty != pretty || !h.shows(`  "cached": true`) {
		t.Fatal("same response was formatted again")
	}
	first.Response = &model.Response{Body: []byte(`{"fresh":true}`)}
	responsePaint(h)
	if !h.shows(`  "fresh": true`) {
		t.Fatal("new response did not invalidate pretty cache")
	}
}

func TestResponsePlainInvalidJSONAndSniffedJSON(t *testing.T) {
	for _, tt := range []struct {
		name, contentType, body, want string
	}{
		{"plain", "text/plain", "hello", "hello"},
		{"invalid JSON", "application/json", `{"unfinished":`, `{"unfinished":`},
		{"sniff JSON", "text/plain", `{"ok":true}`, `  "ok": true`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := responseHarness(t, model.Response{Body: []byte(tt.body), Headers: []model.KV{{Key: "Content-Type", Value: tt.contentType}}}, nil)
			if !h.shows(tt.want) {
				t.Fatalf("body did not show %q", tt.want)
			}
		})
	}
}

func TestResponseTruncationAndDisplayLimit(t *testing.T) {
	h := responseHarness(t, model.Response{Body: []byte("partial body"), Truncated: true}, nil)
	if !h.shows(h.App.S.Response.Truncated) {
		t.Fatal("missing server truncation notice")
	}
	// Use a small visible prefix and long tail, so headless rendering need not
	// shape a two-megabyte paragraph to verify the cache's display boundary.
	var v responseView
	body := strings.Repeat("x", responseDisplayLimit-1) + "中tail"
	v.prepare(&model.Response{Body: []byte(body)})
	if !v.rawLimited || !v.prettyLimited || len(v.raw) != responseDisplayLimit-1 || !utf8.ValidString(v.raw) {
		t.Fatal("display limit split UTF-8 or failed to cap body")
	}
	if string(v.source.Body) != body {
		t.Fatal("display cap altered the received body")
	}
	// Indentation itself can push an otherwise small body over the limit.
	v.prepare(&model.Response{Body: []byte("[" + strings.Repeat("0,", 500000) + "0]")})
	if v.rawLimited || !v.prettyLimited || len(v.pretty) > responseDisplayLimit {
		t.Fatal("formatted expansion did not obey the display cap")
	}
	v.prepare(&model.Response{Body: []byte("[" + strings.Repeat("0,", responseDisplayLimit/2) + "0]")})
	if !v.rawLimited || !v.prettyLimited || !strings.HasPrefix(v.pretty, "[\n  0,") {
		t.Fatal("large JSON did not retain Pretty formatting within the cap")
	}
}

func TestResponseTestsCountAndCacheCleanup(t *testing.T) {
	h := responseHarness(t, model.Response{Body: []byte("first")}, nil)
	h.App.Active().Tests = []TestResult{{Name: "ok", Passed: true}}
	responsePaint(h)
	if !h.shows("Tests (1)") {
		t.Fatal("test count not shown")
	}
	h.click("Tests (1)")
	if h.App.response.views[h.App.Active().ID].section != responseTests {
		t.Fatal("Tests tab did not activate")
	}
	closedID := h.App.Active().ID
	h.App.NewTab().Response = &model.Response{Body: []byte("second")}
	responsePaint(h)
	h.App.CloseTab(0)
	responsePaint(h)
	if _, retained := h.App.response.views[closedID]; retained {
		t.Fatal("closed tab's response body remains cached")
	}
	closedID = h.App.Active().ID
	h.App.NewTab()
	h.App.CloseTab(0)
	responsePaint(h)
	if _, retained := h.App.response.views[closedID]; retained {
		t.Fatal("empty active tab retained closed tab's response body")
	}
}

func BenchmarkResponseRepaint(b *testing.B) {
	a := New(fake.New())
	a.Active().Response = &model.Response{Status: "200 OK", StatusCode: 200, Body: []byte(`{"ok":true}`)}
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	config := page.Config(app.Config{Width: 1280, Height: 800})
	driver, err := app.NewDriver(config, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(driver.Close)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		config.Invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			b.Fatal(err)
		}
	}
}
