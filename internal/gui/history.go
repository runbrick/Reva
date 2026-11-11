package gui

import "github.com/vivaui/viva/ui"

// historyState is the history pane's UI state. Owned by the history PR.
type historyState struct{}

// historyPane lists recently sent requests; choosing one opens it.
func (a *App) historyPane(c ui.C) {
	ui.Text(c, a.S.History.Title).Muted()
}
