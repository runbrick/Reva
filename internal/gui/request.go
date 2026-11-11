package gui

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"

	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type requestSection uint8

const (
	requestParams requestSection = iota
	requestHeaders
	requestBody
	requestAuth
	requestScripts
)

var requestSections = []requestSection{requestParams, requestHeaders, requestBody, requestAuth, requestScripts}
var requestFileModes = []bool{false, true}
var requestAPIKeyLocations = []model.APIKeyIn{model.APIKeyInHeader, model.APIKeyInQuery}

// requestState keeps section selection and table identities separately per tab.
type requestState struct{ tabs map[int]*requestTabState }

type requestTabState struct {
	section                        requestSection
	params, headers, encoded, form kvTableState
	url                            string
	requestID                      string
}

func (a *App) requestTab(t *Tab) *requestTabState {
	if a.request.tabs == nil {
		a.request.tabs = make(map[int]*requestTabState)
	}
	s := a.request.tabs[t.ID]
	// OpenRequest can replace the request in an existing empty tab. Its query
	// rows need the same initialization as a request opened in a new tab.
	if s == nil || s.requestID != t.Request.ID {
		s = &requestTabState{url: t.Request.URL, requestID: t.Request.ID}
		a.request.tabs[t.ID] = s
		if len(t.Request.Params) == 0 {
			requestParamsFromURL(&t.Request)
		} else {
			requestURLFromParams(&t.Request)
			s.url = t.Request.URL
		}
	}
	// Closing a tab releases its editor state on the next declaration.
	for id := range a.request.tabs {
		if !slices.ContainsFunc(a.tabs, func(tab *Tab) bool { return tab.ID == id }) {
			delete(a.request.tabs, id)
		}
	}
	return s
}

func (a *App) requestPane(c ui.C, t *Tab) {
	s := a.requestTab(t)
	if s.url != t.Request.URL {
		requestParamsFromURL(&t.Request)
		s.url = t.Request.URL
	}
	if s.params.sync(t.Request.Params) {
		t.Dirty = true
		requestURLFromParams(&t.Request)
		s.url = t.Request.URL
	}
	ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Items(ui.Center).Children(func() {
			if ui.Select(c, &t.Request.Method, model.Methods, requestMethodLabel).Label(a.S.Request.Method).Changed() {
				t.Dirty = true
			}
			field := ui.TextField(c, &t.Request.URL, "URL", "https://").Grow(1)
			if field.Changed() {
				t.Dirty = true
				requestParamsFromURL(&t.Request)
				s.url = t.Request.URL
			}
			if field.Submitted() && !t.Sending {
				a.Send(t)
			}
			if t.Sending {
				if ui.Button(c, a.S.Request.Cancel).Clicked() {
					a.Cancel(t)
				}
			} else if ui.Button(c, a.S.Request.Send).Label(a.S.Request.SendAccessible).Clicked() {
				a.Send(t)
			}
			if ui.Button(c, a.S.Request.Save).Clicked() {
				c.After(func() { a.RequestSave(t) })
			}
		})
		ui.Tabs(c, &s.section, requestSections, a.requestSectionLabel).FillWidth()
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				switch s.section {
				case requestParams:
					if a.kvTable(c, &s.params, &t.Request.Params) {
						t.Dirty = true
						requestURLFromParams(&t.Request)
						s.url = t.Request.URL
						c.After(func() {
							t.Dirty = true
							requestURLFromParams(&t.Request)
							s.url = t.Request.URL
						})
					}
				case requestHeaders:
					if a.kvTable(c, &s.headers, &t.Request.Headers) {
						t.Dirty = true
					}
				case requestBody:
					a.requestBodySection(c, t, s)
				case requestAuth:
					a.requestAuthSection(c, t)
				case requestScripts:
					a.scriptsSection(c, t)
				}
			})
		})
	})
	if ui.Shortcut(c, ui.Primary, "enter") && !t.Sending {
		a.Send(t)
	}
	if ui.Shortcut(c, ui.Primary, "s") {
		c.After(func() { a.RequestSave(t) })
	}
}

func requestMethodLabel(m model.Method) string { return string(m) }

func (a *App) requestSectionLabel(section requestSection) string {
	s := &a.S.Request
	switch section {
	case requestParams:
		return s.Params
	case requestHeaders:
		return s.Headers
	case requestBody:
		return s.Body
	case requestAuth:
		return s.Auth
	default:
		return s.Scripts
	}
}

func (a *App) requestBodyModeLabel(mode model.BodyMode) string {
	s := &a.S.Request
	switch mode {
	case model.BodyJSON:
		return "JSON"
	case model.BodyRaw:
		return s.Raw
	case model.BodyFormData:
		return s.FormData
	case model.BodyURLEncoded:
		return s.URLEncoded
	case model.BodyBinary:
		return s.Binary
	default:
		return s.None
	}
}

func (a *App) requestBodySection(c ui.C, t *Tab, s *requestTabState) {
	b := &t.Request.Body
	if ui.Choice(c, &b.Mode, model.BodyModes, a.requestBodyModeLabel).Changed() {
		t.Dirty = true
	}
	switch b.Mode {
	case model.BodyJSON, model.BodyRaw:
		if b.Mode == model.BodyJSON {
			if ui.Button(c, a.S.Request.Format).Clicked() {
				var out bytes.Buffer
				if err := json.Indent(&out, []byte(b.Text), "", "  "); err != nil {
					a.Toast(a.S.Request.InvalidJSON, err.Error(), ui.ToastError)
				} else if text := out.String(); text != b.Text {
					b.Text, t.Dirty = text, true
				}
			}
		} else {
			if ui.TextField(c, &b.ContentType, a.S.Request.ContentType, "text/plain").FillWidth().Changed() {
				t.Dirty = true
			}
		}
		if ui.TextArea(c, &b.Text, a.S.Request.BodyText, "").FillWidth().Height(144).Changed() {
			t.Dirty = true
		}
	case model.BodyURLEncoded:
		if a.kvTable(c, &s.encoded, &b.URLEncoded) {
			t.Dirty = true
		}
	case model.BodyFormData:
		a.requestFormSection(c, t, &s.form)
	case model.BodyBinary:
		a.requestFileField(c, t, &b.FilePath)
	}
}

func (a *App) requestFileModeLabel(file bool) string {
	if file {
		return a.S.Request.File
	}
	return a.S.Request.Text
}

func (a *App) requestFormSection(c ui.C, t *Tab, s *kvTableState) {
	rows := &t.Request.Body.Form
	s.resize(len(*rows))
	for i := range *rows {
		r, state := &(*rows)[i], s.rows[i]
		state.enabled = !r.Disabled
		ui.Row(c.KeyInt(state.id)).FillWidth().Gap(8).Items(ui.Center).Children(func() {
			if ui.Checkbox(c, &state.enabled, "").Label(a.S.Request.Enabled).Changed() {
				r.Disabled, t.Dirty = !state.enabled, true
			}
			if ui.TextField(c, &r.Key, a.S.Common.Key, a.S.Common.Key).Grow(1).Changed() {
				t.Dirty = true
			}
			if ui.Choice(c, &r.File, requestFileModes, a.requestFileModeLabel).Changed() {
				t.Dirty = true
			}
			if r.File {
				a.requestFileField(c, t, &r.Value)
			} else if ui.TextField(c, &r.Value, a.S.Common.Value, a.S.Common.Value).Grow(1).Changed() {
				t.Dirty = true
			}
			if ui.Button(c, a.S.Common.Delete).Clicked() {
				c.After(func() {
					*rows = slices.Delete(*rows, i, i+1)
					s.rows = slices.Delete(s.rows, i, i+1)
					t.Dirty = true
				})
			}
		})
	}
	if ui.Button(c, a.S.Common.Add).Clicked() {
		c.After(func() { *rows = append(*rows, model.FormField{}); t.Dirty = true })
	}
}

func (a *App) requestFileField(c ui.C, t *Tab, path *string) {
	ui.Row(c).FillWidth().Grow(1).Gap(8).Children(func() {
		if ui.TextField(c, path, a.S.Request.FilePath, "").Grow(1).Changed() {
			t.Dirty = true
		}
		if ui.Button(c, a.S.Request.Browse).Clicked() {
			c.After(func() {
				paths, err := app.OpenFiles(app.OpenFileOptions{Title: a.S.Request.Browse})
				if err != nil && !errors.Is(err, app.ErrCanceled) {
					a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
				} else if len(paths) > 0 && paths[0] != *path {
					*path, t.Dirty = paths[0], true
				}
			})
		}
	})
}

func (a *App) requestAuthTypeLabel(kind model.AuthType) string {
	s := &a.S.Request
	switch kind {
	case model.AuthBearer:
		return "Bearer"
	case model.AuthBasic:
		return s.Basic
	case model.AuthAPIKey:
		return s.APIKey
	default:
		return s.None
	}
}

func (a *App) requestAPIKeyInLabel(in model.APIKeyIn) string {
	if in == model.APIKeyInQuery {
		return a.S.Request.InQuery
	}
	return a.S.Request.InHeader
}

func (a *App) requestAuthSection(c ui.C, t *Tab) {
	auth := &t.Request.Auth
	s := &a.S.Request
	if ui.Choice(c, &auth.Type, model.AuthTypes, a.requestAuthTypeLabel).Changed() {
		t.Dirty = true
		if auth.Type == model.AuthAPIKey && auth.In == "" {
			auth.In = model.APIKeyInHeader
		}
	}
	switch auth.Type {
	case model.AuthBearer:
		if ui.TextField(c, &auth.Token, s.Token, "").FillWidth().Changed() {
			t.Dirty = true
		}
	case model.AuthBasic:
		if ui.TextField(c, &auth.Username, s.Username, "").FillWidth().Changed() {
			t.Dirty = true
		}
		if ui.TextField(c, &auth.Password, s.Password, "").Secure().FillWidth().Changed() {
			t.Dirty = true
		}
	case model.AuthAPIKey:
		if ui.TextField(c, &auth.Key, s.APIKeyName, "").FillWidth().Changed() {
			t.Dirty = true
		}
		if ui.TextField(c, &auth.Value, s.APIKeyValue, "").FillWidth().Changed() {
			t.Dirty = true
		}
		if ui.Choice(c, &auth.In, requestAPIKeyLocations, a.requestAPIKeyInLabel).Label(s.APIKeyLocation).Changed() {
			t.Dirty = true
		}
	}
}

// Split the raw URL before parsing its query: url.Parse rejects variable hosts
// such as https://{{host}}. The path and fragment retain their exact spelling.
func requestURLParts(raw string) (base, query, fragment string) {
	base, fragment, hasFragment := strings.Cut(raw, "#")
	if hasFragment {
		fragment = "#" + fragment
	}
	base, query, _ = strings.Cut(base, "?")
	return base, query, fragment
}

func requestParamsFromURL(req *model.Request) {
	_, query, _ := requestURLParts(req.URL)
	var rows []model.KV
	for part := range strings.SplitSeq(query, "&") {
		if part == "" {
			continue
		}
		key, value, _ := strings.Cut(part, "=")
		key, err := url.QueryUnescape(key)
		if err != nil {
			return
		} // Keep rows while an escape is still being typed.
		value, err = url.QueryUnescape(value)
		if err != nil {
			return
		}
		rows = append(rows, model.KV{Key: key, Value: value})
	}
	for _, row := range req.Params {
		if row.Disabled || row.Key == "" {
			rows = append(rows, row)
		}
	}
	req.Params = rows
}

func requestQueryEscape(value string) string {
	// Escape literal text, but leave whole variable tokens byte-for-byte intact.
	var out strings.Builder
	for {
		start := strings.Index(value, "{{")
		if start < 0 {
			out.WriteString(url.QueryEscape(value))
			break
		}
		end := strings.Index(value[start+2:], "}}")
		if end < 0 {
			out.WriteString(url.QueryEscape(value))
			break
		}
		end += start + 4
		out.WriteString(url.QueryEscape(value[:start]))
		out.WriteString(value[start:end])
		value = value[end:]
	}
	return out.String()
}

func requestURLFromParams(req *model.Request) {
	base, _, fragment := requestURLParts(req.URL)
	var query strings.Builder
	for _, row := range req.Params {
		if row.Disabled || row.Key == "" {
			continue
		}
		if query.Len() > 0 {
			query.WriteByte('&')
		}
		query.WriteString(requestQueryEscape(row.Key))
		query.WriteByte('=')
		query.WriteString(requestQueryEscape(row.Value))
	}
	if query.Len() > 0 {
		base += "?" + query.String()
	}
	req.URL = base + fragment
}
