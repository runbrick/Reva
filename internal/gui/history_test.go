package gui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func historyEntry(id, url string, method model.Method, at time.Time) model.HistoryEntry {
	req := model.NewRequest("request-" + id)
	req.URL, req.Method = url, method
	return model.HistoryEntry{ID: id, Request: req, At: at, StatusCode: 200}
}

func historyHasRow(h *harness, url string) bool {
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Role == gfx.RoleOption && strings.Contains(hit.Label, url) {
			return true
		}
	}
	return false
}

// historyOpenPage shows the History page. The titlebar PR adds the visible
// navigation item; until then the tests select the page directly.
func historyOpenPage(h *harness) {
	h.t.Helper()
	h.App.Main = PageHistory
	historyRepaint(h)
}

func historyRepaint(h *harness) {
	h.t.Helper()
	if err := h.D.Resize(1280, 800); err != nil {
		h.t.Fatal(err)
	}
}

func historyShowsEmpty(h *harness) bool {
	// Paragraphs are painted line by line at the current width.
	return h.shows(strings.FieldsFunc(h.App.S.History.Empty, func(r rune) bool { return r == '.' || r == '。' })[0])
}

// Action labels repeat across rows, the detail panel and context menu. Match
// their role and use the last button to address the selected entry's detail.
func historyClickAction(h *harness, label string, role gfx.Role) {
	h.t.Helper()
	hits := h.D.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		hit := hits[i]
		if hit.Label == label && hit.Role == role {
			b := hit.Bounds
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				h.t.Fatal(err)
			}
			return
		}
	}
	h.t.Fatalf("no %s action %q", role, label)
}

func historyContext(t *testing.T, h *harness, label string) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label {
			b := hit.Bounds
			if err := h.D.ClickButton(b.X+b.Width/2, b.Y+b.Height/2, gfx.PointerButtonSecondary); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no history row %q", label)
}

func TestHistorySendAppearsAndOpenReturnsToWorkbench(t *testing.T) {
	h := newHarness(t)
	req := model.NewRequest("sent")
	req.Method, req.URL = model.MethodPost, "https://example.test/history-send"
	req.Headers = []model.KV{{Key: "X-Test", Value: "original"}}
	req.Body = model.Body{Mode: model.BodyJSON, Text: `{"hello":"history"}`}
	h.App.Active().Request = req
	h.pump()
	h.click("Send")
	h.waitFor("send recorded", func() bool { return len(h.Svc.History.List()) == 1 })
	historyOpenPage(h)
	if !historyHasRow(h, req.URL) || !h.shows("200") || !h.shows(h.App.S.History.Today) {
		t.Fatal("completed send, status or day missing from History")
	}
	h.App.Active().Request = model.NewRequest("replacement")
	h.pump()
	h.click("POST " + req.URL)
	if h.App.Main != PageHistory || !h.shows(h.App.S.History.Detail) || !h.shows("X-Test: original") || !h.shows(req.Body.Text) {
		t.Fatal("selecting a row did not show its request details")
	}
	historyClickAction(h, h.App.S.History.Open, gfx.RoleButton)
	if h.App.Main != PageWorkbench {
		t.Fatal("Open did not return to workbench")
	}
	if !reflect.DeepEqual(h.App.Active().Request, req) {
		t.Fatalf("opened request = %#v, want %#v", h.App.Active().Request, req)
	}
	h.App.Active().Request.Headers[0].Value = "edited"
	if h.Svc.History.List()[0].Request.Headers[0].Value != "original" {
		t.Fatal("editing opened request changed history")
	}
}

func TestHistorySearchFiltersURLAndMethod(t *testing.T) {
	for _, tc := range []struct {
		query, want, absent string
	}{
		{"ALPHA", "https://example.test/alpha", "https://example.test/beta"},
		{"pOs", "https://example.test/beta", "https://example.test/alpha"},
		{"not-present", "", "https://example.test/alpha"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			h := newHarness(t)
			for _, entry := range []model.HistoryEntry{
				historyEntry("alpha", "https://example.test/alpha", model.MethodGet, time.Now()),
				historyEntry("beta", "https://example.test/beta", model.MethodPost, time.Now()),
			} {
				if err := h.Svc.History.Add(entry); err != nil {
					t.Fatal(err)
				}
			}
			historyOpenPage(h)
			h.typeInto(h.App.S.History.Search, tc.query)
			if historyHasRow(h, tc.absent) || tc.want != "" && !historyHasRow(h, tc.want) {
				t.Fatalf("search %q has wrong rows: %q", tc.query, h.hitLabels())
			}
			if tc.want == "" && !h.shows(h.App.S.History.NoMatches) {
				t.Fatal("no-matches state missing")
			}
			if err := h.D.Key(gfx.KeyHome, 0); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Key(gfx.KeyEnd, gfx.ModifierShift); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Key(gfx.KeyBackspace, 0); err != nil {
				t.Fatal(err)
			}
			if !historyHasRow(h, tc.absent) {
				t.Fatal("clearing search did not restore rows")
			}
		})
	}
}

func TestHistoryContextOpenDeleteAndExternalDelete(t *testing.T) {
	h := newHarness(t)
	entry := historyEntry("context", "https://example.test/context", model.MethodDelete, time.Now())
	if err := h.Svc.History.Add(entry); err != nil {
		t.Fatal(err)
	}
	historyOpenPage(h)
	label := "DELETE " + entry.Request.URL
	historyContext(t, h, label)
	historyClickAction(h, h.App.S.History.Open, gfx.RoleMenuItem)
	if !reflect.DeepEqual(h.App.Active().Request, entry.Request) {
		t.Fatal("context Open did not load request")
	}
	historyOpenPage(h)
	historyContext(t, h, label)
	historyClickAction(h, h.App.S.History.Delete, gfx.RoleMenuItem)
	if len(h.Svc.History.List()) != 0 || historyHasRow(h, entry.Request.URL) || !historyShowsEmpty(h) {
		t.Fatalf("context Delete did not empty history: entries=%d, hits=%q, labels=%v", len(h.Svc.History.List()), h.hitLabels(), h.D.Snapshot().Labels)
	}
	if err := h.Svc.History.Add(entry); err != nil {
		t.Fatal(err)
	}
	historyRepaint(h)
	if !historyHasRow(h, entry.Request.URL) {
		t.Fatal("external Add did not refresh history")
	}
	if err := h.Svc.History.Delete(entry.ID); err != nil {
		t.Fatal(err)
	}
	historyRepaint(h)
	if historyHasRow(h, entry.Request.URL) || !historyShowsEmpty(h) {
		t.Fatal("external Delete did not refresh history")
	}
}

func TestHistoryClearRequiresConfirmation(t *testing.T) {
	h := newHarness(t)
	for i := range 3 {
		if err := h.Svc.History.Add(historyEntry(fmt.Sprint(i), fmt.Sprintf("https://example.test/%d", i), model.MethodGet, time.Now())); err != nil {
			t.Fatal(err)
		}
	}
	historyOpenPage(h)
	h.click(h.App.S.History.Clear)
	if !h.shows(h.App.S.History.ConfirmTitle) || len(h.Svc.History.List()) != 3 {
		t.Fatal("Clear must ask before deleting")
	}
	h.click(h.App.S.History.Cancel)
	if len(h.Svc.History.List()) != 3 || h.App.history.confirmClear {
		t.Fatal("Cancel deleted history or left dialog open")
	}
	// Clearing from a search with no matches still clears the whole history.
	h.typeInto(h.App.S.History.Search, "no-match")
	h.click(h.App.S.History.Clear)
	h.click(h.App.S.History.ConfirmClear)
	if len(h.Svc.History.List()) != 0 || !historyShowsEmpty(h) || h.App.history.confirmClear {
		t.Fatalf("confirmed Clear did not empty history and close dialog: entries=%d, open=%v, labels=%v", len(h.Svc.History.List()), h.App.history.confirmClear, h.D.Snapshot().Labels)
	}
}

func TestHistoryEmptyStateInBothLanguages(t *testing.T) {
	for _, lang := range i18n.Langs {
		t.Run(string(lang), func(t *testing.T) {
			svc := fake.New()
			settings := svc.Settings.Get()
			settings.Language = string(lang)
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			historyOpenPage(h)
			if !historyShowsEmpty(h) || h.shows(h.App.S.History.NoMatches) {
				t.Fatalf("empty History has wrong message: %v", h.D.Snapshot().Labels)
			}
		})
	}
}

func TestHistoryGroupsDatesAndShowsTransportError(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	old := now.AddDate(0, 0, -3)
	for _, entry := range []model.HistoryEntry{
		historyEntry("old", "https://example.test/old", model.MethodGet, old),
		historyEntry("yesterday", "https://example.test/yesterday", model.MethodGet, now.AddDate(0, 0, -1)),
	} {
		if err := h.Svc.History.Add(entry); err != nil {
			t.Fatal(err)
		}
	}
	h.Svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		return model.Response{}, errors.New("connection refused")
	}
	h.App.Active().Request.URL = "https://example.test/failed"
	h.click("Send")
	h.waitFor("failed send recorded", func() bool { return len(h.Svc.History.List()) == 3 })
	historyOpenPage(h)
	for _, text := range []string{h.App.S.History.Today, h.App.S.History.Yesterday, old.Format(h.App.S.History.DateFormat), h.App.S.History.Error} {
		if !h.shows(text) {
			t.Fatalf("history is missing %q", text)
		}
	}
}

func TestHistoryEnterSubmitsAndSkipsHeaders(t *testing.T) {
	h := newHarness(t)
	first := historyEntry("first", "https://example.test/first", model.MethodGet, time.Now().AddDate(0, 0, -1))
	second := historyEntry("second", "https://example.test/second", model.MethodPost, time.Now())
	for _, entry := range []model.HistoryEntry{first, second} {
		if err := h.Svc.History.Add(entry); err != nil {
			t.Fatal(err)
		}
	}
	historyOpenPage(h)
	h.click("POST " + second.Request.URL)
	if err := h.D.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if h.App.Active().Request.URL != first.Request.URL {
		t.Fatalf("Enter did not open selected row after skipping a day header: request=%s selected=%d hits=%q", h.App.Active().Request.URL, h.App.history.list.Selected(), h.hitLabels())
	}
}

func TestHistoryRefreshesWhenCappedListGetsNewEntry(t *testing.T) {
	svc := fake.New()
	svc.History.(*fake.History).Limit = 1
	h := newHarness(t, svc)
	old := historyEntry("old", "https://example.test/old", model.MethodGet, time.Now())
	newest := historyEntry("new", "https://example.test/new", model.MethodPost, time.Now())
	if err := svc.History.Add(old); err != nil {
		t.Fatal(err)
	}
	historyOpenPage(h)
	if err := svc.History.Add(newest); err != nil {
		t.Fatal(err)
	}
	historyRepaint(h)
	if historyHasRow(h, old.Request.URL) || !historyHasRow(h, newest.Request.URL) {
		t.Fatal("same-length history did not refresh for newest ID")
	}
}

func TestHistoryLongListCanOpenOldestRequest(t *testing.T) {
	h := newHarness(t)
	for i := range 200 {
		if err := h.Svc.History.Add(historyEntry(fmt.Sprint(i), fmt.Sprintf("https://example.test/long/%d", i), model.MethodGet, time.Now())); err != nil {
			t.Fatal(err)
		}
	}
	historyOpenPage(h)
	if !historyHasRow(h, "https://example.test/long/199") || historyHasRow(h, "https://example.test/long/0") {
		t.Fatal("long history should initially show the newest requests")
	}
	h.click("GET https://example.test/long/199")
	if err := h.D.Key(gfx.KeyEnd, 0); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if h.App.Active().Request.URL != "https://example.test/long/0" {
		t.Fatal("oldest request could not be reached and opened in long history")
	}
}

func BenchmarkHistoryRepaint(b *testing.B) {
	svc := fake.New()
	for i := range 200 {
		if err := svc.History.Add(historyEntry(fmt.Sprint(i), fmt.Sprintf("https://example.test/history/%d", i), model.MethodGet, time.Now())); err != nil {
			b.Fatal(err)
		}
	}
	a := New(svc)
	a.Main = PageHistory
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	for range 8 {
		if err := d.Resize(1280, 800); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := d.Resize(1280, 800); err != nil {
			b.Fatal(err)
		}
	}
}

func TestHistoryFiltersMethodStatusAndSearchTogether(t *testing.T) {
	h := newHarness(t)
	entries := []model.HistoryEntry{
		historyEntry("get-ok", "https://example.test/get-ok", model.MethodGet, time.Now()),
		historyEntry("post-ok", "https://example.test/post-ok", model.MethodPost, time.Now()),
		historyEntry("post-redirect", "https://example.test/post-redirect", model.MethodPost, time.Now()),
		historyEntry("post-client", "https://example.test/post-client", model.MethodPost, time.Now()),
		historyEntry("post-server", "https://example.test/post-server", model.MethodPost, time.Now()),
		historyEntry("post-error", "https://example.test/post-error", model.MethodPost, time.Now()),
		historyEntry("head", "https://example.test/head", model.MethodHead, time.Now()),
	}
	entries[2].StatusCode = 302
	entries[3].StatusCode = 404
	entries[4].StatusCode = 503
	entries[5].StatusCode, entries[5].Error = 0, "connection refused"
	for _, entry := range entries {
		if err := h.Svc.History.Add(entry); err != nil {
			t.Fatal(err)
		}
	}
	historyOpenPage(h)
	h.click("POST")
	if historyHasRow(h, entries[0].Request.URL) || historyHasRow(h, entries[6].Request.URL) {
		t.Fatal("method chip did not filter rows")
	}
	text := h.App.S.History
	for _, tc := range []struct {
		label string
		index int
	}{{text.Success, 1}, {text.Redirect, 2}, {text.ClientError, 3}, {text.ServerError, 4}, {text.Error, 5}} {
		h.click(tc.label)
		for i, entry := range entries {
			if historyHasRow(h, entry.Request.URL) != (i == tc.index) {
				t.Fatalf("status %q shows wrong rows: %q", tc.label, h.hitLabels())
			}
		}
	}
	h.typeInto(text.Search, "not-present")
	if !h.shows(text.NoMatches) {
		t.Fatal("search did not combine with chips")
	}
	h.click(text.ClearSearch)
	if !historyHasRow(h, entries[5].Request.URL) {
		t.Fatal("clear search lost chip filters")
	}
	h.click(text.AllStatuses)
	h.click(text.Other)
	if !historyHasRow(h, entries[6].Request.URL) || historyHasRow(h, entries[1].Request.URL) {
		t.Fatal("Other chip did not isolate uncommon methods")
	}
	h.click(text.AllMethods)
	if !historyHasRow(h, entries[0].Request.URL) || !historyHasRow(h, entries[1].Request.URL) {
		t.Fatal("All chips did not restore rows")
	}
}

func TestHistoryResendRecordsAnotherEntry(t *testing.T) {
	h := newHarness(t)
	entry := historyEntry("resend", "https://example.test/resend", model.MethodPatch, time.Now())
	entry.Request.Headers = []model.KV{{Key: "X-Resend", Value: "original"}}
	entry.Request.Body = model.Body{Mode: model.BodyJSON, Text: `{"resend":true}`}
	if err := h.Svc.History.Add(entry); err != nil {
		t.Fatal(err)
	}
	sent := make(chan model.Request, 1)
	h.Svc.Requests.(*fake.Requests).Fn = func(_ context.Context, request model.Request) (model.Response, error) {
		sent <- request
		return model.Response{StatusCode: 201, Duration: 25 * time.Millisecond}, nil
	}
	historyOpenPage(h)
	h.click("PATCH " + entry.Request.URL)
	historyClickAction(h, h.App.S.History.Resend, gfx.RoleButton)
	h.waitFor("re-send recorded", func() bool { return len(h.Svc.History.List()) == 2 })
	if h.App.Main != PageWorkbench {
		t.Fatal("Re-send did not open workbench")
	}
	select {
	case request := <-sent:
		if request.URL != entry.Request.URL || request.Method != entry.Request.Method || !reflect.DeepEqual(request.Headers, entry.Request.Headers) || request.Body.Text != entry.Request.Body.Text || request.Body.Mode != entry.Request.Body.Mode {
			t.Fatalf("re-sent wrong request: %#v", request)
		}
	default:
		t.Fatal("fake request service was not called")
	}
	if latest := h.Svc.History.List()[0]; latest.ID == entry.ID || latest.StatusCode != 201 || latest.Duration != 25*time.Millisecond {
		t.Fatalf("wrong new history entry: %#v", latest)
	}
}

func TestHistorySelectedRowCopyAndDelete(t *testing.T) {
	h := newHarness(t)
	entry := historyEntry("copy", "https://example.test/copy?full=yes", model.MethodGet, time.Now())
	if err := h.Svc.History.Add(entry); err != nil {
		t.Fatal(err)
	}
	historyOpenPage(h)
	h.click("GET " + entry.Request.URL)
	h.click(h.App.S.History.CopyURL)
	if h.D.Clipboard() != entry.Request.URL {
		t.Fatalf("copied URL = %q", h.D.Clipboard())
	}
	h.click(h.App.S.History.Delete)
	if len(h.Svc.History.List()) != 0 || !h.shows(h.App.S.History.SelectEntry) || !historyShowsEmpty(h) {
		t.Fatal("deleting selected row left stale history or details")
	}
}

func TestHistoryDetailsBothLanguagesThemesAndNarrowLayout(t *testing.T) {
	for _, lang := range i18n.Langs {
		for _, theme := range []string{"light", "dark"} {
			for _, width := range []int{1280, 960} {
				t.Run(fmt.Sprintf("%s/%s/%d", lang, theme, width), func(t *testing.T) {
					svc := fake.New()
					settings := svc.Settings.Get()
					settings.Language, settings.Theme = string(lang), theme
					if err := svc.Settings.Save(settings); err != nil {
						t.Fatal(err)
					}
					entry := historyEntry("detail", "https://example.test/details", model.MethodPut, time.Now())
					entry.Duration = 125 * time.Millisecond
					entry.Error, entry.StatusCode = "connection refused", 0
					entry.Request.Headers = []model.KV{{Key: "X-Detail", Value: "shown"}, {Key: "X-Disabled", Value: "hidden", Disabled: true}}
					entry.Request.Body = model.Body{Mode: model.BodyRaw, Text: "request preview"}
					if err := svc.History.Add(entry); err != nil {
						t.Fatal(err)
					}
					h := newHarness(t, svc)
					h.App.Main = PageHistory
					if err := h.D.Resize(width, 800); err != nil {
						t.Fatal(err)
					}
					h.click("PUT " + entry.Request.URL)
					for _, label := range []string{h.App.S.History.Detail, h.App.S.History.Resend, "connection refused", "125ms"} {
						if !h.shows(label) {
							t.Fatalf("detail missing %q", label)
						}
					}
					if h.shows("X-Disabled") {
						t.Fatal("disabled request header shown as sent")
					}
					if width == 1280 && (!h.shows("X-Detail: shown") || !h.shows("request preview")) {
						t.Fatal("request details missing")
					}
					// Layout checks cover this feature's full page; ignore the shell's rail
					// because its icon labels intentionally use a narrower fixed width.
					for _, problem := range headless.CheckLayout(h.D.Snapshot()) {
						if problem.Bounds.X >= RailWidth && problem.Bounds.Y > TitlebarHeight {
							t.Errorf("history layout: %+v", problem)
						}
					}
				})
			}
		}
	}
}
