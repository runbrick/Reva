package gui

import "github.com/vivaui/viva/ui"

// searchState is the global search's UI state. Owned by the search PR.
type searchState struct{}

// searchBox declares the titlebar search field (opens the palette).
// Owned by the search PR.
func (a *App) searchBox(c ui.C) {}

// searchPalette declares the search/command palette overlay (Primary+K).
// Owned by the search PR.
func (a *App) searchPalette(c ui.C) {}
