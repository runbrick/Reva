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

func (a *App) collectionSettingsTitle(t *Tab) string {
	d := a.collections.settings[t.ID]
	name := d.savedName
	col, item, ok := a.collectionSettingsTarget(t)
	if ok {
		name = col.Name
		if item != nil {
			name = item.Name
		}
	}
	format := a.S.Collections.SettingsTitle
	if t.TargetID != "" {
		format = a.S.Collections.FolderSettingsTitle
	}
	return fmt.Sprintf(format, name)
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
	col, _, ok := a.collectionSettingsTarget(t)
	d.missing = !ok
	a.syncCollectionSettingsDirty(t)
	ui.Column(c).FillWidth().Grow(1).MinHeight(0).Pad(Space4).Gap(Space3).Children(func() {
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			ui.Icon(c, icons.Folder).Size(Space6, Space6).Color(theme.Primary)
			ui.Text(c, a.collectionSettingsTitle(t)).Bold().Ellipsis().Grow(1)
		})
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			if t.Dirty {
				ui.Text(c, labels.Unsaved).Color(theme.Primary).Grow(1).Ellipsis()
			} else {
				ui.Row(c).Grow(1)
			}
			if ui.Button(c, labels.DiscardReload).Outline().Disabled(d.missing).Clicked() {
				c.After(func() { a.reloadCollectionSettings(t) })
			}
			if ui.Button(c, labels.Save).Icon(icons.Save).Disabled(d.missing || !t.Dirty || strings.TrimSpace(d.name) == "").Clicked() {
				c.After(func() { a.saveCollectionSettings(t) })
			}
		})
		if d.missing {
			ui.Wrap(c, labels.SettingsMissing).Color(theme.Danger)
		}
		if d.saveFailed {
			ui.Wrap(c, labels.SettingsSaveFailed).Color(theme.Danger)
		}
		sections := []collectionSettingsSection{collectionSettingsGeneral, collectionSettingsAuthentication}
		if t.TargetID == "" {
			sections = append(sections, collectionSettingsVariables)
		}
		ui.Choice(c, &d.section, sections, a.collectionSettingsSectionLabel).Label(labels.SettingsNavigation).FillWidth()
		ui.Divider(c)
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Scrollbars().Label(labels.SettingsContent).Children(func() {
			ui.Column(c).FillWidth().Gap(Space3).Children(func() {
				switch d.section {
				case collectionSettingsGeneral:
					ui.Wrap(c, labels.SettingsHelp).Muted()
					ui.TextField(c, &d.name, labels.Name, "").FillWidth()
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
