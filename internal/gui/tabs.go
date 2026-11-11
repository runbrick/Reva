package gui

import (
	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/ui"
)

// tabsState is the tab bar's UI state. Owned by the tabs PR.
type tabsState struct{}

// tabBar declares the strip of open request tabs above the editor.
func (a *App) tabBar(c ui.C) {}

// OpenRequest shows req (from history or a collection) for editing.
// collectionID is where it is stored, or "" when it is not saved.
// Without tabs it replaces the active tab's request.
func (a *App) OpenRequest(req model.Request, collectionID string) {
	t := a.Active()
	t.Request = req.Clone()
	t.CollectionID = collectionID
	t.Response, t.Err, t.Tests, t.Dirty = nil, "", nil, false
}
