package gui

import (
	"context"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/responsejson"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestResponseFilterCopiesOrderIDsAndPreservesBodySearch(t *testing.T) {
	const body = ` { "orders": [{"id":9007199254740993}, {"id":"order-two"}], "other":"unfiltered" } `
	h := responseHarness(t, model.Response{StatusCode: 200, Status: "200 OK", Body: []byte(body)}, nil)
	labels := i18n.ResponseFilterFor(h.App.S.Lang)
	for range 2 {
		if err := h.D.Resize(960, 600); err != nil {
			t.Fatal(err)
		}
	}
	h.click(h.App.S.Response.Search)
	h.typeInto(h.App.S.Response.SearchHint, "unfiltered")
	h.click(labels.Open)
	h.typeInto(labels.Query, `$.orders[*].id`)
	if h.App.response.views[h.App.Active().ID].filter.result.Matches != 0 {
		t.Fatal("typing performed a query before submission")
	}
	if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	h.click(labels.Copy)
	if got := h.D.Clipboard(); got != `[9007199254740993,"order-two"]` {
		t.Fatalf("filter clipboard = %q", got)
	}
	if !h.shows(labels.Title) || !h.shows(labels.Limits) || !h.shows(`"order-two"`) {
		t.Fatal("filter dialog did not show the supported limits and queried values")
	}
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if !h.shows("unfiltered") || !h.shows("1 / 1") {
		t.Fatal("closing the filter did not retain the original body and search")
	}
	h.click(h.App.S.Response.Copy)
	if got := h.D.Clipboard(); got != body {
		t.Fatalf("body clipboard = %q, want the original body", got)
	}
}

func TestResponseFilterDiscardsResultsOnNewResponseAndIsolatesTabs(t *testing.T) {
	const query = `$.orders[1].id`
	h := responseHarness(t, model.Response{StatusCode: 200, Status: "200 OK", Body: []byte(`{"orders":[{}, {"id":"old-id"}]}`)}, nil)
	labels := i18n.ResponseFilterFor(h.App.S.Lang)
	first := h.App.Active()
	h.click(labels.Open)
	h.typeInto(labels.Query, query)
	h.click(labels.Run)
	h.click(labels.Copy)
	if h.D.Clipboard() != `["old-id"]` {
		t.Fatalf("index lookup result = %q", h.D.Clipboard())
	}
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	second := h.App.NewTab()
	second.Response = &model.Response{StatusCode: 200, Status: "200 OK", Body: []byte(`{"orders":[{}, {"id":"other-tab"}]}`)}
	responsePaint(h)
	h.click(labels.Open)
	if state := &h.App.response.views[second.ID].filter; state.query != "" || state.result.Matches != 0 {
		t.Fatal("another tab inherited the filter draft or result")
	}
	h.typeInto(labels.Query, query)
	h.click(labels.Run)
	h.click(labels.Copy)
	if h.D.Clipboard() != `["other-tab"]` {
		t.Fatalf("second tab result = %q", h.D.Clipboard())
	}
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	h.App.active = 0
	responsePaint(h)
	h.click(labels.Open)
	h.click(labels.Copy)
	if h.D.Clipboard() != `["old-id"]` {
		t.Fatalf("original tab result = %q", h.D.Clipboard())
	}
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	h.Svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		return model.Response{StatusCode: 201, Status: "201 Created", Body: []byte(`{"orders":[]}`)}, nil
	}
	h.click(h.App.S.Request.Send)
	h.waitFor("replacement response", func() bool { return first.Response != nil && first.Response.StatusCode == 201 })
	h.click(labels.Open)
	if state := &h.App.response.views[first.ID].filter; state.query != query || state.result.Matches != 0 || h.shows(`"old-id"`) {
		t.Fatal("new response did not clear the old result while retaining the query draft")
	}
	h.click(labels.Run)
	if !h.shows(labels.NoMatches) {
		t.Fatal("the new response did not report a missing array index")
	}
}

func TestResponseFilterShowsLocalizedDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name, body, query string
		truncated         bool
		message           func(*i18n.ResponseFilter) string
	}{
		{"syntax", `{}`, `$..id`, false, func(labels *i18n.ResponseFilter) string { return labels.InvalidQuery }},
		{"query limit", `{}`, "$" + strings.Repeat(".id", responsejson.FilterMaxSegments+1), false, func(labels *i18n.ResponseFilter) string { return labels.QueryLimit }},
		{"invalid JSON", `[broken`, `$`, false, func(labels *i18n.ResponseFilter) string { return labels.InvalidJSON }},
		{"input limit", strings.Repeat(" ", responsejson.FilterMaxBodyBytes) + `0`, `$`, false, func(labels *i18n.ResponseFilter) string { return labels.InputLimit }},
		{"result limit", "[" + strings.Repeat("0,", responsejson.FilterMaxMatches) + "0]", `$[*]`, false, func(labels *i18n.ResponseFilter) string { return labels.ResultLimit }},
		{"truncated", `{"completePrefix":true}`, `$`, true, func(labels *i18n.ResponseFilter) string { return labels.Truncated }},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := responseHarness(t, model.Response{StatusCode: 200, Status: "200 OK", Body: []byte(test.body), Truncated: test.truncated}, nil)
			h.App.S = i18n.For(i18n.Zh)
			responsePaint(h)
			labels := i18n.ResponseFilterFor(h.App.S.Lang)
			h.click(labels.Open)
			if !test.truncated {
				h.typeInto(labels.Query, test.query)
				h.click(labels.Run)
			}
			if !h.shows(test.message(labels)) {
				t.Fatalf("filter did not display %q", test.message(labels))
			}
			if result := h.App.response.views[h.App.Active().ID].filter.result; result != (responsejson.FilterResult{}) {
				t.Fatalf("diagnostic yielded a copyable result: %+v", result)
			}
		})
	}
}

func BenchmarkResponseFilterPaint(b *testing.B) {
	for _, mode := range []string{"toolbar", "result"} {
		b.Run(mode, func(b *testing.B) {
			a := New(fake.New())
			a.S = i18n.For(i18n.En)
			tab := a.Active()
			tab.Response = &model.Response{StatusCode: 200, Status: "200 OK", Body: []byte(`{"orders":[{"id":"A-1"},{"id":9007199254740993}]}`)}
			if mode == "result" {
				view := &responseView{}
				view.prepare(tab.Response)
				view.filter.open, view.filter.query = true, `$.orders[*].id`
				view.filter.run(tab.Response.Body)
				a.response.views = map[int]*responseView{tab.ID: view}
			}
			page := ui.NewPage(ui.PageConfig{Theme: &a.theme, View: func(c ui.C) {
				ui.Column(c).Fill().Children(func() { a.responseResult(c, tab) })
			}})
			driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 960, Height: 600}), nil)
			if err != nil {
				b.Fatal(err)
			}
			defer driver.Close()
			for range 5 {
				if err := driver.Resize(960, 600); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			for b.Loop() {
				if err := driver.Resize(960, 600); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
