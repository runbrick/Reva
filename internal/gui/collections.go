package gui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// collectionsState is the collections pane's and save dialog's UI state.
// Owned by the collections PR.
type collectionsState struct {
	tree                            ui.TreeState
	all, visible                    []collectionNode
	closed                          map[string]bool
	keys                            map[string]int
	nextKey                         int
	ready                           bool
	label                           func(int) string
	collectionIDs                   []string
	collectionNames                 map[string]string
	actionsFormat                   string
	filter, appliedFilter           string
	addOpen, rowMenuOpen            bool
	rowMenuID                       string
	selected                        collectionNode
	activeCollection, activeRequest string
	variablesOpen                   bool
	variablesCollection             string
	variables                       []model.KV
	variablesTable                  kvTableState

	dialogOpen bool
	dialog     collectionDialog
	target     collectionNode
	name       string

	saveOpen                             bool
	saveTab                              *Tab
	saveName, saveCollection, saveFolder string
	inlineCollection                     bool
	newCollectionName                    string
	folderCollection                     string
	folderIDs                            []string
	folderNames                          map[string]string
	collectionLabel, folderLabel         func(string) string
}

type collectionNode struct {
	collectionID, id, parentID, name string
	actions                          string
	method                           model.Method
	depth, key                       int
	branch                           bool
}

type collectionDialog uint8

const (
	collectionCreate collectionDialog = iota
	collectionFolder
	collectionRename
	collectionDelete
)

func (s *collectionsState) node(i int) ui.TreeNode {
	n := s.visible[i]
	return ui.TreeNode{Depth: n.depth, Branch: n.branch, Expanded: s.filter != "" || !s.closed[s.nodeKey(n)], Selected: !n.branch && n.collectionID == s.activeCollection && n.id == s.activeRequest}
}

func (s *collectionsState) nodeKey(n collectionNode) string { return n.collectionID + "/" + n.id }
func (s *collectionsState) rowKey(i int) int                { return s.visible[i].key }
func (s *collectionsState) rowLabel(i int) string           { return s.visible[i].name }

// sameItems compares the service's hierarchy with the cached descriptions without
// allocating another flattened tree. Request contents are read fresh on opening.
func (s *collectionsState) sameItems(colID, parentID string, items []model.Item, at *int, depth int) bool {
	for _, item := range items {
		method := model.Method("")
		if item.Request != nil {
			method = item.Request.Method
		}
		if *at >= len(s.all) {
			return false
		}
		n := s.all[*at]
		if n.collectionID != colID || n.id != item.ID || n.parentID != parentID || n.name != item.Name || n.method != method || n.depth != depth || n.branch != (item.Type == model.ItemFolder) {
			return false
		}
		*at++
		if !s.sameItems(colID, item.ID, item.Items, at, depth+1) {
			return false
		}
	}
	return true
}

func (s *collectionsState) matches(cols []model.Collection) bool {
	at := 0
	for _, col := range cols {
		if at >= len(s.all) {
			return false
		}
		n := s.all[at]
		if n.collectionID != col.ID || n.id != "" || n.name != col.Name {
			return false
		}
		at++
		if !s.sameItems(col.ID, "", col.Items, &at, 1) {
			return false
		}
	}
	return at == len(s.all)
}

func (s *collectionsState) appendNode(n collectionNode) {
	k := s.nodeKey(n)
	if _, ok := s.keys[k]; !ok {
		s.nextKey++
		s.keys[k] = s.nextKey
	}
	n.key = s.keys[k]
	n.actions = fmt.Sprintf(s.actionsFormat, n.name)
	s.all = append(s.all, n)
}

func (s *collectionsState) appendItems(colID, parentID string, items []model.Item, depth int) {
	for _, item := range items {
		n := collectionNode{collectionID: colID, id: item.ID, parentID: parentID, name: item.Name, depth: depth, branch: item.Type == model.ItemFolder}
		if item.Request != nil {
			n.method = item.Request.Method
		}
		s.appendNode(n)
		s.appendItems(colID, item.ID, item.Items, depth+1)
	}
}

func (s *collectionsState) flattenVisible() {
	s.visible = s.visible[:0]
	query := strings.ToLower(strings.TrimSpace(s.filter))
	s.appliedFilter = s.filter
	hiddenBelow := -1
	for i, n := range s.all {
		if query != "" {
			match := strings.Contains(strings.ToLower(n.name), query)
			for j := i + 1; !match && j < len(s.all) && s.all[j].depth > n.depth; j++ {
				match = strings.Contains(strings.ToLower(s.all[j].name), query)
			}
			if match {
				s.visible = append(s.visible, n)
			}
			continue
		}
		if hiddenBelow >= 0 && n.depth > hiddenBelow {
			continue
		}
		hiddenBelow = -1
		s.visible = append(s.visible, n)
		if n.branch && s.closed[s.nodeKey(n)] {
			hiddenBelow = n.depth
		}
	}
}

func (a *App) refreshCollections() {
	s := &a.collections
	if !s.ready {
		s.closed, s.keys = make(map[string]bool), make(map[string]int)
		s.tree.Node, s.tree.Key, s.label = s.node, s.rowKey, s.rowLabel
		s.tree.CanDrop = s.canDrop
		s.collectionLabel = func(id string) string { return s.collectionNames[id] }
		s.folderLabel = func(id string) string {
			if id == "" {
				return a.S.Collections.Root
			}
			return s.folderNames[id]
		}
	}
	cols := a.Svc.Collections.List()
	if s.ready && s.actionsFormat == a.S.Collections.Actions && s.matches(cols) {
		if s.appliedFilter != s.filter {
			s.flattenVisible()
		}
		return
	}
	s.ready = true
	s.actionsFormat = a.S.Collections.Actions
	s.all, s.collectionIDs = s.all[:0], s.collectionIDs[:0]
	s.collectionNames = make(map[string]string, len(cols))
	for _, col := range cols {
		s.collectionIDs = append(s.collectionIDs, col.ID)
		s.collectionNames[col.ID] = col.Name
		s.appendNode(collectionNode{collectionID: col.ID, name: col.Name, branch: true})
		s.appendItems(col.ID, "", col.Items, 1)
	}
	s.folderCollection = "" // invalidate the save dialog's folder choices too
	// Deletion and external imports can invalidate the creation menu's target.
	selected := collectionNode{}
	for _, n := range s.all {
		if n.collectionID == s.selected.collectionID && n.id == s.selected.id {
			selected = n
			break
		}
	}
	s.selected = selected
	s.flattenVisible()
}

// collectionsPane shows the collection tree; choosing a request opens it.
func (a *App) collectionsPane(c ui.C) {
	a.refreshCollections()
	s, text, theme := &a.collections, a.S.Collections, c.Theme()
	s.activeCollection, s.activeRequest = a.Active().CollectionID, a.Active().Request.ID
	ui.Column(c).Fill().Pad(Space3).Gap(Space2).Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Children(func() {
			ui.Text(c, text.Title).Bold().Grow(1)
			add := ui.Button(c, text.Add).Icon(icons.Plus).IconOnly().Ghost()
			if add.Clicked() {
				s.addOpen = !s.addOpen
			}
			ui.Menu(c, add, &s.addOpen, func(menu *ui.MenuItems) {
				if menu.Item(text.NewCollection).Icon(icons.Plus).Clicked() {
					c.After(func() { a.collectionNameDialog(collectionCreate, collectionNode{}) })
				}
				if menu.Item(text.NewFolder).Icon(icons.Folder).Disabled(s.selected.collectionID == "").Clicked() {
					n := s.selected
					if !n.branch {
						n.id = n.parentID
					}
					c.After(func() { a.collectionNameDialog(collectionFolder, n) })
				}
				if menu.Item(text.NewRequest).Clicked() {
					c.After(func() {
						if s.selected.collectionID == "" {
							a.newCollectionTab()
						} else {
							n := s.selected
							if !n.branch {
								n.id = n.parentID
							}
							a.newCollectionRequest(n)
						}
					})
				}
			})
		})
		ui.TextField(c, &s.filter, text.Filter, text.Filter).Icon(icons.Search).FillWidth()
		// Read the bound string after declaring the editor, before the Tree reads
		// its rows. Text input arrives in a regular repaint rather than a click.
		if s.appliedFilter != s.filter {
			s.flattenVisible()
		}
		if len(s.all) == 0 {
			ui.Paragraph(c, text.Empty, 0).FillWidth().Muted().FontSize(theme.Metrics.FontSizeS)
		}
		if len(s.all) > 0 && len(s.visible) == 0 {
			ui.Text(c, text.NoMatches).Muted()
		}
		ui.Tree(c, &s.tree, len(s.visible), s.label, func(i int) {
			n := s.visible[i]
			rc := c.KeyInt(n.key)
			selected := s.node(i).Selected
			row := ui.Row(rc).Grow(1).FillHeight().Gap(Space2).Items(ui.Center).Radius(theme.Metrics.RadiusControl).HitWith(s.tree.RowID(i), n.name, gfx.HitOptions{Role: gfx.RoleButton, Selected: selected})
			row.Children(func() {
				if n.branch {
					icon := icons.Folder
					if !s.closed[s.nodeKey(n)] || s.filter != "" {
						icon = icons.FolderOpen
					}
					ui.Icon(rc, icon).Size(Space4, Space4).Color(theme.MutedForeground)
				} else {
					a.MethodBadge(rc, n.method)
				}
				label := ui.Text(rc, n.name).Grow(1).Ellipsis()
				if selected {
					label.Color(theme.Primary)
				}
				more := ui.Button(rc, n.actions).Icon(icons.Ellipsis).IconOnly().Ghost().Shrink(0)
				if !selected && !(s.rowMenuOpen && s.rowMenuID == s.nodeKey(n)) {
					more.Opacity(0).StateFrom(row).Hover(ui.Look{}.Opacity(1)).Focus(ui.Look{}.Opacity(1))
				}
				if more.Clicked() {
					s.selected, s.rowMenuID, s.rowMenuOpen = n, s.nodeKey(n), true
				}
				if s.rowMenuID == s.nodeKey(n) {
					ui.Menu(rc, more, &s.rowMenuOpen, func(menu *ui.MenuItems) { a.collectionMenu(c, menu, n) })
				}
			})
			ui.ContextMenu(rc, row, func(menu *ui.MenuItems) { a.collectionMenu(c, menu, n) })
		}).FillWidth().Grow(1)
		if i, ok := s.tree.Toggled(); ok {
			n := s.visible[i]
			c.After(func() { s.selected = n; k := s.nodeKey(n); s.closed[k] = !s.closed[k]; s.flattenVisible() })
		}
		if i, ok := s.tree.Chosen(); ok {
			n := s.visible[i]
			c.After(func() {
				s.selected = n
				if !n.branch {
					a.openCollectionRequest(n)
				}
			})
		}
		if move, ok := s.tree.Dropped(); ok {
			from, to := s.visible[move.From], s.visible[move.To]
			c.After(func() { a.moveCollectionItem(from, to) })
		}
	})
}

func (a *App) newCollectionTab() {
	a.NewTab()
	a.Main, a.Section = PageWorkbench, SectionAPIs
}

// Only an inside drop is accepted; collection roots cannot themselves be moved.
func (s *collectionsState) canDrop(from, to int, where ui.TreeDrop) bool {
	if from < 0 || to < 0 || from >= len(s.visible) || to >= len(s.visible) || from == to || where != ui.TreeDropInside {
		return false
	}
	source, target := s.visible[from], s.visible[to]
	if source.id == "" || !target.branch {
		return false
	}
	if source.collectionID != target.collectionID {
		return true
	}
	if source.id == target.id || source.parentID == target.id {
		return false
	}
	for parent := target.id; parent != ""; {
		if parent == source.id {
			return false
		}
		found := false
		for _, n := range s.all {
			if n.collectionID == target.collectionID && n.id == parent {
				parent, found = n.parentID, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Copy the hierarchy before mutation: in-memory services may return shared slices.
func cloneCollectionItems(items []model.Item) []model.Item {
	out := append([]model.Item(nil), items...)
	for i := range out {
		out[i].Items = cloneCollectionItems(out[i].Items)
	}
	return out
}

func (a *App) moveCollectionItem(from, to collectionNode) {
	source, ok := a.Svc.Collections.Get(from.collectionID)
	if !ok {
		return
	}
	source.Items = cloneCollectionItems(source.Items)
	item := model.FindItem(source.Items, from.id)
	if item == nil || from.collectionID == to.collectionID && (to.id == from.id || model.FindItem(item.Items, to.id) != nil) {
		return
	}
	moved := *item
	var removed bool
	source.Items, removed = model.RemoveItem(source.Items, from.id)
	if !removed {
		return
	}
	if from.collectionID == to.collectionID {
		var inserted bool
		source.Items, inserted = model.InsertItem(source.Items, to.id, moved)
		if !inserted || a.collectionError(a.Svc.Collections.Save(source)) {
			return
		}
	} else {
		target, exists := a.Svc.Collections.Get(to.collectionID)
		if !exists {
			return
		}
		previous := target
		target.Items = cloneCollectionItems(target.Items)
		var inserted bool
		target.Items, inserted = model.InsertItem(target.Items, to.id, moved)
		if !inserted || a.collectionError(a.Svc.Collections.Save(target)) {
			return
		}
		if a.collectionError(a.Svc.Collections.Save(source)) {
			a.collectionError(a.Svc.Collections.Save(previous))
			return
		}
		for _, tab := range a.tabs {
			if tab.CollectionID == from.collectionID && (tab.Request.ID == moved.ID || model.FindItem(moved.Items, tab.Request.ID) != nil) {
				tab.CollectionID = to.collectionID
			}
		}
	}
	a.collections.closed[a.collections.nodeKey(to)] = false
	a.refreshCollections()
}

func (a *App) collectionMenu(c ui.C, menu *ui.MenuItems, n collectionNode) {
	s := a.S.Collections
	if n.branch {
		if menu.Item(s.NewFolder).Clicked() {
			c.After(func() { a.collectionNameDialog(collectionFolder, n) })
		}
		if menu.Item(s.NewRequest).Clicked() {
			c.After(func() { a.newCollectionRequest(n) })
		}
	} else {
		if menu.Item(s.Open).Clicked() {
			c.After(func() { a.openCollectionRequest(n) })
		}
		if menu.Item(s.Duplicate).Clicked() {
			c.After(func() { a.duplicateCollectionRequest(n) })
		}
	}
	if menu.Item(s.Rename).Clicked() {
		c.After(func() { a.collectionNameDialog(collectionRename, n) })
	}
	if menu.Item(s.Delete).Destructive().Clicked() {
		c.After(func() { a.collectionNameDialog(collectionDelete, n) })
	}
}

func (a *App) collectionError(err error) bool {
	if err == nil {
		return false
	}
	a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
	return true
}

func (a *App) openCollectionRequest(n collectionNode) {
	if col, ok := a.Svc.Collections.Get(n.collectionID); ok {
		if item := model.FindItem(col.Items, n.id); item != nil && item.Request != nil {
			a.OpenRequest(*item.Request, n.collectionID)
			a.Main, a.Section = PageWorkbench, SectionAPIs
		}
	}
}

func (a *App) newCollectionRequest(n collectionNode) {
	r := model.NewRequest(storage.NewID())
	r.Name = a.S.Collections.NewRequest
	r, err := a.Svc.Collections.SaveRequest(n.collectionID, n.id, r)
	if a.collectionError(err) {
		return
	}
	a.collections.closed[a.collections.nodeKey(n)] = false
	a.refreshCollections()
	a.OpenRequest(r, n.collectionID)
	a.Main, a.Section = PageWorkbench, SectionAPIs
}

func (a *App) duplicateCollectionRequest(n collectionNode) {
	col, ok := a.Svc.Collections.Get(n.collectionID)
	if !ok {
		return
	}
	item := model.FindItem(col.Items, n.id)
	if item == nil || item.Request == nil {
		return
	}
	r := item.Request.Clone()
	r.ID, r.Name = storage.NewID(), fmt.Sprintf(a.S.Collections.CopyName, item.Name)
	r, err := a.Svc.Collections.SaveRequest(n.collectionID, n.parentID, r)
	if a.collectionError(err) {
		return
	}
	a.refreshCollections()
	a.OpenRequest(r, n.collectionID)
}

// RequestSave asks where to save t, or saves in place when t already
// belongs to a collection. The request editor's Save button and
// Ctrl/Cmd+S call it.
func (a *App) RequestSave(t *Tab) {
	if t == nil {
		return
	}
	if col, ok := a.Svc.Collections.Get(t.CollectionID); t.CollectionID != "" && ok {
		found := false
		model.WalkRequests(col.Items, func(r *model.Request) {
			if r.ID == t.Request.ID {
				found = true
			}
		})
		if found {
			r, err := a.Svc.Collections.SaveRequest(t.CollectionID, "", t.Request.Clone())
			if a.collectionError(err) {
				return
			}
			t.Request, t.Dirty = r, false
			a.refreshCollections()
			a.Toast(a.S.Collections.Saved, "", ui.ToastSuccess)
			return
		}
	}
	a.refreshCollections()
	s := &a.collections
	s.saveTab, s.saveOpen, s.saveName = t, true, strings.TrimSpace(t.Request.Name)
	if s.saveName == "" {
		s.saveName = strings.TrimSpace(t.Request.URL)
		if u, err := url.Parse(s.saveName); err == nil && u.Host != "" {
			s.saveName = u.Host + u.EscapedPath()
		}
		if s.saveName == "" {
			s.saveName = a.S.Collections.NewRequest
		}
	}
	s.saveCollection, s.saveFolder, s.folderCollection, s.newCollectionName = "", "", "", ""
	s.inlineCollection = len(s.collectionIDs) == 0
	if len(s.collectionIDs) > 0 {
		s.saveCollection = s.collectionIDs[0]
	}
}

// collectionsDialogs declares the save / new / rename dialogs.
func (a *App) collectionsDialogs(c ui.C) {
	s := &a.collections
	if s.variablesOpen {
		ui.Modal(c, &s.variablesOpen, a.S.Collections.Variables, func() {
			a.kvTable(c, &s.variablesTable, &s.variables)
		}, func() {
			if ui.Button(c, a.S.Collections.Cancel).Outline().Clicked() {
				s.variablesOpen = false
			}
			if ui.Button(c, a.S.Collections.Save).Clicked() {
				c.After(func() {
					col, ok := a.Svc.Collections.Get(s.variablesCollection)
					if !ok {
						s.variablesOpen = false
						return
					}
					col.Variables = append([]model.KV(nil), s.variables...)
					if !a.collectionError(a.Svc.Collections.Save(col)) {
						s.variablesOpen = false
						a.Toast(a.S.Collections.VariablesSaved, "", ui.ToastSuccess)
					}
				})
			}
		})
	}
	if s.dialogOpen {
		a.collectionEditDialog(c)
	}
	if !s.saveOpen {
		return
	}
	a.refreshCollections()
	ui.Modal(c, &s.saveOpen, a.S.Collections.SaveRequest, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.TextField(c, &s.saveName, a.S.Collections.Name, "").FillWidth()
			ui.Checkbox(c, &s.inlineCollection, a.S.Collections.InlineCollection)
			if s.inlineCollection {
				ui.TextField(c, &s.newCollectionName, a.S.Collections.CollectionName, "").FillWidth()
			} else {
				ui.Text(c, a.S.Collections.Collection)
				ui.Select(c, &s.saveCollection, s.collectionIDs, s.collectionLabel).Label(a.S.Collections.Collection).FillWidth()
				a.collectionFolders()
				ui.Text(c, a.S.Collections.Folder)
				ui.Select(c, &s.saveFolder, s.folderIDs, s.folderLabel).Label(a.S.Collections.Folder).FillWidth()
			}
		})
	}, func() {
		if ui.Button(c, a.S.Collections.Cancel).Outline().Clicked() {
			s.saveOpen = false
		}
		valid := strings.TrimSpace(s.saveName) != "" && ((s.inlineCollection && strings.TrimSpace(s.newCollectionName) != "") || (!s.inlineCollection && s.saveCollection != ""))
		if ui.Button(c, a.S.Collections.Save).Disabled(!valid).Clicked() {
			c.After(a.saveCollectionRequest)
		}
	})
}

func (a *App) collectionFolders() {
	s := &a.collections
	if s.folderCollection == s.saveCollection && s.folderIDs != nil {
		return
	}
	s.folderCollection, s.saveFolder = s.saveCollection, ""
	s.folderIDs, s.folderNames = []string{""}, make(map[string]string)
	paths := make(map[string]string)
	for _, n := range s.all {
		if n.collectionID != s.saveCollection || n.id == "" || !n.branch {
			continue
		}
		name := n.name
		if parent := paths[n.parentID]; parent != "" {
			name = parent + " / " + name
		}
		paths[n.id], s.folderNames[n.id] = name, name
		s.folderIDs = append(s.folderIDs, n.id)
	}
}

func (a *App) saveCollectionRequest() {
	s := &a.collections
	if s.saveTab == nil || strings.TrimSpace(s.saveName) == "" {
		return
	}
	if s.inlineCollection {
		name := strings.TrimSpace(s.newCollectionName)
		if name == "" {
			return
		}
		col, err := a.Svc.Collections.Create(name)
		if a.collectionError(err) {
			return
		}
		s.saveCollection, s.saveFolder, s.inlineCollection = col.ID, "", false
	}
	r := s.saveTab.Request.Clone()
	r.Name = strings.TrimSpace(s.saveName)
	r, err := a.Svc.Collections.SaveRequest(s.saveCollection, s.saveFolder, r)
	if a.collectionError(err) {
		a.refreshCollections()
		return
	}
	s.saveTab.Request, s.saveTab.CollectionID, s.saveTab.Dirty = r, s.saveCollection, false
	s.saveOpen = false
	a.refreshCollections()
	a.Toast(a.S.Collections.Saved, "", ui.ToastSuccess)
}

func (a *App) collectionNameDialog(dialog collectionDialog, n collectionNode) {
	s := &a.collections
	s.dialogOpen, s.dialog, s.target, s.name = true, dialog, n, ""
	if dialog == collectionRename {
		s.name = n.name
	}
}

func (a *App) collectionEditDialog(c ui.C) {
	s, text := &a.collections, a.S.Collections
	title, action := text.NewCollection, text.Create
	switch s.dialog {
	case collectionFolder:
		title = text.NewFolder
	case collectionRename:
		title, action = text.Rename, text.Rename
	case collectionDelete:
		title, action = text.DeleteTitle, text.Delete
	}
	ui.Modal(c, &s.dialogOpen, title, func() {
		if s.dialog == collectionDelete {
			ui.Text(c, fmt.Sprintf(text.DeletePrompt, s.target.name))
			return
		}
		ui.TextField(c, &s.name, text.Name, "").FillWidth()
	}, func() {
		if ui.Button(c, text.Cancel).Outline().Clicked() {
			s.dialogOpen = false
		}
		if ui.Button(c, action).Disabled(s.dialog != collectionDelete && strings.TrimSpace(s.name) == "").Clicked() {
			c.After(a.applyCollectionEdit)
		}
	})
}

func (a *App) applyCollectionEdit() {
	s := &a.collections
	n, name := s.target, strings.TrimSpace(s.name)
	var err error
	switch s.dialog {
	case collectionCreate:
		_, err = a.Svc.Collections.Create(name)
	case collectionFolder:
		_, err = a.Svc.Collections.AddFolder(n.collectionID, n.id, name)
		if err == nil {
			s.closed[s.nodeKey(n)] = false
		}
	case collectionRename:
		if n.id == "" {
			if col, ok := a.Svc.Collections.Get(n.collectionID); ok {
				col.Name = name
				err = a.Svc.Collections.Save(col)
			}
		} else {
			err = a.Svc.Collections.RenameItem(n.collectionID, n.id, name)
		}
	case collectionDelete:
		if n.id == "" {
			err = a.Svc.Collections.Delete(n.collectionID)
		} else {
			err = a.Svc.Collections.DeleteItem(n.collectionID, n.id)
		}
	}
	if a.collectionError(err) {
		return
	}
	if s.dialog == collectionRename && n.id != "" && !n.branch {
		// Keep an open editor from undoing the rename on its next save.
		for _, tab := range a.tabs {
			if tab.CollectionID == n.collectionID && tab.Request.ID == n.id {
				tab.Request.Name = name
			}
		}
	}
	s.dialogOpen = false
	a.refreshCollections()
}

// collectionsOverview manages collection metadata in the Collections rail section.
func (a *App) collectionsOverview(c ui.C) {
	a.refreshCollections()
	text, theme := a.S.Collections, c.Theme()
	ui.Column(c).Fill().Pad(Space3).Gap(Space3).Children(func() {
		ui.Text(c, text.Overview).Bold()
		if ui.Button(c, text.NewCollection).Icon(icons.Plus).Outline().FillWidth().Clicked() {
			a.collectionNameDialog(collectionCreate, collectionNode{})
		}
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(Space3).Children(func() {
				for _, col := range a.Svc.Collections.List() {
					rc := c.Key(col.ID)
					n := collectionNode{collectionID: col.ID, name: col.Name, branch: true}
					count := 0
					model.WalkRequests(col.Items, func(*model.Request) { count++ })
					ui.Column(rc).FillWidth().Pad(Space3).Gap(Space2).Background(theme.Card).Radius(theme.Metrics.RadiusSurface).Children(func() {
						ui.Text(rc, col.Name).Bold().Ellipsis()
						ui.Text(rc, fmt.Sprintf(text.RequestCount, count)).Muted().FontSize(theme.Metrics.FontSizeS)
						ui.Row(rc).FillWidth().Gap(Space1).Children(func() {
							if ui.Button(rc, text.Rename).Ghost().Clicked() {
								c.After(func() { a.collectionNameDialog(collectionRename, n) })
							}
							if ui.Button(rc, text.Delete).Ghost().Color(theme.Danger).Clicked() {
								c.After(func() { a.collectionNameDialog(collectionDelete, n) })
							}
						})
						if ui.Button(rc, text.Export).Outline().FillWidth().Clicked() {
							c.After(func() { a.exportCollection(col.ID) })
						}
						if ui.Button(rc, text.Variables).Outline().FillWidth().Clicked() {
							c.After(func() {
								s := &a.collections
								s.variablesCollection, s.variablesOpen = col.ID, true
								s.variables = append([]model.KV(nil), col.Variables...)
								s.variablesTable = kvTableState{}
							})
						}
					})
				}
			})
		})
		if len(a.collections.collectionIDs) == 0 {
			ui.Paragraph(c, text.Empty, 0).FillWidth().Muted()
		}
	})
}

func (a *App) exportCollection(id string) {
	col, ok := a.Svc.Collections.Get(id)
	if !ok {
		return
	}
	path, err := app.SaveFile(app.SaveFileOptions{Title: a.S.Collections.Export, SuggestedName: col.Name + ".json", DefaultExtension: "json", Filters: []app.FileFilter{{Name: "JSON", Pattern: "*.json"}}})
	if a.collectionError(err) || path == "" {
		return
	}
	data, err := json.MarshalIndent(col, "", "  ")
	if err == nil {
		err = os.WriteFile(path, append(data, '\n'), 0600)
	}
	if !a.collectionError(err) {
		a.Toast(a.S.Collections.Exported, "", ui.ToastSuccess)
	}
}
