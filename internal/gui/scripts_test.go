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
	"github.com/vivaui/viva/gfx"
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

func focusScriptEditor(t *testing.T, h *harness, label string) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label && hit.Bounds.Height > 80 {
			if err := h.D.Click(hit.Bounds.X+60, hit.Bounds.Y+20); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("missing script editor %q", label)
}

func TestScriptCodeEditorEditingAndIndependentState(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	h.click(h.App.S.Request.Scripts)
	focusScriptEditor(t, h, h.App.S.Scripts.PreRequest)
	if err := h.D.Type("var count = 42;"); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Type("pm.variables.set('count', count);"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tab.Request.PreScript, "\n    pm.variables") || !tab.Dirty {
		t.Fatalf("script editing or dirty flag failed: %q", tab.Request.PreScript)
	}
	if err := h.D.Type("\nconsole.log(count);"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tab.Request.PreScript, "\nconsole.log(count);") {
		t.Fatalf("multiline input was flattened: %q", tab.Request.PreScript)
	}
	if _, err := h.D.ShortcutChord(ui.Primary, 'z'); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(tab.Request.PreScript, "console.log") {
		t.Fatal("undo did not update the saved script")
	}
	h.click(h.App.S.Scripts.Tests)
	focusScriptEditor(t, h, h.App.S.Scripts.Tests)
	if err := h.D.Type("pm.test('ok', function() {});"); err != nil {
		t.Fatal(err)
	}
	if tab.Request.TestScript == "" || strings.Contains(tab.Request.PreScript, "pm.test") {
		t.Fatal("pre-request and test editors share content")
	}
	h.click(h.App.S.Scripts.PreRequest)
	if !strings.HasPrefix(tab.Request.PreScript, "var count = 42;") {
		t.Fatal("switching scripts lost edits")
	}
}

func TestScriptCodeEditorLineNumbersAndSyntaxInThemes(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	tab.Request.PreScript = "var count = 42; // note\n/* across\nlines */ pm.variables.set('x', true);"
	h.click(h.App.S.Request.Scripts)
	for _, mode := range []string{"light", "dark"} {
		h.App.ApplySettings(model.Settings{Language: "en", Theme: mode})
		if err := h.D.Resize(1280, 800); err != nil {
			t.Fatal(err)
		}
		v := &h.App.scriptTab(tab).preEditor
		if !v.snapshot.LineNumbers || v.view.LineCount() != 3 {
			t.Fatalf("%s: line numbers missing: %d", mode, v.view.LineCount())
		}
		if !h.shows("1") || !h.shows("2") || !h.shows("3") {
			t.Fatalf("%s: line number labels missing", mode)
		}
		for _, want := range []gfx.Color{h.App.palette.CodeLiteral, h.App.palette.CodeNumber, h.App.palette.CodeString, h.App.palette.CodeLineNo, h.App.palette.CodeKey} {
			found := false
			for _, label := range h.D.Snapshot().Labels {
				for _, run := range label.Runs {
					found = found || run.Color == want
				}
			}
			if !found {
				t.Errorf("%s: syntax color %+v not painted", mode, want)
			}
		}
	}
}

func TestScriptCodeEditorClipboardPreservesNewlines(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	h.click(h.App.S.Request.Scripts)
	focusScriptEditor(t, h, h.App.S.Scripts.PreRequest)
	h.D.SetClipboard("var first = 1;\nvar second = 2;")
	if _, err := h.D.ShortcutChord(ui.Primary, 'v'); err != nil {
		t.Fatal(err)
	}
	if tab.Request.PreScript != h.D.Clipboard() {
		t.Fatalf("paste changed line breaks: %q", tab.Request.PreScript)
	}
	if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
		t.Fatal(err)
	}
	h.D.SetClipboard("")
	if _, err := h.D.ShortcutChord(ui.Primary, 'c'); err != nil {
		t.Fatal(err)
	}
	if h.D.Clipboard() != tab.Request.PreScript {
		t.Fatalf("copied %q, want %q", h.D.Clipboard(), tab.Request.PreScript)
	}
}

func TestScriptCodeEditorScrollsLongScripts(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	tab.Request.PreScript = strings.Repeat("console.log('line');\n", 200)
	h.click(h.App.S.Request.Scripts)
	v := &h.App.scriptTab(tab).preEditor
	if v.view.LineCount() != 201 {
		t.Fatalf("line count = %d", v.view.LineCount())
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Scripts.PreRequest && hit.Bounds.Height > 80 {
			if err := h.D.Scroll(gfx.ScrollEvent{X: hit.Bounds.X + 100, Y: hit.Bounds.Y + 30, DeltaY: 320, Precise: true}); err != nil {
				t.Fatal(err)
			}
			if v.view.ScrollY() <= 0 {
				t.Fatal("editor did not scroll")
			}
			return
		}
	}
	t.Fatal("script editor hit not found")
}

func TestScriptCompletionKeyboardAndContext(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	h.click(h.App.S.Request.Scripts)
	focusScriptEditor(t, h, h.App.S.Scripts.PreRequest)
	if err := h.D.Type("pm.re"); err != nil {
		t.Fatal(err)
	}
	v := &h.App.scriptTab(tab).preEditor
	if len(v.completions) != 1 || v.completions[0].name != "request" || !h.shows("request") {
		t.Fatalf("expected request suggestion, got %+v", v.completions)
	}
	if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if tab.Request.PreScript != "pm.request" {
		t.Fatalf("Enter did not accept completion: %q", tab.Request.PreScript)
	}
	if err := h.D.Type(".hea"); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if tab.Request.PreScript != "pm.request.headers" {
		t.Fatalf("Tab did not accept member completion: %q", tab.Request.PreScript)
	}
	if err := h.D.Type(".up"); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if tab.Request.PreScript != "pm.request.headers.upsert()" || v.editor.Caret() != len([]rune(tab.Request.PreScript))-1 {
		t.Fatalf("call completion or caret failed: %q at %d", tab.Request.PreScript, v.editor.Caret())
	}
	h.click(h.App.S.Scripts.Tests)
	focusScriptEditor(t, h, h.App.S.Scripts.Tests)
	if err := h.D.Type("pm.res"); err != nil {
		t.Fatal(err)
	}
	if got := h.App.scriptTab(tab).testEditor.completions; len(got) != 1 || got[0].name != "response" {
		t.Fatalf("test script response suggestion = %+v", got)
	}
}

func TestScriptCompletionClickAndIgnoreStrings(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	h.click(h.App.S.Request.Scripts)
	focusScriptEditor(t, h, h.App.S.Scripts.PreRequest)
	if err := h.D.Type("pm."); err != nil {
		t.Fatal(err)
	}
	h.click("variables")
	if tab.Request.PreScript != "pm.variables" {
		t.Fatalf("click did not insert completion: %q", tab.Request.PreScript)
	}
	if err := h.D.Type(".get"); err != nil {
		t.Fatal(err)
	}
	if tab.Request.PreScript != "pm.variables.get" {
		t.Fatalf("click did not restore editor focus: %q", tab.Request.PreScript)
	}
	if err := h.D.Type("('pm.re"); err != nil {
		t.Fatal(err)
	}
	if got := h.App.scriptTab(tab).preEditor.completions; len(got) != 0 {
		t.Fatalf("suggestions inside string: %+v", got)
	}
}

func TestScriptCompletionNavigationAndDismiss(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	h.click(h.App.S.Request.Scripts)
	focusScriptEditor(t, h, h.App.S.Scripts.PreRequest)
	if err := h.D.Type("pm."); err != nil {
		t.Fatal(err)
	}
	v := &h.App.scriptTab(tab).preEditor
	if err := h.D.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if tab.Request.PreScript != "pm.variables" {
		t.Fatalf("arrow selection inserted %q", tab.Request.PreScript)
	}
	if err := h.D.Type(".s"); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if !v.dismissed {
		t.Fatal("Escape did not dismiss suggestions")
	}
	if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(tab.Request.PreScript, ".s\n") {
		t.Fatalf("Enter accepted a dismissed suggestion: %q", tab.Request.PreScript)
	}
	if err := h.D.Type("Math.fl"); err != nil {
		t.Fatal(err)
	}
	if got := v.completions; len(got) != 1 || got[0].name != "floor" {
		t.Fatalf("ES5 builtin suggestions did not reopen: %+v", got)
	}
}

func TestScriptEditorDragResizesAndKeepsPerScriptHeight(t *testing.T) {
	h := newHarness(t)
	h.click(h.App.S.Request.Scripts)
	v := &h.App.scriptTab(h.App.Active()).preEditor
	before := v.height
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Scripts.ResizeEditor {
			x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
			if err := h.D.Press(x, y, gfx.PointerButtonPrimary); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Move(x, y+72); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Release(x, y+72, gfx.PointerButtonPrimary); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if v.height <= before || v.height > Space6*34 {
		t.Fatalf("resize height = %v, initial = %v", v.height, before)
	}
	grown := v.height
	h.App.mainSplit.Ratio = 0
	h.App.mainSplit.Pos = 500
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	foundGrip := false
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label != h.App.S.Scripts.ResizeEditor {
			continue
		}
		foundGrip = true
		x, y := hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2
		if err := h.D.Press(x, y, gfx.PointerButtonPrimary); err != nil {
			t.Fatal(err)
		}
		if err := h.D.Move(x, y-48); err != nil {
			t.Fatal(err)
		}
		if err := h.D.Release(x, y-48, gfx.PointerButtonPrimary); err != nil {
			t.Fatal(err)
		}
		break
	}
	if !foundGrip || v.height >= grown {
		t.Fatalf("editor did not shrink: %v -> %v", grown, v.height)
	}
	h.click(h.App.S.Scripts.Tests)
	if h.App.scriptTab(h.App.Active()).testEditor.height != Space6*6 {
		t.Fatal("test editor inherited pre-request editor height")
	}
	h.click(h.App.S.Scripts.PreRequest)
	if v.height <= before {
		t.Fatal("resized height was not kept after switching scripts")
	}
}

func TestScriptLexerDoesNotHighlightTokensInsideStringsAndComments(t *testing.T) {
	runs := lexScript("var x = 'http://example/true'; // false 12\n/* var\ntrue */ pm.test(42);")
	if len(runs) != 3 {
		t.Fatalf("lines = %d", len(runs))
	}
	for line, tokens := range runs {
		last := 0
		for _, token := range tokens {
			if token.start < last || token.end <= token.start {
				t.Fatalf("line %d: unordered or empty token: %+v", line, token)
			}
			last = token.end
		}
	}
	if runs[0][2].kind != scriptString || runs[1][0].kind != scriptComment || runs[2][0].kind != scriptComment {
		t.Fatalf("string and block comment tokens = %+v", runs)
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
