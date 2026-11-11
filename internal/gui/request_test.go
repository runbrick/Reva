package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestRequestOpenIntoExistingTabPreservesParams(t *testing.T) {
	h := newHarness(t)
	original := h.App.Active()
	req := model.NewRequest("imported-query")
	req.URL = "https://example.test/users"
	req.Params = []model.KV{{Key: "q", Value: "active"}}
	h.App.OpenRequest(req, "")
	requestRepaint(h)
	if h.App.Active() != original || len(h.App.Tabs()) != 1 {
		t.Fatal("request did not reuse the existing empty tab")
	}
	if !reflect.DeepEqual(original.Request.Params, req.Params) {
		t.Fatalf("opening request discarded params: %#v", original.Request.Params)
	}
	if original.Request.URL != "https://example.test/users?q=active" {
		t.Fatalf("opening request did not rebuild query: %q", original.Request.URL)
	}
	if original.Dirty {
		t.Fatal("opening request marked it dirty")
	}
}

func TestRequestParamDescriptionNeverEntersURL(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	h.typeInto("URL", "https://example.test/search?q=coffee")
	tab := h.App.Active()
	tab.Dirty = false
	requestType(h, "Description", "Product category", false)
	if !tab.Dirty || tab.Request.Params[0].Description != "Product category" {
		t.Fatalf("description edit = %#v, dirty %v", tab.Request.Params, tab.Dirty)
	}
	if tab.Request.URL != "https://example.test/search?q=coffee" {
		t.Fatalf("description leaked into URL: %s", tab.Request.URL)
	}
	requestType(h, "URL", "https://example.test/search?q=tea", true)
	if tab.Request.Params[0].Description != "Product category" {
		t.Fatal("editing query value discarded its description")
	}
}

func TestRequestBulkEditRoundTrip(t *testing.T) {
	for _, headers := range []bool{false, true} {
		t.Run(map[bool]string{false: "params", true: "headers"}[headers], func(t *testing.T) {
			h := newHarness(t)
			tab := h.App.Active()
			rows := &tab.Request.Params
			if headers {
				rows = &tab.Request.Headers
			}
			*rows = []model.KV{{Key: "first", Value: "one", Description: "retained note"}, {Key: "disabled", Value: "two: words", Disabled: true}}
			requestEnglish(h)
			if headers {
				h.click("Headers")
			}
			h.click("Bulk edit")
			if got := requestField(h, "Bulk parameters", 0).Value; got != "first: one\n// disabled: two: words" {
				t.Fatalf("bulk text = %q", got)
			}
			tab.Dirty = false
			requestType(h, "Bulk parameters", "first: updated\n// disabled: two: words\nthird: three", true)
			h.click("Table edit")
			want := []model.KV{{Key: "first", Value: "updated", Description: "retained note"}, {Key: "disabled", Value: "two: words", Disabled: true}, {Key: "third", Value: "three"}}
			if !reflect.DeepEqual(*rows, want) || !tab.Dirty {
				t.Fatalf("bulk round trip = %#v, dirty %v", *rows, tab.Dirty)
			}
			if !headers && (strings.Contains(tab.Request.URL, "disabled") || !strings.Contains(tab.Request.URL, "first=updated&third=three")) {
				t.Fatalf("bulk URL = %s", tab.Request.URL)
			}
			requestType(h, "Description", "updated note", true)
			if (*rows)[0].Description != "updated note" {
				t.Fatal("table not editable after bulk edit")
			}
		})
	}
}

func TestRequestSendMenuCopyResolvedCURL(t *testing.T) {
	h := newHarness(t)
	col, err := h.Svc.Collections.Create("cURL variables")
	if err != nil {
		t.Fatal(err)
	}
	col.Variables = []model.KV{{Key: "host", Value: "example.test"}, {Key: "trace", Value: "header-value"}}
	if err := h.Svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	tab := h.App.Active()
	tab.CollectionID = col.ID
	tab.Request.Method = model.MethodPost
	tab.Request.URL = "https://{{host}}/items"
	tab.Request.Headers = []model.KV{{Key: "X-Test", Value: "{{trace}}"}, {Key: "X-Disabled", Value: "hidden", Disabled: true}}
	tab.Request.Body = model.Body{Mode: model.BodyJSON, Text: `{"message":"it's ready"}`}
	tab.Request.Auth = model.Auth{Type: model.AuthBearer, Token: "secret"}
	requestEnglish(h)
	h.click("Send options")
	h.click("Copy as cURL")
	command := h.D.Clipboard()
	for _, part := range []string{"curl -X 'POST'", "'https://example.test/items'", "'X-Test: header-value'", "'Content-Type: application/json'", "'Authorization: Bearer secret'", "--data-raw", `it'"'"'s ready`} {
		if !strings.Contains(command, part) {
			t.Fatalf("command %q missing %q", command, part)
		}
	}
	if strings.Contains(command, "X-Disabled") || tab.Sending {
		t.Fatalf("copy sent request or included disabled row: %s", command)
	}
	if tab.Request.Headers[0].Value != "{{trace}}" || tab.Request.URL != "https://{{host}}/items" {
		t.Fatal("copy mutated the stored request")
	}
}

func TestRequestSendMenuSaveOpensCollectionsDialog(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	h.click("Send options")
	h.click("Save")
	if !h.App.collections.saveOpen || !h.shows(h.App.S.Collections.SaveRequest) {
		t.Fatal("Send menu Save did not open collections dialog")
	}
}

func TestRequestSendAndDownloadSavesCompletedBody(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	path := filepath.Join(t.TempDir(), "response.bin")
	if err := h.D.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	body := []byte{0, 1, 2, 255}
	h.Svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		return model.Response{StatusCode: 200, Body: body}, nil
	}
	h.click("Send options")
	h.click("Send and download")
	h.waitFor("downloaded response", func() bool { data, err := os.ReadFile(path); return err == nil && reflect.DeepEqual(data, body) })
}

func TestRequestDownloadCancelAndFailures(t *testing.T) {
	for _, scenario := range []string{"cancel", "picker error", "write error"} {
		t.Run(scenario, func(t *testing.T) {
			h := newHarness(t)
			requestEnglish(h)
			path := filepath.Join(t.TempDir(), "missing", "response.bin")
			var pickerError error
			switch scenario {
			case "cancel":
				path, pickerError = "", app.ErrCanceled
			case "picker error":
				path, pickerError = "", errors.New("picker failed")
			}
			if err := h.D.QueueSaveFile(path, pickerError); err != nil {
				t.Fatal(err)
			}
			h.click("Send options")
			h.click("Send and download")
			if scenario != "write error" {
				if h.App.Active().Sending || len(h.Svc.History.List()) > 0 {
					t.Fatal("dismissed picker sent a request")
				}
				if scenario == "picker error" && !h.shows("picker failed") {
					t.Fatal("picker failure not shown")
				}
				return
			}
			h.waitFor("download write failure", func() bool { return h.shows(h.App.S.Common.Error) })
		})
	}
}

func requestClickNth(h *harness, label string, index int) {
	h.t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label != label {
			continue
		}
		if index > 0 {
			index--
			continue
		}
		if err := h.D.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
			h.t.Fatal(err)
		}
		return
	}
	h.t.Fatalf("missing control %s", label)
}

func TestRequestOptionsStoreOverridesAndInherit(t *testing.T) {
	h := newHarness(t)
	settings := h.Svc.Settings.Get()
	settings.RequestTimeoutSec = 12
	if err := h.Svc.Settings.Save(settings); err != nil {
		t.Fatal(err)
	}
	requestEnglish(h)
	h.click("Request settings")
	if !h.shows("12000") {
		t.Fatal("inherited timeout missing")
	}
	tab := h.App.Active()
	tab.Dirty = false
	h.typeInto("Timeout (ms)", "2500")
	if tab.Request.Options.TimeoutMs != 2500 || !tab.Dirty {
		t.Fatalf("timeout = %#v, dirty %v", tab.Request.Options, tab.Dirty)
	}
	tab.Dirty = false
	requestClickNth(h, "Off", 0)
	if tab.Request.Options.FollowRedirects != model.ToggleOff || !tab.Dirty {
		t.Fatal("redirect override did not persist")
	}
	tab.Dirty = false
	requestClickNth(h, "On", 1)
	if tab.Request.Options.VerifyTLS != model.ToggleOn || !tab.Dirty {
		t.Fatal("TLS override did not persist")
	}
	requestClickNth(h, "Inherit", 0)
	requestClickNth(h, "Inherit", 1)
	requestType(h, "Timeout (ms)", "", true)
	if err := h.D.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	if tab.Request.Options != (model.RequestOptions{}) {
		t.Fatalf("inherit options = %#v", tab.Request.Options)
	}
	requestType(h, "Timeout (ms)", "invalid", true)
	if tab.Request.Options.TimeoutMs != 0 {
		t.Fatal("invalid timeout overwrote inherited value")
	}
}

func TestRequestEnabledHeaderCount(t *testing.T) {
	h := newHarness(t)
	h.App.Active().Request.Headers = []model.KV{{Key: "Accept", Value: "application/json"}, {Key: "X-Trace", Value: "1"}, {Key: "X-Off", Disabled: true}}
	requestEnglish(h)
	assertHeaderCount := func(want string) {
		t.Helper()
		for _, hit := range h.D.Snapshot().Hits {
			if hit.Role == gfx.RoleTab && hit.Label == want {
				return
			}
		}
		t.Fatalf("header badge %q missing; have %q", want, h.hitLabels())
	}
	assertHeaderCount("Headers 2")
	h.click("Headers")
	requestClickNth(h, h.App.S.Common.Key, 0)
	assertHeaderCount("Headers 1")
}

func TestRequestEditorBothLanguagesAndThemes(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
			t.Run(theme+"/"+string(lang), func(t *testing.T) {
				h := newHarness(t)
				h.App.ApplySettings(model.Settings{Theme: theme, Language: string(lang)})
				requestRepaint(h)
				s := h.App.S.Request
				for _, text := range []string{s.Params, s.Headers, s.Body, s.Settings, s.ParamName, s.Description, s.AddParam} {
					if !h.shows(text) {
						t.Fatalf("missing %s", text)
					}
				}
				h.click(s.SendMenu)
				if !h.shows(s.SendDownload) || !h.shows(s.CopyCURL) || !h.shows(s.Save) {
					t.Fatal("localized menu missing")
				}
				if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
				h.click(s.Settings)
				if !h.shows(s.Timeout) || !h.shows(s.FollowRedirects) || !h.shows(s.VerifyTLS) {
					t.Fatal("localized options missing")
				}
			})
		}
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
