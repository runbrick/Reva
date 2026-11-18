package gui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/format"
	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

const responseDisplayLimit = 2 * 1024 * 1024

var responseSections = []responseSection{responseBody, responseHeaders, responseCookies, responseTimeline, responseTests}
var responseStreamSections = []responseSection{responseEvents, responseBody, responseHeaders, responseCookies, responseTimeline, responseTests}
var responseModes = []responseMode{responsePretty, responseRaw, responsePreview}

type responseSection int

const (
	responseBody responseSection = iota
	responseHeaders
	responseCookies
	responseTimeline
	responseTests
	responseEvents
)

type responseMode int

const (
	responsePretty responseMode = iota
	responseRaw
	responsePreview
)

// responseView keeps each request tab's selection and most recent immutable
// response. Repainting, switching sections or visiting another tab reuses it.
type responseView struct {
	section                     responseSection
	mode                        responseMode
	source                      *model.Response
	raw, pretty, duration, size string
	rawLimited, prettyLimited   bool
	json                        bool
	testsCount                  int
	testsPattern, testsLabel    string
	rawTokens, prettyTokens     [][]format.Token
	code                        responseCodeView
	wrap, find                  bool
	query, searched, searchBody string
	matches                     []text.Span
	match                       int
	count, countPattern         string
	countIndex, countTotal      int
	findKey                     func(gfx.KeyEvent) gfx.EventResult
	previewText                 string
	previewHTML                 bool
	previewImage                gfx.Bitmap
	cookieExpires               []string
	phaseTimes                  []string
	phaseTimePattern            string
	phaseShares                 []float32
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
	// Format the received body before limiting its display, so a large valid
	// JSON response still shows an indented prefix in Pretty mode. Invalid or
	// incomplete JSON falls back to the original text.
	v.json = format.IsJSON(r.Header("Content-Type"), r.Body)
	if v.json {
		if pretty, err := format.PrettyJSON(r.Body); err == nil {
			v.pretty, v.prettyLimited = responseDisplayText(pretty)
		}
	}
	v.duration, v.size = format.Duration(r.Duration), format.ByteSize(r.Size)
	v.rawTokens = format.TokenizeJSON(v.raw)
	v.prettyTokens = v.rawTokens
	if v.pretty != v.raw {
		v.prettyTokens = format.TokenizeJSON(v.pretty)
	}
	v.code = responseCodeView{}
	v.searchBody = ""
	v.searched = ""
	v.matches = nil
	v.match = 0
	v.previewText, v.previewImage = "", gfx.Bitmap{}
	contentType := strings.ToLower(r.Header("Content-Type"))
	v.previewHTML = strings.HasPrefix(contentType, "text/html")
	if v.previewHTML {
		v.previewText = format.HTMLText(v.raw)
	} else if strings.HasPrefix(contentType, "image/") {
		v.previewImage = responseImage(r.Body)
	}
	v.cookieExpires = make([]string, len(r.Cookies))
	for i, cookie := range r.Cookies {
		if !cookie.Expires.IsZero() {
			v.cookieExpires[i] = cookie.Expires.UTC().Format(time.RFC3339)
		}
	}
	v.phaseTimes = make([]string, len(r.Timeline))
	v.phaseTimePattern = ""
	v.phaseShares = make([]float32, len(r.Timeline))
	var total time.Duration
	for _, phase := range r.Timeline {
		total += max(phase.Duration, 0)
	}
	for i, phase := range r.Timeline {
		if total > 0 {
			v.phaseShares[i] = float32(float64(max(phase.Duration, 0)) / float64(total))
		}
	}
	if v.findKey == nil {
		v.findKey = v.onFindKey
	}
}

// responseImage converts standard-library image formats to Viva's immutable
// premultiplied BGRA bitmap once. Bound decoded dimensions before allocating.
func responseImage(body []byte) gfx.Bitmap {
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16*1024*1024 {
		return gfx.Bitmap{}
	}
	source, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return gfx.Bitmap{}
	}
	w, h := config.Width, config.Height
	pixels := make([]byte, w*h*4)
	bounds := source.Bounds()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := source.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			i := (y*w + x) * 4
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = byte(b>>8), byte(g>>8), byte(r>>8), byte(a>>8)
		}
	}
	return gfx.Bitmap{Pix: pixels, Width: w, Height: h, Stride: w * 4}
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

func (a *App) pruneResponseViews() {
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
}

// responsePane declares the response of t: status, duration, size, headers
// and body (pretty / raw).
func (a *App) responsePane(c ui.C, t *Tab) {
	if t.Response != nil && (t.sse != nil && t.sse.status != 0 || !t.Sending && t.Err == "") {
		a.responseResult(c, t)
		return
	}
	ui.Column(c).Fill().Pad(Space4).Gap(Space2).Background(a.palette.Surface).Children(func() {
		switch {
		case t.sse != nil && t.sse.status != 0:
			a.responseSSE(c, t)
		case t.Sending:
			ui.Row(c).Gap(Space2).Items(ui.Center).Children(func() {
				ui.Spinner(c)
				ui.Text(c, a.S.Response.Sending).Muted()
			})
		case t.Err != "":
			ui.Row(c).Gap(Space2).Items(ui.Center).Children(func() {
				ui.Icon(c, icons.CircleX).Size(Space4, Space4).Color(c.Theme().Danger)
				ui.Text(c, a.S.Response.ErrorTitle).Bold().Color(c.Theme().Danger)
			})
			ui.Scroll(c).FillWidth().Grow(1).Children(func() {
				ui.Paragraph(c, t.Err, 0).Color(c.Theme().Danger)
			})
		default:
			ui.Column(c).Fill().Center().Gap(Space3).Children(func() {
				theme := c.Theme()
				ui.Row(c).Size(Space6*2, Space6*2).Radius(theme.Metrics.RadiusSurface).Background(theme.Muted).
					Center().Children(func() { ui.Icon(c, icons.Send).Size(Space6, Space6).Color(theme.MutedForeground) })
				ui.Paragraph(c, a.S.Response.EmptyHint, 0).Muted()
			})
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
	sections := responseSections
	if t.sse != nil && t.sse.status != 0 {
		sections = responseStreamSections
		if v.source != t.Response {
			v.section = responseEvents
		}
	} else if v.section == responseEvents {
		v.section = responseBody
	}
	v.prepare(t.Response)
	if v.phaseTimePattern != a.S.Response.PhaseDuration {
		v.phaseTimePattern = a.S.Response.PhaseDuration
		for i, phase := range t.Response.Timeline {
			v.phaseTimes[i] = fmt.Sprintf(v.phaseTimePattern, float64(phase.Duration)/float64(time.Millisecond))
		}
	}
	if v.testsPattern != a.S.Response.TestsCount || v.testsCount != len(t.Tests) {
		v.testsPattern, v.testsCount = a.S.Response.TestsCount, len(t.Tests)
		v.testsLabel = fmt.Sprintf(v.testsPattern, v.testsCount)
	}
	theme := c.Theme()
	statusColor := a.palette.StatusColor(theme, t.Response.StatusCode)
	labels := &a.S.Response
	ui.Row(c).FillWidth().Height(TabBarHeight).Shrink(0).PadX(Space4).Gap(Space2).Items(ui.Center).
		Background(theme.Card).Children(func() {
		a.responseTabs(c, v, sections)
		ui.Row(c).MaxWidth(Space6 * 9).Shrink(0).PadX(Space2).PadY(Space1).Gap(Space1).Items(ui.Center).
			Radius(gfx.RadiusFull).Background(statusColor.WithAlpha(0.12)).Children(func() {
			statusDot(c, statusColor)
			ui.Text(c, t.Response.Status).FontSize(theme.Metrics.FontSizeS).Color(statusColor).Ellipsis()
		})
		ui.Text(c, v.duration).FontSize(theme.Metrics.FontSizeS).Muted().Shrink(0)
		ui.Text(c, v.size).FontSize(theme.Metrics.FontSizeS).Muted().Shrink(0)
		ui.Divider(c).Vertical().Height(Space4)
		a.responseCopyButton(c, t.Response)
	})
	ui.Divider(c)
	if t.Response.Truncated {
		ui.Paragraph(c, labels.Truncated, 0).Pad(Space2).Color(a.palette.Warning)
	}
	if v.section == responseBody {
		a.responseWorkbench(c, v)
		return
	}
	ui.Column(c).FillWidth().Grow(1).Pad(Space4).Gap(Space2).Children(func() {
		switch v.section {
		case responseEvents:
			a.responseSSE(c, t)
		case responseHeaders:
			a.responseHeaderTable(c, t.Response)
		case responseCookies:
			a.responseCookieTable(c, v)
		case responseTimeline:
			a.responseTimelineChart(c, v)
		case responseTests:
			a.testsSection(c, t)
		}
	})
}

func (a *App) responseTabs(c ui.C, v *responseView, sections []responseSection) {
	theme, labels := c.Theme(), &a.S.Response
	ui.Scroll(c).FillHeight().Grow(1).MinWidth(0).Sideways().Children(func() {
		ui.Row(c).Height(TabBarHeight).Gap(Space1).FocusGroup(ui.FocusHorizontal).Children(func() {
			for _, section := range sections {
				label := labels.Body
				switch section {
				case responseEvents:
					label = labels.SSEEvents
				case responseHeaders:
					label = labels.Headers
				case responseCookies:
					label = labels.Cookies
				case responseTimeline:
					label = labels.Timeline
				case responseTests:
					label = labels.Tests
					if v.testsCount > 0 {
						label = v.testsLabel
					}
				}
				ui.TabBase(c.KeyInt(int(section)), &v.section, section, label).FillHeight().PadX(Space2).
					Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring)).Children(func() {
					text := ui.Text(c, label)
					if v.section == section {
						text.Color(theme.AccentForeground).Bold()
						ui.Slot(c).Absolute().Left(0).Right(0).Bottom(0).Height(Space1 / 2).Background(theme.Primary)
					} else {
						text.Muted()
					}
				})
			}
		})
	})
}

func (a *App) responseCopy(c ui.C, body string) {
	c.After(func() {
		if err := app.SetClipboardText(body); err != nil {
			a.Toast(a.S.Response.CopyError, err.Error(), ui.ToastError)
		}
	})
}

func (a *App) responseCopyButton(c ui.C, r *model.Response) {
	if ui.Button(c, a.S.Response.Copy).Icon(icons.Copy).IconOnly().Ghost().Tooltip(a.S.Response.Copy).Clicked() {
		a.responseCopy(c, string(r.Body))
	}
}

func (a *App) responseWorkbench(c ui.C, v *responseView) {
	ui.Row(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).Grow(1).FillHeight().Children(func() { a.responseBodyView(c, v) })
	})
}

func (a *App) responseBodyView(c ui.C, v *responseView) {
	pane := ui.Column(c).FillWidth().Grow(1)
	if pane.Shortcut(ui.Primary, "f") && v.mode != responsePreview {
		v.find = true
	}
	if pane.Shortcut(ui.Primary, "a") && v.mode != responsePreview && v.code.editor != nil {
		v.code.editor.SetSelection(text.Range{End: v.code.editor.RuneCount()})
	}
	if pane.Shortcut(ui.Primary, "c") && v.mode != responsePreview && v.code.editor != nil {
		if selected := v.code.selectedText(); selected != "" {
			a.responseCopy(c, selected)
		}
	}
	pane.Children(func() { a.responseBodyContent(c, v) })
}

func (a *App) responseModeLabel(mode responseMode) string {
	switch mode {
	case responseRaw:
		return a.S.Response.Raw
	case responsePreview:
		return a.S.Response.Preview
	default:
		return a.S.Response.Pretty
	}
}

func (a *App) responseBodyContent(c ui.C, v *responseView) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Height(TabBarHeight).Shrink(0).PadX(Space4).Gap(Space2).Items(ui.Center).
		Background(theme.Card).Children(func() {
		ui.Choice(c, &v.mode, responseModes, a.responseModeLabel)
		if v.json {
			ui.Text(c, "JSON").FontSize(theme.Metrics.FontSizeS).Bold().Muted().PadX(Space2).
				Background(theme.Secondary).Radius(theme.Metrics.RadiusControl).Shrink(0)
		}
		if ui.Button(c, a.S.Response.Wrap).Icon(icons.Braces).IconOnly().Ghost().Tooltip(a.S.Response.Wrap).Disabled(v.mode == responsePreview).Clicked() {
			v.wrap = !v.wrap
		}
		ui.Spacer(c)
		if ui.Button(c, a.S.Response.Search).Icon(icons.Search).IconOnly().Ghost().Tooltip(a.S.Response.Search).Disabled(v.mode == responsePreview).Clicked() {
			v.find = !v.find
		}
	})
	if v.mode == responsePreview {
		ui.Column(c).FillWidth().Grow(1).Pad(Space4).Children(func() { a.responsePreviewView(c, v) })
		return
	}
	body, tokens, limited := v.pretty, v.prettyTokens, v.prettyLimited
	if v.mode == responseRaw {
		body, tokens, limited = v.raw, v.rawTokens, v.rawLimited
	}
	v.code.set(body, tokens)
	if limited {
		ui.Paragraph(c, a.S.Response.DisplayLimit, 0).Pad(Space2).Muted()
	}
	if v.find {
		a.responseFindBar(c, v)
	}
	v.updateSearch(body)
	var marks []text.Span
	if v.find {
		marks = v.matches
	}
	v.code.declare(c, a.S.Response.CodeBody, a.palette, v.wrap, marks)
}

func (v *responseView) updateSearch(body string) {
	if v.searched == v.query && v.searchBody == body {
		return
	}
	v.searched, v.searchBody, v.match = v.query, body, 0
	v.matches = v.matches[:0]
	if v.query == "" {
		return
	}
	for from := 0; from <= len(body); {
		at := strings.Index(body[from:], v.query)
		if at < 0 {
			break
		}
		at += from
		v.matches = append(v.matches, text.Span{Start: at, End: at + len(v.query)})
		from = at + len(v.query)
	}
	if len(v.matches) > 0 {
		v.code.selectMatch(v.matches[0])
	}
}

func (v *responseView) stepMatch(delta int) {
	if len(v.matches) == 0 {
		return
	}
	v.match = (v.match + delta + len(v.matches)) % len(v.matches)
	v.code.selectMatch(v.matches[v.match])
}

func (v *responseView) onFindKey(event gfx.KeyEvent) gfx.EventResult {
	command, ok := components.FindBarCommandForKey(event, true)
	if !ok {
		return gfx.EventResult{}
	}
	switch command {
	case components.FindBarNext:
		v.stepMatch(1)
	case components.FindBarPrevious:
		v.stepMatch(-1)
	case components.FindBarCloseCommand:
		v.find = false
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (a *App) responseFindBar(c ui.C, v *responseView) {
	ui.Row(c).FillWidth().Gap(Space1).Items(ui.Center).Children(func() {
		ui.TextField(c, &v.query, a.S.Response.SearchHint, a.S.Response.SearchHint).Icon(icons.Search).Grow(1).AutoFocus().OnKey(v.findKey)
		v.updateSearch(v.code.snapshot.Value)

		if ui.Button(c, a.S.Response.PreviousMatch).Icon(icons.ChevronUp).IconOnly().Ghost().Disabled(len(v.matches) == 0).Clicked() {
			v.stepMatch(-1)
		}
		if ui.Button(c, a.S.Response.NextMatch).Icon(icons.ChevronDown).IconOnly().Ghost().Disabled(len(v.matches) == 0).Clicked() {
			v.stepMatch(1)
		}
		if len(v.matches) == 0 {
			ui.Text(c, a.S.Response.NoMatches).Muted()
		} else {
			if v.countPattern != a.S.Response.MatchCount || v.countIndex != v.match || v.countTotal != len(v.matches) {
				v.countPattern, v.countIndex, v.countTotal = a.S.Response.MatchCount, v.match, len(v.matches)
				v.count = fmt.Sprintf(v.countPattern, v.match+1, len(v.matches))
			}
			ui.Text(c, v.count).Muted()
		}
		if ui.Button(c, a.S.Response.CloseSearch).Icon(icons.X).IconOnly().Ghost().Clicked() {
			v.find = false
		}
	})
}

func (a *App) responsePreviewView(c ui.C, v *responseView) {
	ui.Scroll(c).FillWidth().Grow(1).Scrollbars().Children(func() {
		switch {
		case v.previewHTML:
			if v.previewText == "" {
				ui.Text(c, a.S.Response.EmptyPreview).Muted()
			} else {
				ui.Paragraph(c, v.previewText, 0).FillWidth()
			}
		case len(v.previewImage.Pix) > 0:
			width := min(float32(v.previewImage.Width), c.Bounds().Width-Space6*2)
			ui.Image(c, v.previewImage).Size(width, width*float32(v.previewImage.Height)/float32(v.previewImage.Width))
		default:
			ui.Row(c).Gap(Space2).Items(ui.Center).Children(func() {
				ui.Icon(c, icons.Eye).Size(Space4, Space4).Color(c.Theme().MutedForeground)
				ui.Text(c, a.S.Response.PreviewUnavailable).Muted()
			})
		}
	})
}

func (a *App) responseHeaderTable(c ui.C, r *model.Response) {
	ui.Scroll(c).FillWidth().Grow(1).Scrollbars().Children(func() {
		ui.Column(c).FillWidth().Gap(Space2).Children(func() {
			ui.Row(c).FillWidth().Gap(Space4).Pad(Space2).Background(a.palette.TableHeader).Children(func() {
				ui.Text(c, a.S.Response.HeaderName).Width(Space6 * 8).Bold()
				ui.Text(c, a.S.Response.HeaderValue).Grow(1).Bold()
			})
			ui.Divider(c)
			for i, header := range r.Headers {
				ui.Row(c.KeyInt(i)).FillWidth().Gap(Space4).PadX(Space2).Children(func() {
					ui.Paragraph(c, header.Key, 0).Width(Space6 * 8).Mono()
					ui.Paragraph(c, header.Value, 0).Grow(1).Mono()
				})
			}
			if len(r.Headers) == 0 {
				ui.Text(c, a.S.Response.NoHeaders).Muted()
			}
		})
	})
}

func (a *App) responseCookieTable(c ui.C, v *responseView) {
	if len(v.source.Cookies) == 0 {
		a.responseEmpty(c, icons.Cookie, a.S.Response.NoCookies)
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Scrollbars().Children(func() {
		ui.Column(c).FillWidth().MinWidth(Space6 * 43).Gap(Space2).Children(func() {
			ui.Row(c).FillWidth().Pad(Space2).Gap(Space3).Background(a.palette.TableHeader).Children(func() {
				for i, label := range []string{a.S.Response.HeaderName, a.S.Response.HeaderValue, a.S.Response.CookieDomain, a.S.Response.CookiePath, a.S.Response.CookieExpires, a.S.Response.CookieHTTPOnly, a.S.Response.CookieSecure} {
					ui.Text(c.KeyInt(i), label).Width(responseCookieWidth(i)).Bold()
				}
			})
			for i, cookie := range v.source.Cookies {
				ui.Row(c.KeyInt(i)).FillWidth().PadX(Space2).Gap(Space3).Children(func() {
					expires := v.cookieExpires[i]
					if expires == "" {
						expires = a.S.Response.SessionCookie
					}
					for j, value := range []string{cookie.Name, cookie.Value, cookie.Domain, cookie.Path, expires, a.responseBool(cookie.HTTPOnly), a.responseBool(cookie.Secure)} {
						ui.Paragraph(c.KeyInt(j), value, 0).Width(responseCookieWidth(j)).Mono()
					}
				})
			}
		})
	})
}

func responseCookieWidth(column int) float32 {
	switch column {
	case 0, 2:
		return Space6 * 5
	case 1:
		return Space6 * 7
	case 4:
		return Space6 * 9
	default:
		return Space6 * 4
	}
}

func (a *App) responseBool(value bool) string {
	if value {
		return a.S.Response.Yes
	}
	return a.S.Response.No
}

func (a *App) responseEmpty(c ui.C, icon gfx.Icon, label string) {
	ui.Row(c).FillWidth().Gap(Space2).Pad(Space4).Items(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(Space6, Space6).Color(c.Theme().MutedForeground)
		ui.Text(c, label).Muted()
	})
}

func (a *App) responseTimelineChart(c ui.C, v *responseView) {
	if len(v.source.Timeline) == 0 {
		a.responseEmpty(c, icons.Activity, a.S.Response.NoTimeline)
		return
	}
	ui.Scroll(c).FillWidth().Grow(1).Scrollbars().Children(func() {
		ui.Column(c).FillWidth().Gap(Space3).Pad(Space2).Children(func() {
			for i, phase := range v.source.Timeline {
				ui.Row(c.KeyInt(i)).FillWidth().Gap(Space3).Items(ui.Center).Children(func() {
					ui.Text(c, a.responsePhaseLabel(phase.Kind)).Width(Space6 * 6)
					ui.Slot(c).Grow(1).Height(Space3).Background(c.Theme().Muted).Radius(c.Theme().Metrics.RadiusControl).Draw(func(d *ui.DrawContext) {
						r := d.Rect
						r.Width *= v.phaseShares[i]
						d.Canvas.Rounded(r, d.Theme.Metrics.RadiusControl, d.Theme.Primary)
					})
					ui.Text(c, v.phaseTimes[i]).Width(Space6 * 5).Mono()
				})
			}
			ui.Divider(c)
			ui.Row(c).FillWidth().Gap(Space3).Children(func() {
				ui.Text(c, a.S.Response.Total).Bold()
				ui.Spacer(c)
				ui.Text(c, v.duration).Bold().Mono()
			})
		})
	})
}

func (a *App) responsePhaseLabel(kind model.PhaseKind) string {
	switch kind {
	case model.PhaseDNS:
		return a.S.Response.PhaseDNS
	case model.PhaseConnect:
		return a.S.Response.PhaseConnect
	case model.PhaseTLS:
		return a.S.Response.PhaseTLS
	case model.PhaseSend:
		return a.S.Response.PhaseSend
	case model.PhaseWait:
		return a.S.Response.PhaseWait
	case model.PhaseDownload:
		return a.S.Response.PhaseDownload
	default:
		return a.S.Response.PhaseOther
	}
}
