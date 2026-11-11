package gui

import (
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/vivaui/viva/ui"
)

// tabsState is the tab bar's UI state. Owned by the tabs PR.
type tabsState struct {
	confirm    bool
	revealedID int
	// Tab IDs keep pending closes stable as earlier tabs disappear.
	pending []int
}

const requestTabWidth float32 = 232

// tabBar declares the strip of open request tabs above the editor.
func (a *App) tabBar(c ui.C) {
	if ui.Shortcut(c, ui.Primary, "t") {
		c.After(func() { a.NewTab() })
	}
	if ui.Shortcut(c, ui.Primary, "w") {
		id := a.Active().ID
		c.After(func() { a.closeRequestTabs(id, false) })
	}
	if ui.Shortcut(c, ui.Ctrl, "tab") {
		a.active = (a.active + 1) % len(a.Tabs())
	}
	if ui.Shortcut(c, ui.Ctrl|ui.Shift, "tab") {
		a.active = (a.active + len(a.Tabs()) - 1) % len(a.Tabs())
	}

	theme := c.Theme()
	ui.Row(c).FillWidth().Height(48).Gap(4).PadX(4).Items(ui.Center).Background(theme.Muted).Children(func() {
		area := ui.Scroll(c).Grow(1).Height(48).Sideways().Scrollbars()
		// Keep the selected tab visible after shortcuts, opens, and duplication.
		scroll := area.ScrollState()
		if a.tabBarState.revealedID != a.Active().ID && scroll.ViewportWidth > 0 {
			left := float32(a.active) * requestTabWidth
			if left < scroll.OffsetX {
				scroll.ScrollToX(left)
			} else if left+requestTabWidth > scroll.OffsetX+scroll.ViewportWidth {
				scroll.ScrollToX(left + requestTabWidth - scroll.ViewportWidth)
			}
			a.tabBarState.revealedID = a.Active().ID
		}
		area.Children(func() {
			ui.Row(c).Height(40).Children(func() {
				for i, t := range a.Tabs() {
					tc := c.KeyInt(t.ID)
					title := a.requestTabTitle(t)
					background, border := theme.Muted, theme.Border
					if i == a.active {
						background, border = theme.Card, theme.Primary
					}
					row := ui.Row(tc).Width(requestTabWidth).Height(40).PadX(8).Gap(8).Items(ui.Center).
						Background(background).Border(1, border).Radius(theme.Metrics.RadiusControl).
						Hit(ui.ID(tc), title).Children(func() {
						ui.Text(tc, string(t.Request.Method)).FontSize(12).Bold().Color(theme.Primary).
							Background(theme.Muted).PadX(4).PadY(2).Radius(theme.Metrics.RadiusControl)
						ui.Text(tc, title).Grow(1).Ellipsis()
						if t.Dirty {
							ui.Text(tc, a.S.Tabs.DirtySymbol).FontSize(12).Color(theme.Primary)
						}
						if ui.Button(tc, a.S.Tabs.CloseSymbol).Label(a.S.Tabs.Close).Tooltip(a.S.Tabs.Close).Ghost().Small().Width(24).Clicked() {
							c.After(func() { a.closeRequestTabs(t.ID, false) })
						}
					})
					if row.Clicked() {
						c.After(func() { a.active = i })
					}
					ui.ContextMenu(tc, row, func(menu *ui.MenuItems) {
						if menu.Item(a.S.Tabs.Close).Clicked() {
							c.After(func() { a.closeRequestTabs(t.ID, false) })
						}
						if menu.Item(a.S.Tabs.CloseOthers).Clicked() {
							c.After(func() { a.closeRequestTabs(t.ID, true) })
						}
						if menu.Item(a.S.Tabs.Duplicate).Clicked() {
							c.After(func() {
								copy := a.NewTab()
								copy.Request = t.Request.Clone()
								copy.Request.ID = storage.NewID()
								copy.CollectionID, copy.Dirty = t.CollectionID, true
							})
						}
					})
				}
			})
		})
		if ui.Button(c, a.S.Tabs.NewSymbol).Label(a.S.Tabs.New).Tooltip(a.S.Tabs.New).Ghost().Width(32).Clicked() {
			c.After(func() { a.NewTab() })
		}
	})

	s := &a.tabBarState
	ui.Modal(c, &s.confirm, a.S.Tabs.ConfirmTitle, func() {
		ui.Wrap(c, a.S.Tabs.ConfirmDiscard)
		if len(s.pending) > 0 {
			if i := a.requestTabIndex(s.pending[0]); i >= 0 {
				ui.Text(c, a.requestTabTitle(a.Tabs()[i])).Ellipsis()
			}
		}
	}, func() {
		if ui.Button(c, a.S.Tabs.Cancel).Outline().Clicked() {
			s.confirm = false
		}
		if ui.Button(c, a.S.Tabs.Discard).Destructive().Clicked() {
			c.After(func() {
				s.confirm = false
				if len(s.pending) > 0 {
					a.discardRequestTab(s.pending[0])
					s.pending = s.pending[1:]
				}
				a.continueClosingTabs()
			})
		}
	})
	if !s.confirm {
		s.pending = s.pending[:0]
	}
}

func (a *App) requestTabTitle(t *Tab) string {
	if t.Request.Name != "" {
		return t.Request.Name
	}
	if t.Request.URL != "" {
		return t.Request.URL
	}
	return a.S.Tabs.Title
}

func (a *App) requestTabIndex(id int) int {
	for i, t := range a.Tabs() {
		if t.ID == id {
			return i
		}
	}
	return -1
}

func (a *App) closeRequestTabs(id int, others bool) {
	s := &a.tabBarState
	s.pending = s.pending[:0]
	if others {
		for _, t := range a.Tabs() {
			if t.ID != id {
				s.pending = append(s.pending, t.ID)
			}
		}
	} else {
		s.pending = append(s.pending, id)
	}
	a.continueClosingTabs()
}

func (a *App) continueClosingTabs() {
	s := &a.tabBarState
	for len(s.pending) > 0 {
		id := s.pending[0]
		if i := a.requestTabIndex(id); i >= 0 && a.Tabs()[i].Dirty {
			s.confirm = true
			return
		}
		a.discardRequestTab(id)
		s.pending = s.pending[1:]
	}
	s.confirm = false
}

func (a *App) discardRequestTab(id int) {
	if i := a.requestTabIndex(id); i >= 0 {
		a.Cancel(a.Tabs()[i])
		a.CloseTab(i)
	}
}

// OpenRequest shows req (from history or a collection) for editing.
// collectionID is where it is stored, or "" when it is not saved.
func (a *App) OpenRequest(req model.Request, collectionID string) {
	if req.ID != "" {
		for i, t := range a.Tabs() {
			if t.Request.ID == req.ID {
				a.active = i
				return
			}
		}
	}
	t := a.Active()
	if t.Request.URL != "" || t.Dirty || t.Response != nil || t.Sending {
		t = a.NewTab()
	}
	t.Request = req.Clone()
	t.CollectionID = collectionID
	t.Response, t.Err, t.Tests, t.Dirty = nil, "", nil, false
}
