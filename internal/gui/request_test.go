package gui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func requestEnglish(h *harness) {
	h.App.S = i18n.For(i18n.En)
	requestRepaint(h)
}

func requestRepaint(h *harness) {
	h.t.Helper()
	if err := h.D.Resize(1280, 800); err != nil {
		h.t.Fatal(err)
	}
}

// Fields and their enabled checkboxes intentionally share Common labels; find
// the editable field by role, just as assistive technology does.
func requestField(h *harness, label string, row int) gfx.Hit {
	h.t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label && hit.Role == gfx.RoleTextField {
			if row == 0 {
				return hit
			}
			row--
		}
	}
	h.t.Fatalf("missing field %q; have %q", label, h.hitLabels())
	return gfx.Hit{}
}

func requestType(h *harness, label, text string, replace bool) {
	h.t.Helper()
	hit := requestField(h, label, 0)
	if err := h.D.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		h.t.Fatal(err)
	}
	if replace {
		if _, err := h.D.ShortcutChord(ui.Primary, 'a'); err != nil {
			h.t.Fatal(err)
		}
	}
	if err := h.D.Type(text); err != nil {
		h.t.Fatal(err)
	}
}

func requestCapture(h *harness) <-chan model.Request {
	requests := make(chan model.Request, 1)
	h.Svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, req model.Request) (model.Response, error) {
		requests <- req
		return model.Response{StatusCode: 200, Status: "200 OK"}, nil
	}
	return requests
}

func requestSent(h *harness, requests <-chan model.Request) model.Request {
	h.t.Helper()
	h.click("Send")
	h.waitFor("sent response", func() bool { return h.App.Active().Response != nil })
	select {
	case req := <-requests:
		return req
	default:
		h.t.Fatal("fake did not receive request")
		return model.Request{}
	}
}

func TestRequestTypedURLSendAndDirty(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	requests := requestCapture(h)
	h.typeInto("URL", "https://example.test/ping")
	if !h.App.Active().Dirty {
		t.Fatal("typing URL did not mark dirty")
	}
	if req := requestSent(h, requests); req.URL != "https://example.test/ping" {
		t.Fatalf("sent URL = %q", req.URL)
	}
	h.pump()
	if !h.shows("200 OK") {
		t.Fatal("response status missing")
	}
}

func TestRequestParamsSyncBothWays(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	h.typeInto("URL", "https://{{host}}/path#anchor")
	h.click("Add")
	requestType(h, "Key", "q", false)
	requestType(h, "Value", "{{term}} hello", false)
	tab := h.App.Active()
	if want := "https://{{host}}/path?q={{term}}+hello#anchor"; tab.Request.URL != want {
		t.Fatalf("URL = %q, want %q", tab.Request.URL, want)
	}
	requestType(h, "URL", "https://{{host}}/path?x=one&x=two+words&token={{token}}#frag", true)
	want := []model.KV{{Key: "x", Value: "one"}, {Key: "x", Value: "two words"}, {Key: "token", Value: "{{token}}"}}
	if !reflect.DeepEqual(tab.Request.Params, want) {
		t.Fatalf("params = %#v", tab.Request.Params)
	}
	// Turning off a row removes only that value from the URL, retaining its row.
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Role == gfx.RoleCheckbox {
			if err := h.D.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if !tab.Request.Params[0].Disabled || strings.Contains(tab.Request.URL, "x=one") {
		t.Fatalf("disabled param still in URL: %#v", tab.Request)
	}
	h.click("Delete")
	if len(tab.Request.Params) != 2 || tab.Request.Params[0].Value != "two words" {
		t.Fatalf("delete lost remaining row: %#v", tab.Request.Params)
	}
	requestType(h, "Value", "updated", true)
	if !strings.Contains(tab.Request.URL, "x=updated") {
		t.Fatalf("surviving row not editable: %s", tab.Request.URL)
	}
}

func TestRequestJSONBodyReachesServiceAndFormats(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	requests := requestCapture(h)
	h.click("Body")
	h.click("JSON")
	body := `{"message":"hello","n":2}`
	h.typeInto("Body content", body)
	if req := requestSent(h, requests); req.Body.Mode != model.BodyJSON || req.Body.Text != body {
		t.Fatalf("body = %#v", req.Body)
	}
	h.App.Active().Dirty = false
	h.click("Format")
	if !strings.Contains(h.App.Active().Request.Body.Text, "\n  ") || !h.App.Active().Dirty {
		t.Fatal("format did not indent and mark dirty")
	}
	requestType(h, "Body content", "{bad", true)
	h.click("Format")
	if h.App.Active().Request.Body.Text != "{bad" || !h.shows("Invalid JSON") {
		t.Fatal("invalid JSON should stay editable and show error")
	}
}

func TestRequestAuthFieldsReachService(t *testing.T) {
	for _, kind := range []model.AuthType{model.AuthBearer, model.AuthBasic, model.AuthAPIKey} {
		t.Run(string(kind), func(t *testing.T) {
			h := newHarness(t)
			requestEnglish(h)
			requests := requestCapture(h)
			h.click("Auth")
			var want model.Auth
			switch kind {
			case model.AuthBearer:
				h.click("Bearer")
				h.typeInto("Token", "secret-token")
				want = model.Auth{Type: kind, Token: "secret-token"}
			case model.AuthBasic:
				h.click("Basic")
				h.typeInto("Username", "alice")
				h.typeInto("Password", "secret-password")
				if hit := requestField(h, "Password", 0); !hit.Secure || hit.Value != "" {
					t.Fatal("password is exposed to accessibility")
				}
				want = model.Auth{Type: kind, Username: "alice", Password: "secret-password"}
			case model.AuthAPIKey:
				h.click("API key")
				h.typeInto("API key name", "X-Key")
				h.typeInto("API key value", "secret-key")
				h.click("In query")
				want = model.Auth{Type: kind, Key: "X-Key", Value: "secret-key", In: model.APIKeyInQuery}
			}
			if got := requestSent(h, requests).Auth; got != want {
				t.Fatalf("auth = %#v, want %#v", got, want)
			}
		})
	}
}

func TestRequestCancelWhileSending(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	started, canceled := make(chan struct{}), make(chan struct{})
	h.Svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, req model.Request) (model.Response, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		return model.Response{}, ctx.Err()
	}
	h.click("Send")
	h.waitFor("started request", func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	if !h.shows("Cancel") {
		t.Fatal("cancel is missing while sending")
	}
	h.click("Cancel")
	h.waitFor("cancellation", func() bool {
		select {
		case <-canceled:
			return true
		default:
			return false
		}
	})
	if h.App.Active().Sending || h.App.Active().Err != context.Canceled.Error() {
		t.Fatal("request did not cancel")
	}
	if len(h.Svc.History.List()) != 0 {
		t.Fatal("canceled request added to history")
	}
}

func TestRequestBodyModesAndBrowse(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	requests := requestCapture(h)
	h.click("Headers")
	h.click("Add")
	requestType(h, "Key", "X-Test", false)
	requestType(h, "Value", "header-value", false)
	h.click("Body")
	h.click("Raw")
	h.typeInto("Content type", "application/custom")
	h.typeInto("Body content", "raw text")
	if got := h.App.Active().Request.Body; got.ContentType != "application/custom" || got.Text != "raw text" {
		t.Fatalf("raw body = %#v", got)
	}
	h.click("URL encoded")
	h.click("Add")
	requestType(h, "Key", "name", false)
	requestType(h, "Value", "Alice", false)
	if req := requestSent(h, requests); len(req.Headers) != 1 || req.Headers[0].Value != "header-value" || len(req.Body.URLEncoded) != 1 || req.Body.URLEncoded[0].Value != "Alice" {
		t.Fatalf("header/form fields did not reach service: %#v", req)
	}
	h.click("Form data")
	h.click("Add")
	requestType(h, "Key", "upload", false)
	h.click("File")
	if err := h.D.QueueOpenFiles([]string{"C:/fixtures/upload.txt"}, nil); err != nil {
		t.Fatal(err)
	}
	h.click("Browse")
	if got := h.App.Active().Request.Body.Form[0]; !got.File || got.Value != "C:/fixtures/upload.txt" {
		t.Fatalf("form file = %#v", got)
	}
	h.click("Binary")
	if err := h.D.QueueOpenFiles([]string{"C:/fixtures/body.bin"}, nil); err != nil {
		t.Fatal(err)
	}
	h.click("Browse")
	if h.App.Active().Request.Body.FilePath != "C:/fixtures/body.bin" {
		t.Fatal("binary picker did not set path")
	}
	if err := h.D.QueueOpenFiles(nil, errors.New("picker failed")); err != nil {
		t.Fatal(err)
	}
	h.click("Browse")
	if !h.shows("picker failed") {
		t.Fatal("picker error not shown")
	}
}

func TestRequestSectionsPerTabAndLanguages(t *testing.T) {
	h := newHarness(t)
	if !h.shows("发送") || !h.shows("参数") || !h.shows("保存") {
		t.Fatal("Chinese request controls missing")
	}
	requestEnglish(h)
	h.click("Auth")
	first := h.App.Active()
	second := h.App.NewTab()
	requestRepaint(h)
	if h.App.request.tabs[second.ID].section != requestParams {
		t.Fatal("new tab inherited section")
	}
	h.App.active = 0
	requestRepaint(h)
	if h.App.request.tabs[first.ID].section != requestAuth || !h.shows("Bearer") {
		t.Fatal("tab forgot selected section")
	}
	h.App.S = i18n.For(i18n.Zh)
	requestRepaint(h)
	if !h.shows("认证") || !h.shows("基本认证") {
		t.Fatal("Chinese auth controls missing")
	}
}

func TestRequestEnterAndPrimaryEnterSend(t *testing.T) {
	for _, modifiers := range []gfx.Modifiers{0, ui.Primary} {
		t.Run(string(rune('0'+modifiers)), func(t *testing.T) {
			h := newHarness(t)
			requestEnglish(h)
			h.typeInto("URL", "https://example.test/enter")
			if err := h.D.Key(gfx.KeyEnter, modifiers); err != nil {
				t.Fatal(err)
			}
			h.waitFor("keyboard response", func() bool { return h.App.Active().Response != nil })
			if n := len(h.Svc.History.List()); n != 1 {
				t.Fatalf("shortcut sent %d requests", n)
			}
		})
	}
}

func TestRequestMethodAndPrimaryEnterFromBody(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	requests := requestCapture(h)
	h.click("Method")
	h.click("POST")
	if h.App.Active().Request.Method != model.MethodPost || !h.App.Active().Dirty {
		t.Fatal("method selection did not update request")
	}
	h.click("Body")
	h.click("JSON")
	h.typeInto("Body content", `{"from":"shortcut"}`)
	if err := h.D.Key(gfx.KeyEnter, ui.Primary); err != nil {
		t.Fatal(err)
	}
	h.waitFor("body shortcut response", func() bool { return h.App.Active().Response != nil })
	select {
	case req := <-requests:
		if req.Method != model.MethodPost || req.Body.Text != `{"from":"shortcut"}` {
			t.Fatalf("shortcut request = %#v", req)
		}
	default:
		t.Fatal("shortcut did not send")
	}
}

func TestRequestLoadedParamsAndIncompleteQueryEscape(t *testing.T) {
	svc := fake.New()
	h := newHarness(t, svc)
	tab := h.App.NewTab()
	tab.Request.URL = "https://{{host}}/path#fragment"
	tab.Request.Params = []model.KV{{Key: "q", Value: "{{ term }} space"}, {Key: "off", Value: "hidden", Disabled: true}}
	requestEnglish(h)
	if tab.Dirty || tab.Request.URL != "https://{{host}}/path?q={{ term }}+space#fragment" {
		t.Fatalf("loaded request = %#v, dirty %v", tab.Request, tab.Dirty)
	}
	requestType(h, "URL", "https://{{host}}/path?q=%", true)
	if len(tab.Request.Params) != 2 || tab.Request.Params[0].Value != "{{ term }} space" {
		t.Fatal("incomplete escape discarded rows")
	}
	requestType(h, "URL", "https://{{host}}/path?q=%2B", true)
	if tab.Request.Params[0].Value != "+" || !tab.Request.Params[1].Disabled {
		t.Fatalf("query decode = %#v", tab.Request.Params)
	}
}

func BenchmarkRequestPanePaint(b *testing.B) {
	a := New(fake.New())
	a.Active().Request.URL = "https://example.test/search"
	a.Active().Request.Params = []model.KV{{Key: "q", Value: "hello"}, {Key: "limit", Value: "20"}, {Key: "token", Value: "{{token}}"}, {Key: "disabled", Value: "saved", Disabled: true}}
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	for range 3 {
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
