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
	"github.com/vivaui/viva/gfx"
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

// Request and response sections can share a translated label. The response
// pane is declared after the request editor, so its exact match is the last hit.
func responseClickTab(h *harness, label string) ui.Rect {
	h.t.Helper()
	hits := h.D.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label {
			bounds := hits[i].Bounds
			if err := h.D.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
				h.t.Fatal(err)
			}
			return bounds
		}
	}
	h.t.Fatalf("no response tab named %q on screen; have %q", label, h.hitLabels())
	return ui.Rect{}
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
			{302, "302 Found", h.App.palette.Options},
			{404, "404 Not Found", h.App.palette.Warning},
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
	if !h.shows("1") || !h.shows("4") {
		t.Fatal("Pretty is missing line numbers")
	}
	h.click("Copy body")
	if got := h.D.Clipboard(); got != body {
		t.Fatalf("Copy body = %q, want full original body %q", got, body)
	}
	h.click("Raw")
	if !h.shows(body) {
		t.Fatal("Raw did not show compact response")
	}
	headersTab := responseClickTab(h, h.App.S.Response.Headers)
	var foundName, foundValue bool
	for _, label := range h.D.Snapshot().Labels {
		if label.Bounds.Y >= headersTab.Y+headersTab.Height {
			foundName = foundName || label.Value == "X-Trace"
			foundValue = foundValue || label.Value == "trace-123"
		}
	}
	if !foundName || !foundValue {
		t.Fatal("Headers did not show name and value")
	}
	responseClickTab(h, h.App.S.Response.Body)
	if !h.shows(body) {
		t.Fatal("switching sections lost Raw selection")
	}
	h.click("Pretty")
	if !h.shows(`  "z": 1,`) {
		t.Fatal("switching back to Pretty did not show indentation")
	}
}

func TestResponseSearchCountsStepsWrapsAndReveals(t *testing.T) {
	body := "first needle\n" + strings.Repeat("filler\n", 200) + "last needle"
	h := responseHarness(t, model.Response{Body: []byte(body)}, nil)
	h.click(h.App.S.Response.Search)
	h.typeInto(h.App.S.Response.SearchHint, "needle")
	v := h.App.response.views[h.App.Active().ID]
	if len(v.matches) != 2 || !h.shows("1 / 2") {
		t.Fatalf("search count = %d; labels = %v", len(v.matches), h.D.Snapshot().Labels)
	}
	h.click(h.App.S.Response.NextMatch)
	responsePaint(h)
	if v.match != 1 || !h.shows("2 / 2") || !h.shows("last needle") || v.code.view.ScrollY() <= 0 {
		t.Fatal("Next did not select and reveal the last match")
	}
	h.click(h.App.S.Response.NextMatch)
	if v.match != 0 || !h.shows("first needle") {
		t.Fatal("Next did not wrap to first match")
	}
	h.click(h.App.S.Response.PreviousMatch)
	if v.match != 1 {
		t.Fatal("Previous did not wrap to last match")
	}
	h.click(h.App.S.Response.CloseSearch)
	h.click(h.App.S.Response.CodeBody)
	if _, err := h.D.ShortcutChord(gfx.ModifierControl, 'f'); err != nil {
		t.Fatal(err)
	}
	if !v.find || !h.shows("2 / 2") {
		t.Fatal("Ctrl+F in body did not reopen find")
	}
}

func TestResponseCookiesAndTimelineFixtures(t *testing.T) {
	h := responseHarness(t, model.Response{
		Duration: 125 * time.Millisecond,
		Cookies:  []model.Cookie{{Name: "session_id", Value: "abc123", Domain: "example.test", Path: "/api", Expires: time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC), HTTPOnly: true, Secure: true}},
		Timeline: []model.Phase{{Kind: model.PhaseDNS, Duration: 5 * time.Millisecond}, {Kind: model.PhaseWait, Start: 5 * time.Millisecond, Duration: 120 * time.Millisecond}},
	}, nil)
	responseClickTab(h, h.App.S.Response.Cookies)
	for _, value := range []string{"session_id", "abc123", "example.test", "/api", "2027-01-02T03:04:05Z", "HttpOnly", "Secure", "Yes"} {
		if !h.shows(value) {
			t.Errorf("Cookie tab missing %q", value)
		}
	}
	responseClickTab(h, h.App.S.Response.Timeline)
	for _, value := range []string{"DNS lookup", "5.00 ms", "Wait for response", "120.00 ms", "Total", "125 ms"} {
		if !h.shows(value) {
			t.Errorf("Timeline missing %q", value)
		}
	}
	v := h.App.response.views[h.App.Active().ID]
	if v.phaseShares[1]/v.phaseShares[0] != 24 {
		t.Fatalf("bar shares are not proportional: %v", v.phaseShares)
	}
}

func TestResponseCookiesAndTimelineEmpty(t *testing.T) {
	h := responseHarness(t, model.Response{}, nil)
	responseClickTab(h, h.App.S.Response.Cookies)
	if !h.shows(h.App.S.Response.NoCookies) {
		t.Fatal("missing cookie empty state")
	}
	responseClickTab(h, h.App.S.Response.Timeline)
	if !h.shows(h.App.S.Response.NoTimeline) {
		t.Fatal("missing timeline empty state")
	}
}

func TestResponseHTMLPreviewAndUnsupportedPreview(t *testing.T) {
	h := responseHarness(t, model.Response{Headers: []model.KV{{Key: "Content-Type", Value: "text/html; charset=utf-8"}}, Body: []byte(`<h1>Hello &amp; welcome</h1><p>Visible <b>text</b></p><script>secret()</script>`)}, nil)
	h.click(h.App.S.Response.Preview)
	if !h.shows("Hello & welcome") || !h.shows("Visible text") || h.shows("<h1>") || h.shows("secret()") {
		t.Fatal("HTML preview did not strip tags and hidden code")
	}
	h.App.Active().Response = &model.Response{Body: []byte("plain")}
	responsePaint(h)
	if !h.shows(h.App.S.Response.PreviewUnavailable) {
		t.Fatal("unsupported preview is missing a hint")
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
	responseClickTab(h, h.App.S.Response.Headers)
	responseClickTab(h, h.App.S.Response.Body)
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

func BenchmarkResponseRepaint5000Lines(b *testing.B) {
	a := New(fake.New())
	// Brackets plus 4,998 array values make exactly 5,000 Pretty lines.
	a.Active().Response = &model.Response{Status: "200 OK", StatusCode: 200, Body: []byte("[" + strings.Repeat("1,", 4997) + "0]")}
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	config := page.Config(app.Config{Width: 1280, Height: 800})
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
	b.ResetTimer()
	for b.Loop() {
		config.Invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			b.Fatal(err)
		}
	}
}
