package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/ui"
)

type collectionSettingsSection uint8

const (
	collectionSettingsGeneral collectionSettingsSection = iota
	collectionSettingsAuthentication
	collectionSettingsVariables
)

// Each open target owns an independent draft. The baseline contains only fields
// this editor can change, never a stale collection tree to write back on Save.
type collectionSettingsDraft struct {
	section                   collectionSettingsSection
	name, savedName           string
	auth, savedAuth           model.Auth
	variables, savedVariables []model.KV
	editor                    authEditorState
	table                     kvTableState
	missing, saveFailed       bool
}

func (a *App) collectionSettingsTarget(t *Tab) (model.Collection, *model.Item, bool) {
	col, ok := a.Svc.Collections.Get(t.CollectionID)
	if !ok {
		return col, nil, false
	}
	if t.TargetID == "" {
		return col, nil, true
	}
	item := model.FindItem(col.Items, t.TargetID)
	return col, item, item != nil && item.Type == model.ItemFolder
}

func (a *App) openCollectionSettings(n collectionNode, section collectionSettingsSection) *Tab {
	a.Main, a.Section = PageWorkbench, SectionAPIs
	for i, t := range a.tabs {
		if t.Kind == collectionSettingsTabKind && t.CollectionID == n.collectionID && t.TargetID == n.id {
			a.active = i
			a.collections.settings[t.ID].section = section
			return t
		}
	}
	a.nextID++
	t := &Tab{ID: a.nextID, Kind: collectionSettingsTabKind, CollectionID: n.collectionID, TargetID: n.id}
	if a.collections.settings == nil {
		a.collections.settings = make(map[int]*collectionSettingsDraft)
	}
	a.collections.settings[t.ID] = &collectionSettingsDraft{section: section}
	a.reloadCollectionSettings(t)
	a.tabs = append(a.tabs, t)
	a.active = len(a.tabs) - 1
	return t
}

func (a *App) reloadCollectionSettings(t *Tab) {
	d := a.collections.settings[t.ID]
	col, item, ok := a.collectionSettingsTarget(t)
	d.missing = !ok
	if !ok {
		return // retain a deleted target's draft; never recreate it
	}
	d.name, d.auth = col.Name, col.Auth
	d.variables = slices.Clone(col.Variables)
	if item != nil {
		d.name, d.auth, d.variables = item.Name, item.Auth, nil
	}
	d.savedName, d.savedAuth = d.name, d.auth
	d.savedVariables = slices.Clone(d.variables)
	d.editor, d.table = authEditorState{}, kvTableState{}
	d.saveFailed, t.Dirty = false, false
}

func (a *App) syncCollectionSettingsDirty(t *Tab) {
	if t.Kind != collectionSettingsTabKind {
		return
	}
	d := a.collections.settings[t.ID]
	t.Dirty = d.name != d.savedName || d.auth != d.savedAuth || !slices.Equal(d.variables, d.savedVariables)
}

func (a *App) saveCollectionSettings(t *Tab) {
	d := a.collections.settings[t.ID]
	col, item, ok := a.collectionSettingsTarget(t)
	d.missing = !ok
	if !ok || strings.TrimSpace(d.name) == "" {
		return
	}
	a.syncCollectionSettingsDirty(t)
	if !t.Dirty {
		return
	}
	if item == nil {
		if d.name != d.savedName {
			col.Name = strings.TrimSpace(d.name)
		}
		if d.auth != d.savedAuth {
			col.Auth = d.auth
		}
		if !slices.Equal(d.variables, d.savedVariables) {
			col.Variables = slices.Clone(d.variables)
		}
	} else {
		col.Items = cloneCollectionItems(col.Items)
		item = model.FindItem(col.Items, t.TargetID)
		if d.name != d.savedName {
			item.Name = strings.TrimSpace(d.name)
		}
		if d.auth != d.savedAuth {
			item.Auth = d.auth
		}
	}
	if err := a.Svc.Collections.Save(col); err != nil {
		d.saveFailed = true
		return // backend errors may contain credentials; show fixed localized text
	}
	a.reloadCollectionSettings(t)
	a.refreshCollections()
	a.Toast(a.S.Collections.SettingsSaved, "", ui.ToastSuccess)
}

func (a *App) collectionSettingsName(t *Tab) string {
	d := a.collections.settings[t.ID]
	name := d.savedName
	col, item, ok := a.collectionSettingsTarget(t)
	if ok {
		name = col.Name
		if item != nil {
			name = item.Name
		}
	}
	return name
}

func (a *App) collectionSettingsTitle(t *Tab) string {
	format := a.S.Collections.SettingsTitle
	if t.TargetID != "" {
		format = a.S.Collections.FolderSettingsTitle
	}
	return fmt.Sprintf(format, a.collectionSettingsName(t))
}

func (a *App) collectionSettingsSectionLabel(section collectionSettingsSection) string {
	switch section {
	case collectionSettingsAuthentication:
		return a.S.Collections.Authentication
	case collectionSettingsVariables:
		return a.S.Collections.Variables
	default:
		return a.S.Collections.General
	}
}

func (a *App) collectionSettingsPane(c ui.C, t *Tab) {
	d, labels, theme := a.collections.settings[t.ID], &a.S.Collections, c.Theme()
	col, item, ok := a.collectionSettingsTarget(t)
	d.missing = !ok
	a.syncCollectionSettingsDirty(t)
	width := max(c.Bounds().Width-RailWidth-a.sideSplit.Pos-Space2, 0)
	ui.Column(c).Width(width).Grow(1).MinHeight(0).PadX(Space4).PadTop(Space2).Gap(Space2).Children(func() {
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			ui.Icon(c, icons.Folder).Size(Space4, Space4).Color(theme.MutedForeground)
			if d.section == collectionSettingsGeneral {
				ui.TextField(c, &d.name, labels.Name, "").Grow(1).MinWidth(0).MaxWidth(15 * Space6).Disabled(d.missing)
				ui.Row(c).Basis(0).Grow(1)
			} else {
				ui.Text(c, a.collectionSettingsName(t)).Bold().Ellipsis().Grow(1).MinWidth(0)
			}
			if t.Dirty {
				ui.Text(c, labels.Unsaved).Color(theme.Primary).FontSize(theme.Metrics.FontSizeS)
			}
			if ui.Button(c, labels.DiscardReload).Small().Ghost().Tooltip(labels.DiscardHelp).Shrink(0).Disabled(d.missing).Clicked() {
				c.After(func() { a.reloadCollectionSettings(t) })
			}
			if ui.Button(c, labels.Save).Small().Icon(icons.Save).Shrink(0).Disabled(d.missing || !t.Dirty || strings.TrimSpace(d.name) == "").Clicked() {
				c.After(func() { a.saveCollectionSettings(t) })
			}
		})
		if d.missing {
			ui.Wrap(c, labels.SettingsMissing).Color(theme.Danger)
		}
		if d.saveFailed {
			ui.Wrap(c, labels.SettingsSaveFailed).Color(theme.Danger)
		}
		sections := []collectionSettingsSection{collectionSettingsGeneral}
		if t.TargetID == "" {
			sections = append(sections, collectionSettingsVariables)
		}
		sections = append(sections, collectionSettingsAuthentication)
		ui.Row(c).FillWidth().Gap(Space2).FocusGroup(ui.FocusHorizontal).Children(func() {
			for _, section := range sections {
				tab := ui.TabBase(c.KeyInt(int(section)), &d.section, section, a.collectionSettingsSectionLabel(section)).
					Height(theme.Metrics.ControlHeight + Space2).PadX(Space2).
					Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring))
				fg := theme.MutedForeground
				if d.section == section {
					fg = theme.Primary
				}
				tab.Children(func() {
					ui.Text(c, a.collectionSettingsSectionLabel(section)).Color(fg)
					if d.section == section {
						ui.Row(c).Absolute().Left(0).Right(0).Bottom(0).Height(Space1 / 2).Background(theme.Primary)
					}
				})
			}
		})
		ui.Divider(c)
		if d.section == collectionSettingsGeneral {
			a.collectionOverview(c, t, col, item, ok)
			return
		}
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Scrollbars().Label(labels.SettingsContent).Children(func() {
			ui.Column(c).FillWidth().Gap(Space3).Children(func() {
				switch d.section {
				case collectionSettingsAuthentication:
					folder := t.TargetID != ""
					a.authEditor(c, &d.auth, &d.editor, folder, folder)
					if folder && (d.auth.Type == "" || d.auth.Type == model.AuthInherit) && ok {
						a.authPreview(c, &d.editor, col, t.TargetID)
					}
				case collectionSettingsVariables:
					if t.TargetID == "" {
						a.kvTable(c, &d.table, &d.variables)
					}
				}
			})
		})
	})
	a.syncCollectionSettingsDirty(t)
	if ui.Shortcut(c, ui.Primary, "s") {
		c.After(func() { a.saveCollectionSettings(t) })
	}
}

// Count saved request nodes, including HTTP GraphQL bodies, but never folders.
// The overview and tree use the same definition and never include editor drafts.
func collectionRequestCounts(items []model.Item) (http, websocket int) {
	for _, item := range items {
		if item.Type == model.ItemRequest && item.Request != nil {
			if isWebSocket(*item.Request) {
				websocket++
			} else {
				http++
			}
		}
		h, w := collectionRequestCounts(item.Items)
		http, websocket = http+h, websocket+w
	}
	return
}

func (a *App) collectionOverview(c ui.C, t *Tab, col model.Collection, item *model.Item, exists bool) {
	// C.Bounds is the window, so subtract the shell's actual sidebar allocation.
	// Wide panes have independent scroll areas; narrow panes keep one reading order.
	width := c.Bounds().Width - RailWidth - a.sideSplit.Pos - Space2 - 2*Space4
	if width < 30*Space6 {
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Scrollbars().Label(a.S.Collections.SettingsContent).Children(func() {
			ui.Column(c).FillWidth().Gap(Space6).PadBottom(Space4).Children(func() {
				a.collectionOverviewActions(c, t, col, item, exists)
				a.collectionOverviewGuide(c, t.TargetID != "")
			})
		})
		return
	}
	ui.Row(c).FillWidth().Grow(1).MinHeight(0).Gap(2 * Space6).Children(func() {
		ui.Scroll(c.Key("api")).Basis(0).Grow(1).MinWidth(0).FillHeight().Scrollbars().Label(a.S.Collections.APISection).Children(func() {
			a.collectionOverviewActions(c, t, col, item, exists)
		})
		ui.Scroll(c.Key("guide")).Basis(0).Grow(1).MinWidth(0).FillHeight().Scrollbars().Label(a.S.Collections.GuideSection).Children(func() {
			a.collectionOverviewGuide(c, t.TargetID != "")
		})
	})
}

func (a *App) collectionOverviewActions(c ui.C, t *Tab, col model.Collection, item *model.Item, exists bool) {
	s, theme := a.S.Collections, c.Theme()
	items := col.Items
	if item != nil {
		items = item.Items
	}
	http, ws := collectionRequestCounts(items)
	n := collectionNode{collectionID: t.CollectionID, id: t.TargetID, branch: true}
	ui.Column(c).FillWidth().Gap(Space2).PadBottom(Space4).Children(func() {
		ui.Text(c, s.APISection).Muted().FontSize(theme.Metrics.FontSizeS)
		ui.Divider(c)
		if t.TargetID == "" && ui.Button(c, s.RunCollection).Small().Outline().Icon(icons.Play).Self(ui.Start).Disabled(!exists).Clicked() {
			c.After(func() { a.openCollectionRunner(t.CollectionID) })
		}
		for i, count := range []int{http, ws} {
			ui.Row(c.KeyInt(i)).FillWidth().PadY(Space2).Gap(Space3).Items(ui.Center).Children(func() {
				icon, protocol, action := icons.Send, "HTTP", s.NewRequest
				if i == 1 {
					icon, protocol, action = icons.Activity, "WebSocket", s.NewWebSocket
				}
				ui.Icon(c, icon).Size(Space6, Space6).Color(theme.MutedForeground)
				ui.Column(c).Grow(1).MinWidth(0).Gap(Space1).Children(func() {
					ui.Text(c, protocol)
					ui.Text(c, fmt.Sprintf(s.RequestCount, count)).Muted().FontSize(theme.Metrics.FontSizeS)
				})
				if ui.Button(c, action).Icon(icons.Plus).IconOnly().Ghost().Disabled(!exists).Clicked() {
					c.After(func() {
						if i == 1 {
							a.newWebSocket(n)
						} else {
							a.newCollectionRequest(n)
						}
					})
				}
			})
		}
		ui.Text(c, s.OrganizeSection).Muted().FontSize(theme.Metrics.FontSizeS).PadTop(Space4)
		ui.Divider(c)
		ui.Row(c).FillWidth().PadY(Space2).Gap(Space3).Items(ui.Center).Children(func() {
			ui.Icon(c, icons.Folder).Size(Space6, Space6).Color(theme.MutedForeground)
			ui.Text(c, s.NewFolder).Grow(1)
			if ui.Button(c, s.NewFolder).Icon(icons.Plus).IconOnly().Ghost().Disabled(!exists).Clicked() {
				c.After(func() { a.collectionNameDialog(collectionFolder, n) })
			}
		})
		if t.TargetID == "" {
			ui.Text(c, s.ExportSection).Muted().FontSize(theme.Metrics.FontSizeS).PadTop(Space4)
			ui.Divider(c)
			ui.Row(c).FillWidth().PadY(Space2).Gap(Space3).Items(ui.Center).Children(func() {
				ui.Icon(c, icons.Download).Size(Space6, Space6).Color(theme.MutedForeground)
				ui.Column(c).Grow(1).MinWidth(0).Gap(Space1).Children(func() {
					ui.Text(c, s.NativeJSON)
					ui.Wrap(c, s.NativeJSONHelp).Muted().FontSize(theme.Metrics.FontSizeS)
				})
				if ui.Button(c, s.Export).Icon(icons.ChevronRight).IconOnly().Ghost().Disabled(!exists).Clicked() {
					c.After(func() { a.exportCollection(t.CollectionID) })
				}
			})
		}
	})
}

func (a *App) collectionOverviewGuide(c ui.C, folder bool) {
	s, theme := a.S.Collections, c.Theme()
	ui.Column(c).FillWidth().Gap(Space3).PadBottom(Space4).Children(func() {
		ui.Text(c, s.GuideSection).Muted().FontSize(theme.Metrics.FontSizeS)
		ui.Divider(c)
		ui.Wrap(c, s.GuideIntro)
		for i, section := range [][2]string{{s.GuideScopeTitle, s.GuideScope}, {s.GuideVariablesTitle, s.GuideVariables}, {s.GuideAuthTitle, s.GuideAuth}, {s.GuideEditingTitle, s.GuideEditing}} {
			ui.Column(c.KeyInt(i)).FillWidth().Gap(Space2).Children(func() {
				ui.Text(c, section[0]).Bold().FontSize(theme.Metrics.FontSizeM + Space1)
				body := section[1]
				if folder && i == 0 {
					body = s.GuideFolderScope
				}
				ui.Wrap(c, body).Color(theme.MutedForeground)
			})
		}
	})
}
