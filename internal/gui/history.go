package gui

import (
	"strconv"
	"strings"
	"time"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/ui"
)

// historyState is the history pane's UI state. Owned by the history PR.
type historyState struct {
	search, cachedSearch string
	list                 ui.ListState
	rows                 []historyRow
	keys                 map[string]int
	nextKey              int
	count                int
	newest               string
	day                  time.Time
	strings              i18n.History
	ready, confirmClear  bool
}

type historyRow struct {
	key               int
	header            bool
	entry             model.HistoryEntry
	label, status, at string
}

func (s *historyState) rowKey(i int) int      { return s.rows[i].key }
func (s *historyState) rowLabel(i int) string { return s.rows[i].label }
func (s *historyState) rowHeader(i int) bool  { return s.rows[i].header }

func (s *historyState) rebuild(entries []model.HistoryEntry, text i18n.History, now time.Time) {
	newest := ""
	if len(entries) > 0 {
		newest = entries[0].ID
	}
	localDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// History entries are immutable: Add changes the newest ID (even at the
	// limit), Delete changes the length, and Clear empties it. Cache filtering,
	// grouping and display strings by that cheap signature plus query, language
	// and local day. The service's List snapshot is still read on every paint.
	if s.ready && s.count == len(entries) && s.newest == newest && s.cachedSearch == s.search && s.day.Equal(localDay) && s.strings == text {
		return
	}
	s.ready, s.count, s.newest = true, len(entries), newest
	s.cachedSearch, s.day, s.strings = s.search, localDay, text
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
	today := now.Format(time.DateOnly)
	yesterday := now.AddDate(0, 0, -1).Format(time.DateOnly)
	lastDay := ""
	for _, entry := range entries {
		if !strings.Contains(strings.ToLower(entry.Request.URL), query) && !strings.Contains(strings.ToLower(string(entry.Request.Method)), query) {
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
		stamp := at.Format(text.DateFormat)
		if day == today {
			stamp = at.Format(text.TimeFormat)
		}
		status := strconv.Itoa(entry.StatusCode)
		if entry.Error != "" || entry.StatusCode == 0 {
			status = text.Error
		}
		s.rows = append(s.rows, historyRow{
			key: key("entry:" + entry.ID), entry: entry,
			label:  string(entry.Request.Method) + " " + entry.Request.URL,
			status: status, at: stamp,
		})
	}
	s.list.Key, s.list.Label, s.list.Header = s.rowKey, s.rowLabel, s.rowHeader
	s.list.Select(-1)
	s.list.ScrollTo(0, ui.ListStart)
}

// historyPane lists recently sent requests; choosing one opens it.
func (a *App) historyPane(c ui.C) {
	s, text := &a.history, a.S.History
	ui.Column(c).Fill().Gap(8).Children(func() {
		ui.TextField(c, &s.search, text.Search, text.SearchPlaceholder).FillWidth()
		s.rebuild(a.Svc.History.List(), text, time.Now())
		if ui.Button(c, text.Clear).Outline().Disabled(s.count == 0).Clicked() {
			s.confirmClear = true
		}
		if len(s.rows) == 0 {
			empty := text.Empty
			if s.count > 0 {
				empty = text.NoMatches
			}
			ui.Paragraph(c, empty, 0).Muted().FillWidth()
		} else {
			ui.List(c, &s.list, len(s.rows), a.historyRow).Label(text.Title).FillWidth().Grow(1)
			if i, ok := s.list.Submitted(); ok && i >= 0 && i < len(s.rows) && !s.rows[i].header {
				// Keyboard submission arrives in a regular repaint, where After
				// is ignored. Opening does not change the history list being drawn.
				a.OpenRequest(s.rows[i].entry.Request, "")
			}
		}
	})
	// This pane owns its modal because the shared dialogs hook has no history
	// callback. The modal traps focus, so the sidebar cannot change underneath it.
	ui.Modal(c, &s.confirmClear, text.ConfirmTitle, func() {
		ui.Paragraph(c, text.ConfirmBody, 0)
	}, func() {
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

func (a *App) historyRow(row ui.ListRow) {
	item, theme := a.history.rows[row.Index], row.C.Theme()
	if row.Header {
		row.Element.Background(theme.Card).PadX(8).PadY(8).Children(func() {
			ui.Text(row.C, item.label).Muted().Bold().FontSize(theme.Metrics.FontSizeS)
		})
		return
	}
	entry := item.entry
	box := row.Element.Pad(8).Gap(8).Items(ui.Center).Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(theme.Hover)).Pressed(ui.Look{}.Background(theme.Muted)).
		Focus(ui.Look{}.Ring(3, theme.Ring.WithAlpha(0.5)))
	if row.Selected() {
		box.Background(theme.Accent)
	}
	methodColor := theme.Primary
	switch entry.Request.Method {
	case model.MethodGet, model.MethodHead:
		methodColor = theme.Success
	case model.MethodPost:
		methodColor = theme.Chart[0]
	case model.MethodPut, model.MethodPatch:
		methodColor = theme.Chart[1]
	case model.MethodDelete:
		methodColor = theme.Danger
	case model.MethodOptions:
		methodColor = theme.Chart[2]
	}
	statusColor := theme.MutedForeground
	switch {
	case entry.Error != "" || entry.StatusCode == 0 || entry.StatusCode >= 500:
		statusColor = theme.Danger
	case entry.StatusCode >= 400:
		statusColor = theme.Chart[0]
	case entry.StatusCode >= 300:
		statusColor = theme.Chart[1]
	case entry.StatusCode >= 200:
		statusColor = theme.Success
	}
	box.Children(func() {
		ui.Text(row.C, string(entry.Request.Method)).FontSize(theme.Metrics.FontSizeS).Bold().
			Color(methodColor).Background(methodColor.WithAlpha(0.12)).PadX(4).PadY(4).
			Radius(theme.Metrics.RadiusControl).Shrink(0)
		ui.Column(row.C).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(row.C, entry.Request.URL).FillWidth().Ellipsis()
			ui.Row(row.C).FillWidth().Gap(8).Children(func() {
				ui.Text(row.C, item.status).Color(statusColor).FontSize(theme.Metrics.FontSizeS)
				ui.Spacer(row.C)
				ui.Text(row.C, item.at).Muted().FontSize(theme.Metrics.FontSizeS)
			})
		})
	})
	if box.Clicked() && !box.DoubleClicked() {
		row.C.After(func() { a.OpenRequest(entry.Request, "") })
	}
	ui.ContextMenu(row.C, box, func(menu *ui.MenuItems) {
		if menu.Item(a.S.History.Open).Clicked() {
			row.C.After(func() { a.OpenRequest(entry.Request, "") })
		}
		if menu.Item(a.S.History.Delete).Destructive().Clicked() {
			row.C.After(func() {
				if err := a.Svc.History.Delete(entry.ID); err != nil {
					a.Toast(a.S.History.Error, err.Error(), ui.ToastError)
				}
			})
		}
	})
}

// historyPage is the full-width History page (titlebar navigation).
// Until the history PR redesigns it, it shows the sidebar pane.
func (a *App) historyPage(c ui.C) {
	ui.Column(c).Fill().Pad(Space6).Children(func() { a.historyPane(c) })
}
