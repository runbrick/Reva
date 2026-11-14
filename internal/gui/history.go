package gui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// historyState owns the page's filters, selection and cached display strings.
type historyState struct {
	search, cachedSearch string
	method, cachedMethod int
	status, cachedStatus int
	list                 ui.ListState
	rows                 []historyRow
	keys                 map[string]int
	nextKey, count       int
	newest               string
	day                  time.Time
	strings              i18n.History
	ready, confirmClear  bool
}

type historyRow struct {
	key                                                 int
	header                                              bool
	entry                                               model.HistoryEntry
	label, status, at, duration, headers, body, preview string
}

func (s *historyState) rowKey(i int) int      { return s.rows[i].key }
func (s *historyState) rowLabel(i int) string { return s.rows[i].label }
func (s *historyState) rowHeader(i int) bool  { return s.rows[i].header }

var historyMethods = [...]model.Method{"", model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch, model.MethodDelete}

func (s *historyState) matches(entry model.HistoryEntry, query string) bool {
	if !strings.Contains(strings.ToLower(entry.Request.URL), query) && !strings.Contains(strings.ToLower(string(entry.Request.Method)), query) {
		return false
	}
	if s.method > 0 && s.method < len(historyMethods) && entry.Request.Method != historyMethods[s.method] {
		return false
	}
	if s.method == len(historyMethods) {
		for _, method := range historyMethods[1:] {
			if entry.Request.Method == method {
				return false
			}
		}
	}
	failed := entry.Error != "" || entry.StatusCode == 0
	if s.status == 5 {
		return failed
	}
	return s.status == 0 || !failed && entry.StatusCode/100 == s.status+1
}

func (s *historyState) rebuild(entries []model.HistoryEntry, text i18n.History, now time.Time) {
	newest := ""
	if len(entries) > 0 {
		newest = entries[0].ID
	}
	localDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// Entries are immutable; Add changes the newest ID even at the history limit.
	// Cache filtering, grouping and summaries by this signature, filters and locale.
	if s.ready && s.count == len(entries) && s.newest == newest && s.cachedSearch == s.search && s.cachedMethod == s.method && s.cachedStatus == s.status && s.day.Equal(localDay) && s.strings == text {
		return
	}
	selectedID := ""
	if i := s.list.Selected(); i >= 0 && i < len(s.rows) && !s.rows[i].header {
		selectedID = s.rows[i].entry.ID
	}
	s.ready, s.count, s.newest = true, len(entries), newest
	s.cachedSearch, s.cachedMethod, s.cachedStatus, s.day, s.strings = s.search, s.method, s.status, localDay, text
	previousKeys := s.keys
	s.keys = make(map[string]int, len(entries)*2)
	key := func(id string) int {
		k, ok := previousKeys[id]
		if !ok {
			s.nextKey++
			k = s.nextKey
		}
		s.keys[id] = k
		return k
	}
	clear(s.rows)
	s.rows = s.rows[:0]
	query := strings.ToLower(s.search)
	today, yesterday := now.Format(time.DateOnly), now.AddDate(0, 0, -1).Format(time.DateOnly)
	lastDay, selected := "", -1
	for _, entry := range entries {
		if !s.matches(entry, query) {
			continue
		}
		at := entry.At.In(now.Location())
		day := at.Format(time.DateOnly)
		if day != lastDay {
			label := at.Format(text.DateFormat)
			switch day {
			case today:
				label = text.Today
			case yesterday:
				label = text.Yesterday
			}
			s.rows = append(s.rows, historyRow{key: key("day:" + day), header: true, label: label})
			lastDay = day
		}
		status := strconv.Itoa(entry.StatusCode)
		if entry.Error != "" || entry.StatusCode == 0 {
			status = text.Error
		}
		headers := strings.Builder{}
		for _, header := range entry.Request.Headers {
			if header.Disabled || header.Key == "" {
				continue
			}
			if headers.Len() > 0 {
				headers.WriteByte('\n')
			}
			headers.WriteString(header.Key)
			headers.WriteString(": ")
			headers.WriteString(header.Value)
		}
		headerText := headers.String()
		if headerText == "" {
			headerText = text.NoHeaders
		}
		body, preview := text.NoBody, ""
		b := entry.Request.Body
		switch b.Mode {
		case model.BodyJSON, model.BodyRaw:
			body = fmt.Sprintf(text.BodyText, b.Mode, len(b.Text))
			// Bound a detail preview; opening always restores the complete request.
			preview = b.Text
			if len(preview) > 512 {
				runes := []rune(preview)
				preview = string(runes[:min(len(runes), 512)])
			}
		case model.BodyBinary:
			body = fmt.Sprintf(text.BodyFile, b.FilePath)
		case model.BodyFormData:
			n := 0
			for _, f := range b.Form {
				if !f.Disabled {
					n++
				}
			}
			body = fmt.Sprintf(text.BodyFields, n)
		case model.BodyURLEncoded:
			n := 0
			for _, f := range b.URLEncoded {
				if !f.Disabled {
					n++
				}
			}
			body = fmt.Sprintf(text.BodyFields, n)
		}
		if entry.ID == selectedID {
			selected = len(s.rows)
		}
		s.rows = append(s.rows, historyRow{key: key("entry:" + entry.ID), entry: entry,
			label: string(entry.Request.Method) + " " + entry.Request.URL, status: status,
			at: at.Format(text.TimeFormat), duration: entry.Duration.String(), headers: headerText, body: body, preview: preview})
	}
	s.list.Key, s.list.Label, s.list.Header = s.rowKey, s.rowLabel, s.rowHeader
	s.list.Select(selected)
	if selected < 0 {
		s.list.ScrollTo(0, ui.ListStart)
	}
}

// historyPane remains an alias for callers of the original pane.
func (a *App) historyPane(c ui.C) { a.historyPage(c) }

func (a *App) historyPage(c ui.C) {
	s, text, theme := &a.history, a.S.History, c.Theme()
	entries := a.Svc.History.List()
	ui.Column(c).Fill().Pad(Space6).Gap(Space4).Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Gap(Space4).Children(func() {
			ui.Icon(c, icons.History).Size(Space6, Space6).Color(theme.Primary)
			ui.Text(c, text.Title).Bold().FontSize(theme.Metrics.FontSizeL)
			ui.Spacer(c)
			if ui.Button(c, text.Clear).Icon(icons.Trash2).Destructive().Disabled(len(entries) == 0).Clicked() {
				s.confirmClear = true
			}
		})
		field := ui.TextField(c, &s.search, text.Search, text.SearchPlaceholder).Icon(icons.Search).Action(icons.X, text.ClearSearch).FillWidth()
		if field.ActionClicked() {
			s.search = ""
		}
		a.historyFilters(c)
		s.rebuild(entries, text, time.Now())
		wide := c.Bounds().Width >= 75*Space4
		s.list.RowWidth = max(42*Space4, c.Bounds().Width-RailWidth-2*Space6)
		if wide {
			s.list.RowWidth = max(42*Space4, s.list.RowWidth-21*Space4)
		}
		var content ui.Box
		if wide {
			content = ui.Row(c).FillWidth().Grow(1).MinHeight(0).Gap(Space4)
		} else {
			content = ui.Column(c).FillWidth().Grow(1).MinHeight(0).Gap(Space4)
		}
		content.Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).FillWidth().Children(func() { a.historyTable(c) })
			var detail ui.Box
			if wide {
				detail = ui.Column(c).Width(20 * Space4).FillHeight()
			} else {
				detail = ui.Column(c).FillWidth().Height(15 * Space4)
			}
			detail.Background(theme.Card).Border(1, theme.Border).Radius(theme.Metrics.RadiusSurface).Pad(Space4).Gap(Space3).Children(func() { a.historyDetail(c) })
		})
	})
	ui.Modal(c, &s.confirmClear, text.ConfirmTitle, func() { ui.Paragraph(c, text.ConfirmBody, 0) }, func() {
		if ui.Button(c, text.Cancel).Outline().Clicked() {
			s.confirmClear = false
		}
		if ui.Button(c, text.ConfirmClear).Destructive().Clicked() {
			c.After(func() {
				if err := a.Svc.History.Clear(); err != nil {
					a.Toast(text.Error, err.Error(), ui.ToastError)
					return
				}
				s.confirmClear = false
			})
		}
	})
}

func (a *App) historyFilters(c ui.C) {
	s, text := &a.history, a.S.History
	ui.Flow(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		ui.Text(c, text.Method).Muted().Width(4 * Space4)
		for i := range len(historyMethods) + 1 {
			label := text.AllMethods
			if i > 0 && i < len(historyMethods) {
				label = string(historyMethods[i])
			} else if i == len(historyMethods) {
				label = text.Other
			}
			chip := ui.Button(c.KeyInt(i), label).Small()
			if s.method != i {
				chip.Outline()
			}
			if chip.Clicked() {
				s.method = i
			}
		}
	})
	ui.Flow(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		ui.Text(c, text.Status).Muted().Width(4 * Space4)
		for i, label := range [...]string{text.AllStatuses, text.Success, text.Redirect, text.ClientError, text.ServerError, text.Error} {
			chip := ui.Button(c.KeyInt(i), label).Small()
			if s.status != i {
				chip.Outline()
			}
			if chip.Clicked() {
				s.status = i
			}
		}
	})
}

func (a *App) historyTable(c ui.C) {
	s, text, theme := &a.history, a.S.History, c.Theme()
	if len(s.rows) == 0 {
		empty := text.Empty
		if s.count > 0 {
			empty = text.NoMatches
		}
		ui.Column(c).Fill().Center().Gap(Space3).Children(func() {
			ui.Icon(c, icons.History).Size(2*Space6, 2*Space6).Color(theme.MutedForeground)
			ui.Paragraph(c, empty, 0).Muted().FillWidth()
		})
		return
	}
	ui.Row(c).FillWidth().Height(theme.Metrics.ControlHeight).Clip().Children(func() {
		ui.Row(c).Width(s.list.RowWidth).Height(theme.Metrics.ControlHeight).PadX(Space3).Gap(Space2).Background(a.palette.TableHeader).ScrollWith(&s.list).Children(func() {
			ui.Text(c, text.Time).Width(5 * Space4).Muted()
			ui.Text(c, text.Method).Width(4 * Space4).Muted()
			ui.Text(c, text.URL).Grow(1).MinWidth(6 * Space4).Muted()
			ui.Text(c, text.Status).Width(4 * Space4).Muted()
			ui.Text(c, text.Duration).Width(5 * Space4).Muted()
			ui.Text(c, text.Actions).Width(7 * Space4).Muted()
		})
	})
	ui.List(c, &s.list, len(s.rows), a.historyRow).Label(text.Title).FillWidth().Grow(1)
	if i, ok := s.list.Submitted(); ok && i >= 0 && i < len(s.rows) && !s.rows[i].header {
		a.openHistory(s.rows[i].entry, false)
	}
}

func (a *App) historyRow(row ui.ListRow) {
	item, theme, text := a.history.rows[row.Index], row.C.Theme(), a.S.History
	if row.Header {
		row.Element.Background(a.palette.TableHeader).PadX(Space3).PadY(Space2).Children(func() { ui.Text(row.C, item.label).Muted().Bold().FontSize(theme.Metrics.FontSizeS) })
		return
	}
	entry := item.entry
	box := row.Element.Height(theme.Metrics.ControlHeight + Space2).PadX(Space3).Gap(Space2).Items(ui.Center).
		Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(1, theme.Ring))
	if row.Selected() {
		box.Background(theme.Accent)
	}
	box.Children(func() {
		ui.Text(row.C, item.at).Width(5 * Space4).Muted().FontSize(theme.Metrics.FontSizeS)
		a.MethodBadge(row.C, entry.Request.Method).Width(4 * Space4)
		ui.Text(row.C, entry.Request.URL).Grow(1).MinWidth(6 * Space4).Ellipsis()
		code := entry.StatusCode
		if entry.Error != "" {
			code = 0
		}
		ui.Text(row.C, item.status).Width(4 * Space4).Color(a.palette.StatusColor(theme, code)).FontSize(theme.Metrics.FontSizeS)
		ui.Text(row.C, item.duration).Width(5 * Space4).Muted().FontSize(theme.Metrics.FontSizeS)
		actions := ui.Row(row.C).Width(7 * Space4).Gap(Space1).Opacity(0).StateFrom(box).Hover(ui.Look{}.Opacity(1)).Focused(ui.Look{}.Opacity(1))
		if row.Selected() {
			actions.Opacity(1)
		}
		actions.Children(func() {
			if ui.Button(row.C, text.Open).Icon(icons.ExternalLink).Small().IconOnly().Ghost().Width(2 * Space4).Clicked() {
				row.C.After(func() { a.openHistory(entry, false) })
			}
			if ui.Button(row.C, text.CopyURL).Icon(icons.Copy).Small().IconOnly().Ghost().Width(2 * Space4).Clicked() {
				row.C.After(func() { a.copyHistoryURL(entry.Request.URL) })
			}
			if ui.Button(row.C, text.Delete).Icon(icons.Trash2).Small().IconOnly().Ghost().Width(2 * Space4).Color(theme.Danger).Clicked() {
				row.C.After(func() { a.deleteHistory(entry.ID) })
			}
		})
	})
	ui.ContextMenu(row.C, box, func(menu *ui.MenuItems) {
		if menu.Item(text.Open).Icon(icons.ExternalLink).Clicked() {
			row.C.After(func() { a.openHistory(entry, false) })
		}
		if menu.Item(text.CopyURL).Icon(icons.Copy).Clicked() {
			row.C.After(func() { a.copyHistoryURL(entry.Request.URL) })
		}
		if menu.Item(text.Delete).Icon(icons.Trash2).Destructive().Clicked() {
			row.C.After(func() { a.deleteHistory(entry.ID) })
		}
	})
}

func (a *App) historyDetail(c ui.C) {
	s, text, theme := &a.history, a.S.History, c.Theme()
	ui.Text(c, text.Detail).Bold()
	i := s.list.Selected()
	if i < 0 || i >= len(s.rows) || s.rows[i].header {
		ui.Paragraph(c, text.SelectEntry, 0).Muted().FillWidth()
		return
	}
	item := s.rows[i]
	ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Scrollbars().Children(func() {
		ui.Column(c).FillWidth().Gap(Space2).Children(func() {
			a.MethodBadge(c, item.entry.Request.Method)
			ui.Paragraph(c, item.entry.Request.URL, 3).FillWidth()
			code := item.entry.StatusCode
			if item.entry.Error != "" {
				code = 0
			}
			ui.Row(c).FillWidth().Gap(Space2).Children(func() {
				ui.Text(c, text.Status).Muted().FontSize(theme.Metrics.FontSizeS)
				ui.Text(c, item.status).Color(a.palette.StatusColor(theme, code))
				ui.Spacer(c)
				ui.Text(c, text.Duration).Muted().FontSize(theme.Metrics.FontSizeS)
				ui.Text(c, item.duration)
			})
			if item.entry.Error != "" {
				ui.Paragraph(c, item.entry.Error, 0).FillWidth().Color(theme.Danger)
			}
			ui.Text(c, item.entry.At.Local().Format(text.DateFormat+" "+text.TimeFormat)).Muted().FontSize(theme.Metrics.FontSizeS)
			ui.Divider(c)
			ui.Text(c, text.Headers).Bold().FontSize(theme.Metrics.FontSizeS)
			ui.Paragraph(c, item.headers, 8).FillWidth()
			ui.Text(c, text.Body).Bold().FontSize(theme.Metrics.FontSizeS)
			ui.Paragraph(c, item.body, 2).FillWidth().Muted()
			if item.preview != "" {
				ui.Paragraph(c, item.preview, 5).FillWidth().Mono()
			}
		})
	})
	ui.Row(c).FillWidth().Gap(Space2).Children(func() {
		if ui.Button(c, text.Open).Icon(icons.ExternalLink).Outline().Clicked() {
			c.After(func() { a.openHistory(item.entry, false) })
		}
		if ui.Button(c, text.Resend).Icon(icons.Send).Clicked() {
			c.After(func() { a.openHistory(item.entry, true) })
		}
	})
}

func (a *App) openHistory(entry model.HistoryEntry, resend bool) {
	a.OpenRequest(entry.Request, "")
	a.Main = PageWorkbench
	if resend {
		a.Send(a.Active())
	}
}

func (a *App) copyHistoryURL(url string) {
	if err := app.SetClipboardText(url); err != nil {
		a.Toast(a.S.History.CopyError, err.Error(), ui.ToastError)
	}
}

func (a *App) deleteHistory(id string) {
	if err := a.Svc.History.Delete(id); err != nil {
		a.Toast(a.S.History.Error, err.Error(), ui.ToastError)
	}
}
