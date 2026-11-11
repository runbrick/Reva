package gui

import (
	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/ui"
)

// envState is the environment selector's and editor's UI state. Owned by
// the environments PR.
type envState struct{}

// Vars returns the variables used for {{name}} substitution in the next
// send, lowest precedence first (collection variables, then the active
// environment).
func (a *App) Vars() []model.KV { return nil }

// envSelector declares the active-environment picker in the top bar.
func (a *App) envSelector(c ui.C) {}

// envDialog declares the environment editor dialog.
func (a *App) envDialog(c ui.C) {}
