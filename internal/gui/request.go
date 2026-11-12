package gui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/vars"
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
	requestSettings
)

var requestSections = []requestSection{requestParams, requestHeaders, requestBody, requestAuth, requestScripts, requestSettings}
var requestFileModes = []bool{false, true}
var requestAPIKeyLocations = []model.APIKeyIn{model.APIKeyInHeader, model.APIKeyInQuery}

// requestState keeps section selection and table identities separately per tab.
type requestState struct {
	tabs map[int]*requestTabState
}

type requestTabState struct {
	section                        requestSection
	params, headers, encoded, form kvTableState
	url                            string
	requestID                      string
	menu                           bool
	downloadJob                    *sendJob
	downloadPath                   string
	writeJob                       *requestDownload
	write                          *ui.EffectState[struct{}, *requestDownload]
	timeout                        string
	timeoutValue                   int
	captions                       [requestSettings + 1]requestSectionCaption
}

type requestSectionCaption struct {
	label, accessible, countLabel string
	count                         int
}

type requestDownload struct {
	path string
	body []byte
}

func (a *App) requestTab(t *Tab) *requestTabState {
	if a.request.tabs == nil {
		a.request.tabs = make(map[int]*requestTabState)
	}
	s := a.request.tabs[t.ID]
	// OpenRequest can replace the request in an existing empty tab. Its query
	// rows need the same initialization as a request opened in a new tab.
	if s == nil || s.requestID != t.Request.ID {
		s = &requestTabState{url: t.Request.URL, requestID: t.Request.ID, timeoutValue: t.Request.Options.TimeoutMs}
		if s.timeoutValue > 0 {
			s.timeout = strconv.Itoa(s.timeoutValue)
		}
		s.headers.headers = true
		s.write = ui.NewEffectKeyed(func(_ context.Context, job *requestDownload) (struct{}, error) {
			return struct{}{}, os.WriteFile(job.path, job.body, 0600)
		})
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
	a.requestDownloads(c)
	if s.url != t.Request.URL {
		requestParamsFromURL(&t.Request)
		s.url = t.Request.URL
	}
	bulkChanged := s.params.syncBulk(&t.Request.Params)
	if s.params.sync(t.Request.Params) || bulkChanged {
		t.Dirty = true
		requestURLFromParams(&t.Request)
		s.url = t.Request.URL
	}
	ui.Column(c).Fill().Pad(Space4).Gap(Space2).Children(func() {
		a.requestURLBar(c, t, s)
		a.requestSectionTabs(c, t, s)
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(Space2).Children(func() {
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
				case requestSettings:
					a.requestSettingsSection(c, t, s)
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
	case requestSettings:
		return s.Settings
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
	theme := c.Theme()
	ui.Row(c).FillWidth().Gap(Space1).FocusGroup(ui.FocusHorizontal).Children(func() {
		for _, mode := range model.BodyModes {
			pill := ui.RadioBase(c.Key(string(mode)), &b.Mode, mode, a.requestBodyModeLabel(mode)).
				PadX(Space3).Height(theme.Metrics.ControlHeight).Radius(theme.Metrics.ControlHeight / 2).
				Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring))
			if pill.Changed() {
				t.Dirty = true
			}
			fg, bg := theme.MutedForeground, theme.Background
			if b.Mode == mode {
				fg, bg = theme.Primary, theme.Accent
			}
			pill.Background(bg).Children(func() { ui.Text(c, a.requestBodyModeLabel(mode)).Color(fg).FontSize(theme.Metrics.FontSizeS) })
		}
	})
	switch b.Mode {
	case model.BodyJSON, model.BodyRaw:
		if b.Mode == model.BodyJSON {
			if ui.Button(c, a.S.Request.Format).Icon(icons.Braces).Ghost().Small().Clicked() {
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
		if ui.TextArea(c, &b.Text, a.S.Request.BodyText, "").Mono().FillWidth().Height(Space6 * 6).Changed() {
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
		ui.Row(c.KeyInt(state.id)).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
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
	ui.Row(c).FillWidth().Grow(1).Gap(Space2).Children(func() {
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
	used := make([]bool, len(req.Params))
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
		row := model.KV{Key: key, Value: value}
		for i, old := range req.Params {
			if !used[i] && !old.Disabled && old.Key == key {
				row.Description, used[i] = old.Description, true
				break
			}
		}
		rows = append(rows, row)
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

func (a *App) requestURLBar(c ui.C, t *Tab, s *requestTabState) {
	theme, labels := c.Theme(), &a.S.Request
	height := theme.Metrics.ControlHeight + Space2
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		ui.Row(c).Grow(1).Height(height).Items(ui.Center).Border(1, theme.Input).
			Radius(theme.Metrics.RadiusControl).Background(theme.Background).Children(func() {
			method := ui.Select(c, &t.Request.Method, model.Methods, requestMethodLabel).Label(labels.Method).
				Width(Space6*4 + Space4).Height(height).Radius(0)
			if method.Changed() {
				t.Dirty = true
			}
			ui.Divider(c).Vertical()
			field := ui.TextField(c, &t.Request.URL, labels.URL, "https://").Grow(1).Height(height-Space1).
				Border(0, theme.Border).Radius(0).Background(theme.Background).PadX(Space3)
			if field.Changed() {
				t.Dirty = true
				requestParamsFromURL(&t.Request)
				s.url = t.Request.URL
			}
			if field.Submitted() && !t.Sending {
				a.Send(t)
			}
		})
		ui.Row(c).Height(height).Items(ui.Center).Radius(theme.Metrics.RadiusControl).Background(theme.Primary).Children(func() {
			if t.Sending {
				if ui.Button(c, labels.Cancel).Icon(icons.X).Height(height).Clicked() {
					a.Cancel(t)
				}
			} else if ui.Button(c, labels.Send).Label(labels.SendAccessible).Icon(icons.Send).Height(height).Clicked() {
				a.Send(t)
			}
			ui.Divider(c).Vertical().Height(Space6).Background(theme.PrimaryForeground.WithAlpha(0.3))
			more := ui.ButtonBase(c, labels.SendMenu).Size(theme.Metrics.ControlHeight, height).
				Radius(theme.Metrics.RadiusControl).Hover(ui.Look{}.Background(theme.Primary.WithAlpha(0.8))).
				Focus(ui.Look{}.Ring(2, theme.Ring)).Children(func() {
				ui.Icon(c, icons.ChevronDown).Size(Space4, Space4).Color(theme.PrimaryForeground)
			})
			if more.Clicked() {
				s.menu = !s.menu
			}
			ui.Menu(c, more, &s.menu, func(menu *ui.MenuItems) {
				if menu.Item(labels.SendDownload).Icon(icons.Download).Disabled(t.Sending).Clicked() {
					c.After(func() {
						path, err := app.SaveFile(app.SaveFileOptions{Title: labels.SendDownload, SuggestedName: labels.DownloadName})
						if errors.Is(err, app.ErrCanceled) {
							return
						}
						if err != nil {
							a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
							return
						}
						if path == "" {
							return
						}
						a.Send(t)
						s.downloadJob, s.downloadPath = t.job, path
					})
				}
				if menu.Item(labels.CopyCURL).Icon(icons.Copy).Clicked() {
					command := requestCURL(vars.Apply(t.Request, a.Vars()))
					c.After(func() {
						if err := app.SetClipboardText(command); err != nil {
							a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
						}
					})
				}
				menu.Separator()
				if menu.Item(labels.Save).Icon(icons.Save).Clicked() {
					c.After(func() { a.RequestSave(t) })
				}
			})
		})
	})
}

func (a *App) requestSectionTabs(c ui.C, t *Tab, s *requestTabState) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Gap(Space2).FocusGroup(ui.FocusHorizontal).Children(func() {
		for _, section := range requestSections {
			count := 0
			var rows []model.KV
			switch section {
			case requestParams:
				rows = t.Request.Params
			case requestHeaders:
				rows = t.Request.Headers
			case requestBody:
				if t.Request.Body.Mode == model.BodyURLEncoded {
					rows = t.Request.Body.URLEncoded
				}
				if t.Request.Body.Mode == model.BodyFormData {
					for _, row := range t.Request.Body.Form {
						if !row.Disabled && row.Key != "" {
							count++
						}
					}
				}
			}
			for _, row := range rows {
				if !row.Disabled && row.Key != "" {
					count++
				}
			}
			label := a.requestSectionLabel(section)
			caption := &s.captions[section]
			if caption.label != label || caption.count != count {
				caption.label, caption.count, caption.accessible = label, count, label
				if count > 0 {
					caption.countLabel = strconv.Itoa(count)
					caption.accessible += " " + caption.countLabel
				}
			}
			tab := ui.TabBase(c.KeyInt(int(section)), &s.section, section, caption.accessible).
				Height(theme.Metrics.ControlHeight + Space2).PadX(Space2).Gap(Space1).
				Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring))
			fg := theme.MutedForeground
			if s.section == section {
				fg = theme.Primary
			}
			tab.Children(func() {
				ui.Text(c, label).Color(fg).FontSize(theme.Metrics.FontSizeM)
				if count > 0 {
					ui.Text(c, caption.countLabel).Color(fg).Background(theme.Accent).FontSize(theme.Metrics.FontSizeS).
						PadX(Space1).Radius(theme.Metrics.RadiusControl)
				}
				if s.section == section {
					ui.Row(c).Absolute().Left(0).Right(0).Bottom(0).Height(Space1 / 2).Background(theme.Primary)
				}
			})
		}
	})
	ui.Divider(c)
}

var requestToggles = []model.Toggle{model.ToggleInherit, model.ToggleOn, model.ToggleOff}

func (a *App) requestToggleLabel(toggle model.Toggle) string {
	switch toggle {
	case model.ToggleOn:
		return a.S.Request.On
	case model.ToggleOff:
		return a.S.Request.Off
	default:
		return a.S.Request.Inherit
	}
}

func (a *App) requestSettingsSection(c ui.C, t *Tab, s *requestTabState) {
	labels, options, theme := &a.S.Request, &t.Request.Options, c.Theme()
	if options.TimeoutMs != s.timeoutValue {
		s.timeoutValue = options.TimeoutMs
		s.timeout = ""
		if s.timeoutValue > 0 {
			s.timeout = strconv.Itoa(s.timeoutValue)
		}
	}
	inherited := (model.RequestOptions{}).Resolve(a.Svc.Settings.Get())
	ui.Text(c, labels.Timeout).Bold()
	if ui.TextField(c, &s.timeout, labels.Timeout, strconv.Itoa(inherited.TimeoutMs)).FillWidth().Changed() {
		text := strings.TrimSpace(s.timeout)
		value, err := strconv.Atoi(text)
		if text == "" || err == nil && value > 0 {
			if text == "" {
				value = 0
			}
			options.TimeoutMs, s.timeoutValue, t.Dirty = value, value, true
		}
	}
	ui.Text(c, labels.TimeoutHelp).Muted().FontSize(theme.Metrics.FontSizeS)
	ui.Row(c).FillWidth().Gap(Space4).Items(ui.Center).Children(func() {
		ui.Text(c, labels.FollowRedirects).Grow(1)
		if ui.Choice(c, &options.FollowRedirects, requestToggles, a.requestToggleLabel).Label(labels.FollowRedirects).Changed() {
			t.Dirty = true
		}
	})
	ui.Row(c).FillWidth().Gap(Space4).Items(ui.Center).Children(func() {
		ui.Text(c, labels.VerifyTLS).Grow(1)
		if ui.Choice(c, &options.VerifyTLS, requestToggles, a.requestToggleLabel).Label(labels.VerifyTLS).Changed() {
			t.Dirty = true
		}
	})
}

// This is declared with the editor, and uses completed sends from the shell.
// Visiting another request tab still finishes any pending download.
func (a *App) requestDownloads(c ui.C) {
	for _, tab := range a.tabs {
		state := a.request.tabs[tab.ID]
		if state == nil {
			continue
		}
		if state.downloadJob != nil {
			if tab.job != state.downloadJob {
				state.downloadJob = nil
			} else if !tab.Sending {
				state.downloadJob = nil
				if tab.Response != nil {
					state.writeJob = &requestDownload{path: state.downloadPath, body: tab.Response.Body}
				}
			}
		}
		if state.writeJob != nil {
			ui.Effect(c, state.write, state.writeJob)
			if _, err, ok := state.write.Take(); ok {
				state.writeJob = nil
				if err != nil {
					a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
				}
			}
		}
	}
}

// requestCURL receives an already resolved copy, leaving the stored request
// untouched. Arguments use POSIX shell quoting, including embedded apostrophes.
func requestCURL(req model.Request) string {
	if len(req.Params) == 0 {
		requestParamsFromURL(&req)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	var out strings.Builder
	out.WriteString("curl -X ")
	out.WriteString(quote(string(req.Method)))
	header := func(key, value string) { out.WriteString(" -H "); out.WriteString(quote(key + ": " + value)) }
	hasContentType := false
	for _, row := range req.Headers {
		if !row.Disabled && row.Key != "" {
			header(row.Key, row.Value)
			hasContentType = hasContentType || strings.EqualFold(row.Key, "Content-Type")
		}
	}
	switch req.Auth.Type {
	case model.AuthBearer:
		header("Authorization", "Bearer "+req.Auth.Token)
	case model.AuthBasic:
		header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(req.Auth.Username+":"+req.Auth.Password)))
	case model.AuthAPIKey:
		if req.Auth.In == model.APIKeyInQuery {
			req.Params = append(req.Params, model.KV{Key: req.Auth.Key, Value: req.Auth.Value})
		} else {
			header(req.Auth.Key, req.Auth.Value)
		}
	}
	if len(req.Params) > 0 {
		requestURLFromParams(&req)
	}
	out.WriteByte(' ')
	out.WriteString(quote(req.URL))
	body := req.Body
	data := func(flag, value string) {
		out.WriteByte(' ')
		out.WriteString(flag)
		out.WriteByte(' ')
		out.WriteString(quote(value))
	}
	switch body.Mode {
	case model.BodyJSON, model.BodyRaw:
		if !hasContentType {
			contentType := body.ContentType
			if body.Mode == model.BodyJSON {
				contentType = "application/json"
			} else if contentType == "" {
				contentType = "text/plain"
			}
			header("Content-Type", contentType)
		}
		data("--data-raw", body.Text)
	case model.BodyBinary:
		if !hasContentType {
			header("Content-Type", "application/octet-stream")
		}
		data("--data-binary", "@"+body.FilePath)
	case model.BodyURLEncoded:
		values := make(url.Values)
		for _, row := range body.URLEncoded {
			if !row.Disabled && row.Key != "" {
				values.Add(row.Key, row.Value)
			}
		}
		if !hasContentType {
			header("Content-Type", "application/x-www-form-urlencoded")
		}
		data("--data-raw", values.Encode())
	case model.BodyFormData:
		for _, row := range body.Form {
			if row.Disabled || row.Key == "" {
				continue
			}
			if row.File {
				data("--form", row.Key+"=@"+row.Value)
			} else {
				data("--form-string", row.Key+"="+row.Value)
			}
		}
	}
	return out.String()
}
