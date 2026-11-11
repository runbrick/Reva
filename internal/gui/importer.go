package gui

import "github.com/vivaui/viva/ui"

// importerState is the import dialog's UI state. Owned by the import PR.
type importerState struct{}

// importButton declares the top-bar button that opens the import dialog.
func (a *App) importButton(c ui.C) {}

// importDialog declares the import dialog (Postman collection / curl).
func (a *App) importDialog(c ui.C) {}
