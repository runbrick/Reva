package gui

import (
	"context"
	"path/filepath"

	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

const settingsVersion = "0.2.0"

// settingsState holds the page's controls and folder-reveal effect.
type settingsState struct {
	initialized                bool
	current                    model.Settings
	language                   i18n.Lang
	followRedirects, verifyTLS bool
	timeout                    int
	dir                        string
	dirError                   error
	reveal                     *ui.EffectState[struct{}, string]
}

var settingsThemes = []string{"light", "dark", "system"}
var settingsHistoryLimits = []int{50, 100, 200, 500, 1000}
var settingsTimeouts = []int{10, 30, 60, 120, 300}

// settingsButton declares the titlebar action and its window-wide shortcut.
func (a *App) settingsButton(c ui.C) {
	if ui.Shortcut(c, ui.Primary, ",") {
		a.openSettings()
	}
	if ui.Button(c, a.S.Settings.Title).Icon(icons.Settings).IconOnly().Ghost().Tooltip(a.S.Settings.Title).Clicked() {
		a.openSettings()
	}
	// Keep the effect declared while navigating away from the settings page.
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
}

func (a *App) openSettings() { a.Main = PageSettings }

// settingsDialog remains the shell's compatibility hook; settings is a page.
func (a *App) settingsDialog(c ui.C) {}

func (a *App) settingsPage(c ui.C) {
	state := &a.settings
	if !state.initialized {
		state.dir, state.dirError = storage.Dir()
		state.initialized = true
	}
	state.current = a.Svc.Settings.Get()
	state.language = i18n.Lang(state.current.Language)
	state.followRedirects, state.verifyTLS = !state.current.NoFollowRedirects, !state.current.InsecureTLS
	state.timeout = state.current.RequestTimeoutSec
	if state.timeout == 0 {
		state.timeout = 60
	}
	s := &a.S.Settings
	theme := c.Theme()
	ui.Scroll(c).FillWidth().Grow(1).Basis(0).Scrollbars().Children(func() {
		ui.Column(c).FillWidth().Pad(Space6).Items(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(720).Gap(Space4).Children(func() {
				ui.Text(c, s.Title).FontSize(theme.Metrics.FontSizeL).Bold()
				settingsCard(c, s.Appearance, func() {
					settingsRow(c, s.Theme, s.ThemeDescription, func() {
						if ui.Choice(c, &state.current.Theme, settingsThemes, a.settingsThemeLabel).Width(Space6 * 15).Changed() {
							a.saveSettingsAfter(c, state.current)
						}
					})
					ui.Divider(c)
					settingsRow(c, s.Language, s.LanguageDescription, func() {
						if ui.Choice(c, &state.language, i18n.Langs, a.settingsLanguageLabel).Changed() {
							state.current.Language = string(state.language)
							a.saveSettingsAfter(c, state.current)
						}
					})
				})
				settingsCard(c, s.Requests, func() {
					settingsRow(c, s.Timeout, s.TimeoutDescription, func() {
						if ui.Select(c, &state.timeout, settingsTimeouts, a.settingsTimeoutLabel).
							Label(s.Timeout).Width(Space6 * 6).Changed() {
							state.current.RequestTimeoutSec = state.timeout
							a.saveSettingsAfter(c, state.current)
						}
					})
					ui.Divider(c)
					settingsRow(c, s.FollowRedirects, s.FollowRedirectsDescription, func() {
						if ui.Switch(c, &state.followRedirects, "").Label(s.FollowRedirects).Changed() {
							state.current.NoFollowRedirects = !state.followRedirects
							a.saveSettingsAfter(c, state.current)
						}
					})
					ui.Divider(c)
					settingsRow(c, s.VerifyTLS, s.VerifyTLSDescription, func() {
						if ui.Switch(c, &state.verifyTLS, "").Label(s.VerifyTLS).Changed() {
							state.current.InsecureTLS = !state.verifyTLS
							a.saveSettingsAfter(c, state.current)
						}
					})
					if state.current.InsecureTLS {
						ui.Wrap(c, s.TLSWarning).FillWidth().FontSize(theme.Metrics.FontSizeS).Color(a.palette.Warning)
					}
				})
				settingsCard(c, s.History, func() {
					settingsRow(c, s.HistoryLimit, s.HistoryDescription, func() {
						if ui.Select(c, &state.current.HistoryLimit, settingsHistoryLimits, a.settingsHistoryLabel).
							Label(s.HistoryLimit).Width(Space6 * 6).Changed() {
							a.saveSettingsAfter(c, state.current)
						}
					})
				})
				settingsCard(c, s.Data, func() {
					directory := state.dir
					if state.dirError != nil {
						directory = state.dirError.Error()
					}
					settingsRow(c, s.DataDirectory, directory, func() {
						if state.dirError == nil && ui.Button(c, s.OpenFolder).Outline().Disabled(state.reveal != nil).Clicked() {
							c.After(func() { state.reveal = ui.NewEffectKeyed[struct{}, string](revealSettingsFolder) })
						}
					})
					ui.Divider(c)
					settingsRow(c, s.About, a.S.Common.AppName, func() {
						ui.Text(c, settingsVersion).Muted()
					})
				})
			})
		})
	})
}

// settingsCard and settingsRow adapt Viva's settings-row recipe to grouped controls.
func settingsCard(c ui.C, title string, body func()) {
	theme := c.Theme()
	ui.Column(c).FillWidth().PadX(Space4).PadY(Space3).
		Background(theme.Card).Border(1, theme.Border).Radius(theme.Metrics.RadiusSurface).Children(func() {
		ui.Text(c, title).Bold().Color(theme.Primary).MarginBottom(Space1)
		body()
	})
}

func settingsRow(c ui.C, title, description string, control func()) {
	ui.Row(c).FillWidth().Gap(Space4).PadY(Space2).Items(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(Space1).Children(func() {
			ui.Text(c, title)
			ui.Wrap(c, description).FillWidth().Muted().FontSize(c.Theme().Metrics.FontSizeS)
		})
		ui.Row(c).Shrink(0).Items(ui.Center).Children(control)
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
	switch theme {
	case "dark":
		return a.S.Settings.Dark
	case "system":
		return a.S.Settings.System
	default:
		return a.S.Settings.Light
	}
}

func (a *App) settingsLanguageLabel(lang i18n.Lang) string {
	if lang == i18n.En {
		return a.S.Settings.English
	}
	return a.S.Settings.Chinese
}

func (a *App) settingsHistoryLabel(limit int) string {
	switch limit {
	case 50:
		return "50"
	case 100:
		return "100"
	case 200:
		return "200"
	case 500:
		return "500"
	case 1000:
		return "1000"
	}
	return ""
}

func (a *App) settingsTimeoutLabel(seconds int) string {
	switch seconds {
	case 10:
		return "10"
	case 30:
		return "30"
	case 60:
		return "60"
	case 120:
		return "120"
	case 300:
		return "300"
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
