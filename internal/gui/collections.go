package gui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/vivaui/viva/ui"
)

// collectionsState is the collections pane's and save dialog's UI state.
// Owned by the collections PR.
type collectionsState struct {
	tree            ui.TreeState
	all, visible    []collectionNode
	closed          map[string]bool
	keys            map[string]int
	nextKey         int
	ready           bool
	label           func(int) string
	collectionIDs   []string
	collectionNames map[string]string

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
	return ui.TreeNode{Depth: n.depth, Branch: n.branch, Expanded: !s.closed[s.nodeKey(n)]}
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
	hiddenBelow := -1
	for _, n := range s.all {
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
		s.collectionLabel = func(id string) string { return s.collectionNames[id] }
		s.folderLabel = func(id string) string {
			if id == "" {
				return a.S.Collections.Root
			}
			return s.folderNames[id]
		}
	}
	cols := a.Svc.Collections.List()
	if s.ready && s.matches(cols) {
		return
	}
	s.ready = true
	s.all, s.collectionIDs = s.all[:0], s.collectionIDs[:0]
	s.collectionNames = make(map[string]string, len(cols))
	for _, col := range cols {
		s.collectionIDs = append(s.collectionIDs, col.ID)
		s.collectionNames[col.ID] = col.Name
		s.appendNode(collectionNode{collectionID: col.ID, name: col.Name, branch: true})
		s.appendItems(col.ID, "", col.Items, 1)
	}
	s.folderCollection = "" // invalidate the save dialog's folder choices too
	s.flattenVisible()
}

// collectionsPane shows the collection tree; choosing a request opens it.
func (a *App) collectionsPane(c ui.C) {
	a.refreshCollections()
	s := &a.collections
	ui.Column(c).Fill().Gap(8).Children(func() {
		if ui.Button(c, a.S.Collections.NewCollection).Outline().FillWidth().Clicked() {
			a.collectionNameDialog(collectionCreate, collectionNode{})
		}
		if len(s.all) == 0 {
			ui.Text(c, a.S.Collections.Empty).Muted()
			return
		}
		ui.Tree(c, &s.tree, len(s.visible), s.label, func(i int) {
			n := s.visible[i]
			rc := c.KeyInt(n.key)
			row := ui.Row(rc).Grow(1).FillHeight().Gap(8).Items(ui.Center).Hit(s.tree.RowID(i), n.name).Children(func() {
				if n.method != "" {
					theme := c.Theme()
					ui.Text(rc, string(n.method)).FontSize(theme.Metrics.FontSizeS).Color(theme.Primary).Background(theme.Secondary).PadX(4).PadY(2).Radius(4).Shrink(0)
				}
				ui.Text(rc, n.name)
			})
			ui.ContextMenu(rc, row, func(menu *ui.MenuItems) { a.collectionMenu(c, menu, n) })
		}).FillWidth().Grow(1)
		if i, ok := s.tree.Toggled(); ok {
			n := s.visible[i]
			c.After(func() { k := s.nodeKey(n); s.closed[k] = !s.closed[k]; s.flattenVisible() })
		}
		if i, ok := s.tree.Chosen(); ok {
			n := s.visible[i]
			if !n.branch {
				c.After(func() { a.openCollectionRequest(n) })
			}
		}
	})
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

// collectionsOverview is the sidebar of the Collections rail section
// (manage collections). Until the collections PR builds it, it is the tree.
func (a *App) collectionsOverview(c ui.C) { a.collectionsPane(c) }
