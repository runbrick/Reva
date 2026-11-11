package gui

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// searchState is the global search's UI state. Owned by the search PR.
type searchState struct {
	open, submit, reset bool
	query, indexedQuery string
	rows                []searchResult
	selected            int
	list                ui.ListState
	key                 func(gfx.KeyEvent) gfx.EventResult
}

type searchKind uint8

const (
	searchRequest searchKind = iota
	searchTab
	searchHistory
	searchEnvironment
	searchCommand
)

type searchAction uint8

const (
	searchNewRequest searchAction = iota
	searchNewCollection
	searchImport
	searchSettings
	searchToggleTheme
	searchSwitchLanguage
)

// Match segments are prepared only when the query changes, including Unicode
// byte boundaries, so painting visible rows does not split or lowercase text.
type searchText struct{ before, match, after string }

type searchResult struct {
	kind                        searchKind
	header, prefix              bool
	name, detail                string
	nameText, detailText        searchText
	request                     model.Request
	collectionID, environmentID string
	tabID                       int
	action                      searchAction
}

func (s *searchState) rowLabel(i int) string { return s.rows[i].name }
func (s *searchState) rowHeader(i int) bool  { return s.rows[i].header }

// searchBox declares the titlebar search field (opens the palette).
// Owned by the search PR.
func (a *App) searchBox(c ui.C) {
	theme := c.Theme()
	hint := a.S.Search.OtherHint
	if a.Platform == "darwin" {
		hint = a.S.Search.MacHint
	}
	box := ui.Row(c).Width(320).Height(32).Shrink(0).PadX(Space3).Gap(Space2).Items(ui.Center).
		Background(theme.Muted).Radius(theme.Metrics.RadiusControl).
		Hit(ui.ID(c.Part("global-search")), a.S.Search.Title).
		Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring)).Children(func() {
		ui.Icon(c, icons.Search).Size(Space4, Space4).Color(theme.MutedForeground)
		ui.Text(c, a.S.Search.Placeholder).FontSize(theme.Metrics.FontSizeS).Muted().Grow(1).Ellipsis()
		ui.Text(c, hint).FontSize(theme.Metrics.FontSizeS).Muted().Background(theme.Card).
			PadX(Space1).Radius(theme.Metrics.RadiusControl).Shrink(0)
	})
	if box.Clicked() {
		a.openSearch()
	}
}

func (a *App) openSearch() {
	s := &a.search
	if s.open {
		return
	}
	s.open, s.query, s.submit = true, "", false
	if s.key == nil {
		s.list.Label, s.list.Header, s.list.NoTypeahead = s.rowLabel, s.rowHeader, true
		s.key = s.onKey
	}
	a.indexSearch()
}

// onKey keeps typing in the field while the arrows move the virtual list.
// Running an action is deferred to the page, after its declarations finish.
func (s *searchState) onKey(event gfx.KeyEvent) gfx.EventResult {
	if event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyUp, gfx.KeyDown:
		delta := 1
		if event.Key == gfx.KeyUp {
			delta = -1
		}
		for i := s.selected + delta; i >= 0 && i < len(s.rows); i += delta {
			if !s.rows[i].header {
				s.selected = i
				s.list.Select(i)
				first, last := s.list.Visible()
				if i <= first || i >= last {
					s.list.ScrollTo(i, ui.ListCenter)
				}
				break
			}
		}
	case gfx.KeyEnter:
		return gfx.EventResult{Handled: true, Activate: event.Target}
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// searchPalette declares the search/command palette overlay (Primary+K).
// Owned by the search PR.
func (a *App) searchPalette(c ui.C) {
	if ui.Shortcut(c, ui.Primary, "k") {
		a.openSearch()
	}
	s := &a.search
	if !s.open {
		return
	}
	c = c.Part("search-palette")
	// A roomy fixed-height dialog puts the search input near the top of the
	// window and avoids the surface jumping while the results are filtered.
	dialog := ui.Dialog(c, ui.DialogSpec{
		Title: a.S.Search.Title, Width: 640, Height: min(640, c.Bounds().Height-2*Space6),
		Scope: ui.ID(c.Part("scope")), DismissID: ui.ID(c.Part("dismiss")),
		CloseID: ui.ID(c.Part("close")), NoClose: true,
	}).Body(func() {
		ui.Column(c).Fill().Gap(Space3).Children(func() {
			field := ui.TextField(c, &s.query, a.S.Search.Field, a.S.Search.Placeholder).
				Icon(icons.Search).AutoFocus().FillWidth().OnKey(s.key)
			if field.Submitted() {
				s.submit = true
			}
			if s.query != s.indexedQuery {
				a.indexSearch()
			}
			if len(s.rows) == 0 {
				ui.Text(c, a.S.Search.Empty).Muted()
				return
			}
			ui.List(c, &s.list, len(s.rows), func(row ui.ListRow) {
				r := &s.rows[row.Index]
				if r.header {
					row.Element.Height(Space6 + Space2).PadX(Space2).Items(ui.Center).
						Background(c.Theme().Popover).Children(func() {
						ui.Text(row.C, r.name).FontSize(c.Theme().Metrics.FontSizeS).Muted()
					})
					return
				}
				theme := c.Theme()
				box := row.Element.Height(2*Space6 + Space2).PadX(Space2).Gap(Space2).Items(ui.Center).
					Radius(theme.Metrics.RadiusControl).Hover(ui.Look{}.Background(theme.Hover))
				if row.Index == s.selected || row.Selected() {
					box.Background(theme.Accent)
				}
				box.Children(func() {
					if r.request.Method != "" {
						a.MethodBadge(row.C, r.request.Method)
					} else if r.kind == searchEnvironment {
						ui.Icon(row.C, icons.Globe).Size(Space4, Space4).Color(theme.MutedForeground)
					} else {
						ui.Icon(row.C, icons.Search).Size(Space4, Space4).Color(theme.MutedForeground)
					}
					ui.Column(row.C).Grow(1).Gap(Space1).Children(func() {
						a.searchHighlight(row.C, r.nameText, false)
						if r.detail != "" {
							a.searchHighlight(row.C, r.detailText, true)
						}
					})
				})
				if box.Clicked() {
					s.selected, s.submit = row.Index, true
				}
			}).FillWidth().Grow(1).Label(a.S.Search.Results)
			if s.reset {
				s.list.Select(s.selected)
				s.list.ScrollTo(0, ui.ListStart)
				s.reset = false
			} else if s.list.Changed() {
				s.selected = s.list.Selected()
			}
			if i, ok := s.list.Submitted(); ok {
				s.selected, s.submit = i, true
			}
		})
	}).Footer(func() { ui.Text(c, a.S.Search.Navigation).FontSize(c.Theme().Metrics.FontSizeS).Muted() })
	if dialog.Dismissed() {
		s.open, s.submit = false, false
	}
	if s.submit {
		s.submit = false
		if s.selected >= 0 && s.selected < len(s.rows) && !s.rows[s.selected].header {
			r := s.rows[s.selected]
			c.After(func() { a.runSearch(r) })
		}
	}
}

func (a *App) searchHighlight(c ui.C, text searchText, muted bool) {
	theme := c.Theme()
	color := theme.Foreground
	if muted {
		color = theme.MutedForeground
	}
	ui.Row(c).FillWidth().Children(func() {
		if text.before != "" {
			ui.Text(c, text.before).Color(color).FontSize(theme.Metrics.FontSizeS).Ellipsis()
		}
		if text.match != "" {
			ui.Text(c, text.match).Color(theme.Primary).Background(theme.Accent).
				FontSize(theme.Metrics.FontSizeS).Bold().Ellipsis()
		}
		if text.after != "" {
			ui.Text(c, text.after).Color(color).FontSize(theme.Metrics.FontSizeS).Ellipsis()
		}
	})
}

func searchMatch(text, query string) (searchText, bool, bool) {
	if query == "" {
		return searchText{before: text}, true, false
	}
	lower := strings.ToLower(text)
	i := strings.Index(lower, query)
	if i < 0 {
		return searchText{before: text}, false, false
	}
	start, end := utf8.RuneCountInString(lower[:i]), utf8.RuneCountInString(lower[:i+len(query)])
	runes := []rune(text)
	return searchText{before: string(runes[:start]), match: string(runes[start:end]), after: string(runes[end:])}, true, i == 0
}

func (a *App) indexSearch() {
	s, text := &a.search, a.S.Search
	s.indexedQuery = s.query
	query := strings.ToLower(strings.TrimSpace(s.query))
	results := s.rows[:0]
	add := func(r searchResult) {
		var nameOK, detailOK, namePrefix, detailPrefix bool
		r.nameText, nameOK, namePrefix = searchMatch(r.name, query)
		r.detailText, detailOK, detailPrefix = searchMatch(r.detail, query)
		url := strings.ToLower(r.request.URL)
		requestName := strings.ToLower(r.request.Name)
		if nameOK || detailOK || strings.Contains(requestName, query) || strings.Contains(url, query) {
			r.prefix = namePrefix || detailPrefix || query != "" && (strings.HasPrefix(url, query) || strings.HasPrefix(requestName, query))
			results = append(results, r)
		}
	}
	requestName := func(r model.Request) string {
		if r.Name != "" {
			return r.Name
		}
		if r.URL != "" {
			return r.URL
		}
		return text.Untitled
	}
	var items func([]model.Item, string, string)
	items = func(nodes []model.Item, collectionID, path string) {
		for _, item := range nodes {
			if item.Request != nil {
				r := *item.Request
				name := item.Name
				if name == "" {
					name = requestName(r)
				}
				add(searchResult{kind: searchRequest, name: name, detail: path + " · " + r.URL, request: r, collectionID: collectionID})
			}
			if len(item.Items) > 0 {
				items(item.Items, collectionID, path+" / "+item.Name)
			}
		}
	}
	for _, col := range a.Svc.Collections.List() {
		items(col.Items, col.ID, col.Name)
	}
	for _, tab := range a.Tabs() {
		add(searchResult{kind: searchTab, name: requestName(tab.Request), detail: tab.Request.URL, request: tab.Request, tabID: tab.ID})
	}
	seen := make(map[string]bool)
	for _, entry := range a.Svc.History.List() {
		if len(seen) == 50 {
			break
		}
		if seen[entry.Request.URL] {
			continue
		}
		seen[entry.Request.URL] = true
		add(searchResult{kind: searchHistory, name: requestName(entry.Request), detail: entry.Request.URL, request: entry.Request})
	}
	for _, env := range a.Svc.Environments.List() {
		add(searchResult{kind: searchEnvironment, name: env.Name, environmentID: env.ID})
	}
	commands := [...]string{text.NewRequest, text.NewCollection, text.Import, text.Settings, text.ToggleTheme, text.SwitchLanguage}
	for i, name := range commands {
		add(searchResult{kind: searchCommand, name: name, action: searchAction(i)})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].kind != results[j].kind {
			return results[i].kind < results[j].kind
		}
		return results[i].prefix && !results[j].prefix
	})
	// Insert headers backwards in the reused slice so source rows are not
	// overwritten before they have been copied.
	groups := 0
	for i, r := range results {
		if i == 0 || r.kind != results[i-1].kind {
			groups++
		}
	}
	n := len(results)
	results = append(results, make([]searchResult, groups)...)
	labels := [...]string{text.Requests, text.Tabs, text.History, text.Environments, text.Commands}
	for i, at := n-1, len(results)-1; i >= 0; i-- {
		r := results[i]
		results[at] = r
		at--
		if i == 0 || r.kind != results[i-1].kind {
			results[at] = searchResult{kind: r.kind, header: true, name: labels[r.kind]}
			at--
		}
	}
	s.rows, s.selected, s.reset = results, -1, true
	for i, r := range s.rows {
		if !r.header {
			s.selected = i
			break
		}
	}
}

func (a *App) runSearch(r searchResult) {
	a.search.open = false
	switch r.kind {
	case searchRequest, searchHistory:
		a.OpenRequest(r.request, r.collectionID)
		a.Main = PageWorkbench
	case searchTab:
		for i, tab := range a.Tabs() {
			if tab.ID == r.tabID {
				a.active, a.Main = i, PageWorkbench
				break
			}
		}
	case searchEnvironment:
		settings := a.Svc.Settings.Get()
		settings.ActiveEnvID = r.environmentID
		a.saveSearchSettings(settings)
	case searchCommand:
		switch r.action {
		case searchNewRequest:
			a.NewTab()
			a.Main = PageWorkbench
		case searchNewCollection:
			a.SelectSection(SectionCollections)
			a.collectionNameDialog(collectionCreate, collectionNode{})
		case searchImport:
			a.importer.open, a.importer.err = true, ""
		case searchSettings:
			a.SelectSection(SectionSettings)
		case searchToggleTheme:
			settings := a.Svc.Settings.Get()
			settings.Theme = "dark"
			if a.Dark() {
				settings.Theme = "light"
			}
			a.saveSearchSettings(settings)
		case searchSwitchLanguage:
			settings := a.Svc.Settings.Get()
			settings.Language = "en"
			if a.S.Lang == "en" {
				settings.Language = "zh"
			}
			a.saveSearchSettings(settings)
		}
	}
}

func (a *App) saveSearchSettings(settings model.Settings) {
	if err := a.Svc.Settings.Save(settings); err != nil {
		a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
		return
	}
	a.ApplySettings(settings)
}
