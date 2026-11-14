package gui

import (
	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// tabsState is the tab bar's UI state. Owned by the tabs PR.
type tabsState struct {
	confirm    bool
	revealedID int
	// Tab IDs keep pending closes stable as earlier tabs disappear.
	pending  []int
	menu     bool
	closed   []closedRequestTab
	drag     ui.DragState
	bindings []requestTabBinding
	drop     func(source, target gfx.ID)
}

type closedRequestTab struct {
	request      model.Request
	collectionID string
	dirty        bool
}

type requestTabBinding struct {
	hit gfx.ID
	id  int
}

const requestTabWidth float32 = 220

// tabBar declares the strip of open request tabs above the editor.
func (a *App) tabBar(c ui.C) {
	s := &a.tabBarState
	if s.drop == nil {
		s.drop = func(source, target gfx.ID) {
			var from, to int
			for _, binding := range s.bindings {
				if binding.hit == source {
					from = binding.id
				}
				if binding.hit == target {
					to = binding.id
				}
			}
			// Viva 0.6 calls drop outside declaration and outside an event
			// paint, where c.After is ignored. Moving here is already safe.
			a.reorderRequestTab(from, to)
		}
	}
	s.bindings = s.bindings[:0]
	if ui.Shortcut(c, ui.Primary, "t") {
		c.After(func() { a.NewTab() })
	}
	if ui.Shortcut(c, ui.Primary|ui.Shift, "t") {
		c.After(a.reopenRequestTab)
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
	ui.Column(c).FillWidth().Height(TabBarHeight).Background(a.palette.Surface).Children(func() {
		ui.Row(c).FillWidth().Grow(1).Items(ui.Center).Children(func() {
			area := ui.Scroll(c).Grow(1).FillHeight().Sideways()
			// Keep the selected tab visible after shortcuts, opens, and duplication.
			scroll := area.ScrollState()
			if a.tabBarState.revealedID != a.Active().ID && scroll.ViewportWidth > 0 {
				left := float32(a.active) * requestTabWidth
				right := left + requestTabWidth
				if a.active == len(a.Tabs())-1 {
					right += TabBarHeight - Space2
				}
				if left < scroll.OffsetX {
					scroll.ScrollToX(left)
				} else if right > scroll.OffsetX+scroll.ViewportWidth {
					scroll.ScrollToX(right - scroll.ViewportWidth)
				}
				a.tabBarState.revealedID = a.Active().ID
			}
			area.Children(func() {
				ui.Row(c).Height(TabBarHeight - 1).Items(ui.Center).Children(func() {
					for i, t := range a.Tabs() {
						tc := c.KeyInt(t.ID)
						title := a.requestTabTitle(t)
						background, foreground := a.palette.Surface, theme.MutedForeground
						if i == a.active {
							background, foreground = theme.Background, theme.Foreground
						}
						row := ui.Row(tc).Width(requestTabWidth).FillHeight().PadX(Space2).Gap(Space2).Items(ui.Center)
						row.Background(background).
							Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring))
						var id gfx.ID
						row.Children(func() {
							id = ui.ID(tc.Part("tab"))
							s.bindings = append(s.bindings, requestTabBinding{hit: id, id: t.ID})
							row.Hit(id, title).DragSource(&s.drag).DropTarget(&s.drag, nil, s.drop)
							if s.drag.Dragging() && s.drag.Target() == id {
								row.Background(theme.Accent)
							}
							if i == a.active {
								ui.Row(tc).Absolute().Top(0).Left(0).Right(0).Height(2).Background(theme.Primary)
							}
							a.MethodBadge(tc, t.Request.Method)
							ui.Text(tc, title).Grow(1).Ellipsis().Color(foreground)
							ui.Stack(tc).Size(Space6, Space6).Children(func() {
								close := ui.ButtonBase(tc, a.S.Tabs.Close).Tooltip(a.S.Tabs.Close).Size(Space6, Space6).
									Radius(theme.Metrics.RadiusControl).Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring))
								close.Children(func() {
									ui.Stack(tc).Fill().Center().Children(func() {
										if t.Dirty {
											ui.Row(tc).Fill().Center().StateFrom(row).Hover(ui.Look{}.Opacity(0)).Focus(ui.Look{}.Opacity(0)).Children(func() {
												ui.Text(tc, a.S.Tabs.DirtySymbol).FontSize(theme.Metrics.FontSizeS).Color(theme.Primary).
													StateFrom(close).Hover(ui.Look{}.Opacity(0)).Focus(ui.Look{}.Opacity(0))
											})
										}
										icon := ui.Icon(tc, icons.X).Size(Space4, Space4).Color(theme.MutedForeground)
										if i != a.active || t.Dirty {
											// Viva tracks hover on the innermost hit: follow both
											// the tab and the close control as the pointer enters it.
											icon.Opacity(0).StateFrom(row).Hover(ui.Look{}.Opacity(1)).Focus(ui.Look{}.Opacity(1))
											ui.Icon(tc, icons.X).Size(Space4, Space4).Color(theme.MutedForeground).Opacity(0).
												StateFrom(close).Hover(ui.Look{}.Opacity(1)).Focus(ui.Look{}.Opacity(1))
										}
									})
								})
								if close.Clicked() {
									c.After(func() { a.closeRequestTabs(t.ID, false) })
								}
							})
						})
						if canvas := c.Canvas(); canvas != nil {
							if event, ok := canvas.PointerOn(id); ok && event.Kind == gfx.PointerUp && event.Button == gfx.PointerButtonMiddle {
								canvas.Consume()
								c.After(func() { a.closeRequestTabs(t.ID, false) })
							}
						}
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
							if menu.Item(a.S.Tabs.CloseRight).Clicked() {
								c.After(func() {
									rightOf := a.requestTabIndex(t.ID)
									a.closeRequestTabsMatching(func(index int, _ *Tab) bool { return index > rightOf })
								})
							}
							if menu.Item(a.S.Tabs.CloseSaved).Clicked() {
								c.After(func() { a.closeRequestTabsMatching(func(_ int, tab *Tab) bool { return tab.CollectionID != "" }) })
							}
							menu.Separator()
							if menu.Item(a.S.Tabs.Duplicate).Clicked() {
								c.After(func() {
									copy := a.NewTab()
									copy.Request = t.Request.Clone()
									copy.Request.ID = storage.NewID()
									copy.CollectionID, copy.Dirty = t.CollectionID, true
								})
							}
							if menu.Item(a.S.Tabs.CopyURL).Disabled(t.Request.URL == "").Clicked() {
								c.After(func() {
									if err := app.SetClipboardText(t.Request.URL); err != nil {
										a.Toast(a.S.Tabs.CopyError, err.Error(), ui.ToastError)
									}
								})
							}
						})
					}
					if ui.Button(c, "").Icon(icons.Plus).Label(a.S.Tabs.New).Tooltip(a.S.Tabs.New).
						Ghost().IconOnly().Size(TabBarHeight-Space2, TabBarHeight-Space2).Clicked() {
						c.After(func() { a.NewTab() })
					}
				})
			})
			if scroll.MaxX() > 0 {
				more := ui.Button(c, "").Icon(icons.ChevronDown).Label(a.S.Tabs.AllTabs).Tooltip(a.S.Tabs.AllTabs).
					Ghost().IconOnly().Size(TabBarHeight-Space2, TabBarHeight-Space2)
				if more.Clicked() {
					s.menu = !s.menu
				}
				ui.Menu(c, more, &s.menu, func(menu *ui.MenuItems) {
					for i, tab := range a.Tabs() {
						if menu.Item(a.requestTabTitle(tab)).Description(string(tab.Request.Method)).Chosen(i == a.active).Clicked() {
							c.After(func() { a.active = a.requestTabIndex(tab.ID) })
						}
					}
				})
			}
		})
		ui.Row(c).FillWidth().Height(1).Background(theme.Border)
	})

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
	a.closeRequestTabsMatching(func(_ int, tab *Tab) bool {
		if others {
			return tab.ID != id
		}
		return tab.ID == id
	})
}

func (a *App) closeRequestTabsMatching(match func(int, *Tab) bool) {
	s := &a.tabBarState
	s.pending = s.pending[:0]
	for i, tab := range a.Tabs() {
		if match(i, tab) {
			s.pending = append(s.pending, tab.ID)
		}
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
		tab := a.Tabs()[i]
		s := &a.tabBarState
		if len(s.closed) == 10 {
			s.closed = append(s.closed[:0], s.closed[1:]...)
		}
		s.closed = append(s.closed, closedRequestTab{request: tab.Request.Clone(), collectionID: tab.CollectionID, dirty: tab.Dirty})
		a.Cancel(a.Tabs()[i])
		a.CloseTab(i)
	}
}

func (a *App) reopenRequestTab() {
	s := &a.tabBarState
	if len(s.closed) == 0 {
		return
	}
	last := s.closed[len(s.closed)-1]
	s.closed[len(s.closed)-1] = closedRequestTab{}
	s.closed = s.closed[:len(s.closed)-1]
	tab := a.NewTab()
	tab.Request, tab.CollectionID, tab.Dirty = last.request, last.collectionID, last.dirty
	a.Main = PageWorkbench
}

func (a *App) reorderRequestTab(fromID, toID int) {
	from, to := a.requestTabIndex(fromID), a.requestTabIndex(toID)
	if from < 0 || to < 0 || from == to {
		return
	}
	active, moved := a.Active(), a.tabs[from]
	if from < to {
		copy(a.tabs[from:to], a.tabs[from+1:to+1])
	} else {
		copy(a.tabs[to+1:from+1], a.tabs[to:from])
	}
	a.tabs[to] = moved
	a.active = a.requestTabIndex(active.ID)
	a.tabBarState.revealedID = 0
}

// OpenRequest shows req (from history or a collection) for editing.
// collectionID is where it is stored, or "" when it is not saved.
func (a *App) OpenRequest(req model.Request, collectionID string) {
	a.Main = PageWorkbench
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
