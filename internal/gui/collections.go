package gui

import "github.com/vivaui/viva/ui"

// collectionsState is the collections pane's and save dialog's UI state.
// Owned by the collections PR.
type collectionsState struct{}

// collectionsPane shows the collection tree; choosing a request opens it.
func (a *App) collectionsPane(c ui.C) {
	ui.Text(c, a.S.Collections.Title).Muted()
}

// RequestSave asks where to save t, or saves in place when t already
// belongs to a collection. The request editor's Save button and
// Ctrl/Cmd+S call it.
func (a *App) RequestSave(t *Tab) {}

// collectionsDialogs declares the save / new / rename dialogs.
func (a *App) collectionsDialogs(c ui.C) {}
