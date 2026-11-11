package gui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/runbrick/reqo/internal/format"
	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

const responseDisplayLimit = 2 * 1024 * 1024

type responseSection int

const (
	responseBody responseSection = iota
	responseHeaders
	responseTests
)

// responseView keeps each request tab's selection and most recent immutable
// response. Repainting, switching sections or visiting another tab reuses it.
type responseView struct {
	section                     responseSection
	rawMode                     bool
	source                      *model.Response
	raw, pretty, duration, size string
	rawLimited, prettyLimited   bool
	testsCount                  int
	testsPattern, testsLabel    string
}

// responseState is the response viewer's UI state. Owned by the response PR.
type responseState struct {
	views map[int]*responseView
}

// prepare runs only when the shell installs a new response pointer.
func (v *responseView) prepare(r *model.Response) {
	if v.source == r {
		return
	}
	v.source = r
	body := r.Body
	v.rawLimited = len(body) > responseDisplayLimit
	if v.rawLimited {
		end := responseDisplayLimit
		for end > 0 && !utf8.RuneStart(body[end]) {
			end--
		}
		body = body[:end]
	}
	v.raw = string(body)
	v.pretty, v.prettyLimited = v.raw, v.rawLimited
	// A partial body cannot be parsed reliably. Avoid formatting bodies above
	// the display limit and preserve their raw prefix instead.
	if !v.rawLimited && format.IsJSON(r.Header("Content-Type"), r.Body) {
		if pretty, err := format.PrettyJSON(r.Body); err == nil {
			v.pretty, v.prettyLimited = responseDisplayText(pretty)
		}
	}
	v.duration, v.size = format.Duration(r.Duration), format.ByteSize(r.Size)
}

func responseDisplayText(value string) (string, bool) {
	if len(value) <= responseDisplayLimit {
		return value, false
	}
	end := responseDisplayLimit
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return strings.Clone(value[:end]), true
}

// responsePane declares the response of t: status, duration, size, headers
// and body (pretty / raw).
func (a *App) responsePane(c ui.C, t *Tab) {
	// Also prune while the active tab has no response (for example, after
	// closing a large response and returning to a new request).
	for id := range a.response.views {
		open := false
		for _, tab := range a.tabs {
			if tab.ID == id {
				open = true
				break
			}
		}
		if !open {
			delete(a.response.views, id)
		}
	}
	ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
		ui.Text(c, a.S.Response.Title).Bold()
		switch {
		case t.Sending:
			ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, a.S.Response.Sending).Muted()
			})
		case t.Err != "":
			ui.Text(c, a.S.Response.ErrorTitle).Bold().Color(c.Theme().Danger)
			ui.Scroll(c).FillWidth().Grow(1).Children(func() {
				ui.Paragraph(c, t.Err, 0).Color(c.Theme().Danger)
			})
		case t.Response != nil:
			a.responseResult(c, t)
		default:
			ui.Paragraph(c, a.S.Response.EmptyHint, 0).Muted()
		}
	})
}

func (a *App) responseResult(c ui.C, t *Tab) {
	if a.response.views == nil {
		a.response.views = make(map[int]*responseView)
	}
	v := a.response.views[t.ID]
	if v == nil {
		v = &responseView{}
		a.response.views[t.ID] = v
	}
	v.prepare(t.Response)
	if v.testsPattern != a.S.Response.TestsCount || v.testsCount != len(t.Tests) {
		v.testsPattern, v.testsCount = a.S.Response.TestsCount, len(t.Tests)
		v.testsLabel = fmt.Sprintf(v.testsPattern, v.testsCount)
	}
	theme := c.Theme()
	statusColor := theme.Foreground
	switch t.Response.StatusCode / 100 {
	case 2:
		statusColor = theme.Success
	case 4, 5:
		statusColor = theme.Danger
	}
	ui.Flow(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, t.Response.Status).Bold().Color(statusColor)
		ui.Text(c, a.S.Response.Duration).Muted()
		ui.Text(c, v.duration)
		ui.Text(c, a.S.Response.Size).Muted()
		ui.Text(c, v.size)
	})
	if t.Response.Truncated {
		ui.Paragraph(c, a.S.Response.Truncated, 0).Color(theme.Danger)
	}
	ui.Tabs(c, &v.section, []responseSection{responseBody, responseHeaders, responseTests}, func(section responseSection) string {
		switch section {
		case responseHeaders:
			return a.S.Response.Headers
		case responseTests:
			if len(t.Tests) > 0 {
				return v.testsLabel
			}
			return a.S.Response.Tests
		default:
			return a.S.Response.Body
		}
	}).FillWidth()
	switch v.section {
	case responseHeaders:
		a.responseHeaderTable(c, t.Response)
	case responseTests:
		a.testsSection(c, t)
	default:
		ui.Row(c).FillWidth().Gap(8).Items(ui.Center).Children(func() {
			ui.Choice(c, &v.rawMode, []bool{false, true}, func(raw bool) string {
				if raw {
					return a.S.Response.Raw
				}
				return a.S.Response.Pretty
			})
			ui.Spacer(c)
			if ui.Button(c, a.S.Response.Copy).Clicked() {
				// Native clipboard access may block; run it after declaration.
				c.After(func() {
					if err := app.SetClipboardText(string(t.Response.Body)); err != nil {
						a.Toast(a.S.Response.CopyError, err.Error(), ui.ToastError)
					}
				})
			}
		})
		body, limited := v.pretty, v.prettyLimited
		if v.rawMode {
			body, limited = v.raw, v.rawLimited
		}
		if limited {
			ui.Paragraph(c, a.S.Response.DisplayLimit, 0).Muted()
		}
		// Viva v0.6.0's ui.TextArea has no read-only mode. Paragraph keeps
		// responses immutable; the explicit Copy action uses the full body.
		ui.Scroll(c).FillWidth().Grow(1).Scrollbars().Children(func() {
			ui.Paragraph(c, body, 0).Mono().FillWidth()
		})
	}
}

func (a *App) responseHeaderTable(c ui.C, r *model.Response) {
	ui.Scroll(c).FillWidth().Grow(1).Scrollbars().Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Row(c).FillWidth().Gap(16).Children(func() {
				ui.Text(c, a.S.Response.HeaderName).Width(200).Bold()
				ui.Text(c, a.S.Response.HeaderValue).Grow(1).Bold()
			})
			ui.Divider(c)
			for i, header := range r.Headers {
				ui.Row(c.KeyInt(i)).FillWidth().Gap(16).Children(func() {
					ui.Paragraph(c, header.Key, 0).Width(200).Mono()
					ui.Paragraph(c, header.Value, 0).Grow(1).Mono()
				})
			}
			if len(r.Headers) == 0 {
				ui.Text(c, a.S.Response.NoHeaders).Muted()
			}
		})
	})
}
