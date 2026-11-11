package gui

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// settingsState is the settings dialog's UI state. Owned by the settings PR.
type settingsState struct {
	open          bool
	current       model.Settings
	language      i18n.Lang
	dir           string
	dirError      error
	historyLabels [5]string
	reveal        *ui.EffectState[struct{}, string]
}

var settingsThemes = []string{"light", "dark"}
var settingsHistoryLimits = []int{50, 100, 200, 500, 1000}

// settingsButton declares the top-bar button that opens settings.
func (a *App) settingsButton(c ui.C) {
	if ui.Shortcut(c, ui.Primary, ",") {
		a.openSettings()
	}
	if ui.Button(c, a.S.Settings.Title).Clicked() {
		a.openSettings()
	}
}

func (a *App) openSettings() {
	if a.settings.open {
		return
	}
	a.settings.open = true
	a.settings.dir, a.settings.dirError = storage.Dir()
	for i, limit := range settingsHistoryLimits {
		a.settings.historyLabels[i] = strconv.Itoa(limit)
	}
}

// settingsDialog declares the settings dialog.
func (a *App) settingsDialog(c ui.C) {
	state := &a.settings
	if state.reveal != nil {
		ui.Effect(c, state.reveal, state.dir)
		if _, err, ok := state.reveal.Take(); ok {
			state.reveal = nil
			if err != nil {
				a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
			}
		}
	}
	if !state.open {
		return
	}
	state.current = a.Svc.Settings.Get()
	state.language = i18n.Lang(state.current.Language)
	ui.Modal(c, &state.open, a.S.Settings.Title, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, a.S.Settings.Theme)
				if ui.Choice(c, &state.current.Theme, settingsThemes, a.settingsThemeLabel).FillWidth().Changed() {
					a.saveSettingsAfter(c, state.current)
				}
			})
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, a.S.Settings.Language)
				if ui.Choice(c, &state.language, i18n.Langs, a.settingsLanguageLabel).FillWidth().Changed() {
					state.current.Language = string(state.language)
					a.saveSettingsAfter(c, state.current)
				}
			})
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, a.S.Settings.HistoryLimit)
				if ui.Select(c, &state.current.HistoryLimit, settingsHistoryLimits, a.settingsHistoryLabel).
					Label(a.S.Settings.HistoryLimit).FillWidth().Changed() {
					a.saveSettingsAfter(c, state.current)
				}
			})
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, a.S.Settings.DataDirectory)
				if state.dirError != nil {
					ui.Wrap(c, state.dirError.Error()).FillWidth()
					return
				}
				ui.Wrap(c, state.dir).FillWidth().Muted()
				if ui.Button(c, a.S.Settings.OpenFolder).Disabled(state.reveal != nil).Clicked() {
					c.After(func() {
						state.reveal = ui.NewEffectKeyed[struct{}, string](revealSettingsFolder)
					})
				}
			})
		})
	}, func() {
		if ui.Button(c, a.S.Settings.Close).Clicked() {
			state.open = false
		}
	})
}

// Save outside declaration so the following paint uses the new strings and theme.
func (a *App) saveSettingsAfter(c ui.C, s model.Settings) {
	c.After(func() {
		if err := a.Svc.Settings.Save(s); err != nil {
			a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
			return
		}
		a.ApplySettings(s)
	})
}

func (a *App) settingsThemeLabel(theme string) string {
	if theme == "dark" {
		return a.S.Settings.Dark
	}
	return a.S.Settings.Light
}

func (a *App) settingsLanguageLabel(lang i18n.Lang) string {
	if lang == i18n.En {
		return a.S.Settings.English
	}
	return a.S.Settings.Chinese
}

func (a *App) settingsHistoryLabel(limit int) string {
	for i, value := range settingsHistoryLimits {
		if value == limit {
			return a.settings.historyLabels[i]
		}
	}
	return ""
}

func revealSettingsFolder(ctx context.Context, dir string) (struct{}, error) {
	if err := ctx.Err(); err != nil {
		return struct{}{}, err
	}
	path, err := filepath.Abs(dir)
	if err == nil {
		err = app.RevealPath(path)
	}
	return struct{}{}, err
}

// settingsPage is the full-width Settings page (rail item and titlebar
// navigation). Until the settings PR builds it, it offers the dialog.
func (a *App) settingsPage(c ui.C) {
	ui.Column(c).Fill().Pad(Space6).Gap(Space3).Children(func() {
		ui.Text(c, a.S.Settings.Title).FontSize(c.Theme().Metrics.FontSizeL).Bold()
		a.settingsButton(c)
	})
}
