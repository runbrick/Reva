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

func historyRepaint(h *harness) {
	h.t.Helper()
	if err := h.D.Resize(1280, 800); err != nil {
		h.t.Fatal(err)
	}
}

func historyShowsEmpty(h *harness) bool {
	// Paragraphs are painted line by line at the sidebar's current width.
	return h.shows(strings.FieldsFunc(h.App.S.History.Empty, func(r rune) bool { return r == '.' || r == '。' })[0])
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

func TestHistorySendAppearsAndClickLoadsRequest(t *testing.T) {
	h := newHarness(t)
	req := model.NewRequest("sent")
	req.Method, req.URL = model.MethodPost, "https://example.test/history-send"
	req.Headers = []model.KV{{Key: "X-Test", Value: "original"}}
	req.Body = model.Body{Mode: model.BodyJSON, Text: `{"hello":"history"}`}
	h.App.Active().Request = req
	h.pump()
	h.click("Send")
	h.waitFor("send recorded", func() bool { return len(h.Svc.History.List()) == 1 })
	h.click(h.App.S.History.Title)
	if !historyHasRow(h, req.URL) || !h.shows("200") || !h.shows(h.App.S.History.Today) {
		t.Fatal("completed send, status or day missing from History")
	}
	h.App.Active().Request = model.NewRequest("replacement")
	h.pump()
	h.click("POST " + req.URL)
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
			h.click(h.App.S.History.Title)
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
	h.click(h.App.S.History.Title)
	label := "DELETE " + entry.Request.URL
	historyContext(t, h, label)
	h.click(h.App.S.History.Open)
	if !reflect.DeepEqual(h.App.Active().Request, entry.Request) {
		t.Fatal("context Open did not load request")
	}
	historyContext(t, h, label)
	h.click(h.App.S.History.Delete)
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
	h.click(h.App.S.History.Title)
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
			h.click(h.App.S.History.Title)
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
	h.click(h.App.S.History.Title)
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
	h.click(h.App.S.History.Title)
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
	h.click(h.App.S.History.Title)
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
	h.click(h.App.S.History.Title)
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
	a.sideTab = sideHistory
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
