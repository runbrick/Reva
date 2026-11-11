package gui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/script"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestScriptPassesAfterSend(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	tab.Request.URL = "https://example.test"
	tab.Request.TestScript = `pm.test("ok", function(){ pm.response.to.have.status(200); }); console.log("response log");`
	h.pump()
	h.click("Send")
	h.waitFor("script result", func() bool { return tab.Response != nil })
	if len(tab.Tests) != 1 || !tab.Tests[0].Passed || tab.Tests[0].Name != "ok" {
		t.Fatalf("results = %+v", tab.Tests)
	}
	h.click(fmt.Sprintf(h.App.S.Response.TestsCount, len(tab.Tests)))
	if !h.shows("ok") || !h.shows("1/1") || !h.shows("response log") {
		t.Fatal("missing passing result, summary or console log")
	}
}

func TestPreScriptHeaderReachesRequestService(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	tab.Request.URL = "https://example.test/{{path}}"
	tab.Request.Headers = []model.KV{{Key: "X-Token", Value: "old"}}
	tab.Request.PreScript = `pm.request.headers.upsert({key:"x-token", value:"fresh"}); pm.variables.set("path", "sent");`
	tab.Request.TestScript = `pm.test("sent context", function() {
        pm.expect(pm.variables.get("path")).to.equal("sent");
        pm.expect(pm.request.url).to.equal("https://example.test/sent");
    });`
	received := make(chan model.Request, 1)
	h.Svc.Requests.(*fake.Requests).Fn = func(_ context.Context, req model.Request) (model.Response, error) {
		received <- req
		return model.Response{StatusCode: 200, Status: "200 OK"}, nil
	}
	h.pump()
	h.click("Send")
	h.waitFor("scripted send", func() bool { return tab.Response != nil })
	got := <-received
	if len(got.Headers) != 1 || got.Headers[0].Value != "fresh" || got.URL != "https://example.test/sent" {
		t.Fatalf("sent = %+v", got)
	}
	if tab.Request.Headers[0].Value != "old" || tab.Request.URL != "https://example.test/{{path}}" {
		t.Fatal("pre-script changed saved request")
	}
	if len(tab.Tests) != 1 || !tab.Tests[0].Passed {
		t.Fatalf("sent context results = %+v", tab.Tests)
	}
}

func TestPreScriptErrorAndConsoleSurviveSendReset(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	tab.Request.PreScript = `console.log("pre log"); throw new Error("pre failed");`
	tab.Request.TestScript = `pm.test("response ok", function() { pm.expect(pm.response.code).to.equal(200); }); console.log("test log");`
	h.pump()
	h.click("Send")
	h.waitFor("pre error and test result", func() bool { return tab.Response != nil })
	if len(tab.Tests) != 2 || tab.Tests[0].Name != h.App.S.Scripts.PreScriptError || tab.Tests[0].Passed || !strings.Contains(tab.Tests[0].Error, "pre failed") || !tab.Tests[1].Passed {
		t.Fatalf("results = %+v", tab.Tests)
	}
	h.click(fmt.Sprintf(h.App.S.Response.TestsCount, len(tab.Tests)))
	if !h.shows("pre log") || !h.shows("test log") || !h.shows("pre failed") || !h.shows("1/2") {
		t.Fatal("missing merged pre-script diagnostics")
	}
	// A later send with no scripts must clear diagnostics from the previous run.
	tab.Request.PreScript, tab.Request.TestScript = "", ""
	h.click("Send")
	h.waitFor("second send", func() bool { return tab.Response != nil })
	if len(tab.Tests) != 0 || len(h.App.scriptTab(tab).logs) != 0 {
		t.Fatal("stale results or logs after second send")
	}
}

func TestScriptStateIsPerTabAndExceptionsBecomeFailures(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	first.Request.PreScript = `console.log("first log"); throw new Error("first failure");`
	h.App.Send(first)
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	h.waitFor("first tab result", func() bool { return first.Response != nil })
	second := h.App.NewTab()
	second.Request.TestScript = `throw new Error("second failure");`
	h.App.Send(second)
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	h.waitFor("both tab results", func() bool { return first.Response != nil && second.Response != nil })
	if len(first.Tests) != 1 || !strings.Contains(first.Tests[0].Error, "first failure") {
		t.Fatalf("first = %+v", first.Tests)
	}
	if len(second.Tests) != 1 || second.Tests[0].Name != h.App.S.Scripts.TestScriptError || !strings.Contains(second.Tests[0].Error, "second failure") {
		t.Fatalf("second = %+v", second.Tests)
	}
	if len(h.App.scriptTab(second).logs) != 0 {
		t.Fatal("logs leaked between tabs")
	}
}

func TestScriptEditorsAndSnippetsMarkDirty(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	h.click(h.App.S.Request.Scripts)
	// The TextArea and Choice both use the localized section name; select the
	// editable hit below the choice rather than depending on a numeric hit ID.
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Scripts.PreRequest && hit.Bounds.Height > 80 {
			if err := h.D.Click(hit.Bounds.X+16, hit.Bounds.Y+16); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Type(`console.log("edited");`); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if !tab.Dirty || !strings.Contains(tab.Request.PreScript, "edited") {
		t.Fatal("pre-request editor did not mark dirty")
	}
	tab.Dirty = false
	h.click(h.App.S.Scripts.Snippets)
	h.click(h.App.S.Scripts.StatusSnippetLabel)
	if !tab.Dirty || !strings.Contains(tab.Request.TestScript, "pm.response.to.have.status(200)") || !h.App.scriptTab(tab).testsEditor {
		t.Fatalf("snippet not inserted in tests: %+v", tab.Request)
	}
	h.click(h.App.S.Scripts.Snippets)
	h.click(h.App.S.Scripts.TimeSnippetLabel)
	h.click(h.App.S.Scripts.Snippets)
	h.click(h.App.S.Scripts.VariableSnippetLabel)
	if !strings.Contains(tab.Request.TestScript, ".to.be.below(500)") || !strings.Contains(tab.Request.TestScript, "pm.response.json().token") {
		t.Fatalf("missing snippets: %s", tab.Request.TestScript)
	}
}

func TestScriptSnippetsRunAsES5InBothLanguages(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		t.Run(string(lang), func(t *testing.T) {
			text := i18n.For(lang).Scripts
			if !strings.Contains(text.Help, "ES5") {
				t.Fatal("missing ES5 help")
			}
			for _, source := range []string{text.StatusSnippet, text.TimeSnippet, text.VariableSnippet} {
				results, _, err := script.RunTests(source, model.Request{}, model.Response{StatusCode: 200, Body: []byte(`{"token":"example"}`)}, nil, 0)
				if err != nil {
					t.Fatal(err)
				}
				for _, result := range results {
					if !result.Passed {
						t.Fatalf("snippet failed: %+v", result)
					}
				}
			}
		})
	}
}

func BenchmarkScriptSectionsPaint(b *testing.B) {
	svc := fake.New()
	a := New(svc)
	a.Active().Tests = []TestResult{{Name: "status", Passed: true}, {Name: "body", Error: "missing field"}}
	page := ui.NewPage(ui.PageConfig{Theme: &a.theme, View: func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Height(300).Children(func() { a.scriptsSection(c, a.Active()) })
			ui.Column(c).FillWidth().Grow(1).Children(func() { a.testsSection(c, a.Active()) })
		})
	}})
	d, err := app.NewDriver(page.Config(app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	for range 5 {
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
