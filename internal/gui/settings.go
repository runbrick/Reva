package gui

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
	"github.com/runbrick/reva/internal/update"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

const settingsVersion = update.Current

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
	check                      func(context.Context) (update.Release, error)
	checking                   *ui.EffectState[update.Release, string]
	latest                     update.Release
	checkError                 error
	checked                    bool
	sync                       syncState
	guideOpen                  bool
	guideError                 string
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
	if state.checking != nil {
		ui.Effect(c, state.checking, update.Feed)
		if release, err, ok := state.checking.Take(); ok {
			state.checking = nil
			state.latest, state.checkError, state.checked = release, err, true
		}
	}
	a.declareSync(c)
}

func (a *App) openSettings() { a.Main = PageSettings }

// settingsDialog declares the settings page's optional dialogs.
func (a *App) settingsDialog(c ui.C) {
	a.syncDialog(c)
	a.onboardingDialog(c)
}

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
				ui.Row(c).FillWidth().Items(ui.Center).Gap(Space3).Children(func() {
					ui.Text(c, s.Title).FontSize(theme.Metrics.FontSizeL).Bold().Grow(1)
					a.onboardingEntry(c)
				})
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
				})
				a.syncSettings(c)
				settingsCard(c, s.About, func() {
					settingsRow(c, s.About, a.S.Common.AppName, func() {
						ui.Text(c, "v"+settingsVersion).Muted()
						if ui.Button(c, s.CheckUpdates).Icon(icons.RefreshCw).Outline().Disabled(state.checking != nil).Clicked() {
							c.After(func() {
								check := state.check
								if check == nil {
									check = func(ctx context.Context) (update.Release, error) { return update.Latest(ctx, nil, update.Feed) }
								}
								state.checked, state.checkError, state.latest = false, nil, update.Release{}
								state.checking = ui.NewEffectKeyed[update.Release, string](func(ctx context.Context, _ string) (update.Release, error) {
									return check(ctx)
								})
							})
						}
					})
					if state.checking != nil {
						ui.Text(c, s.CheckingUpdates).Muted()
					} else if state.checked {
						switch {
						case errors.Is(state.checkError, update.ErrNoRelease):
							ui.Text(c, s.NoRelease).Muted()
						case state.checkError != nil:
							ui.Wrap(c, s.UpdateFailed+": "+state.checkError.Error()).FillWidth().Color(theme.Danger)
						case update.Newer(settingsVersion, state.latest.Version):
							ui.Row(c).FillWidth().Items(ui.Center).Gap(Space2).Children(func() {
								ui.Text(c, s.UpdateAvailable+" v"+state.latest.Version).Color(theme.Success)
								if ui.Button(c, s.OpenRelease).Icon(icons.ExternalLink).Outline().Clicked() {
									c.After(func() {
										if err := app.OpenURL(state.latest.URL); err != nil {
											a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
										}
									})
								}
							})
						default:
							ui.Text(c, s.UpToDate).Muted()
						}
					}
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
