package gui

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// titlebarState is the custom titlebar's UI state. Owned by the titlebar PR.
type titlebarState struct{}

// windowConfig sets the window chrome on base: a custom titlebar Reqo paints
// itself. Owned by the titlebar PR.
func (a *App) windowConfig(base app.Config) app.Config {
	base.WindowMode = app.WindowCustomTitlebar
	base.TitlebarHeight = TitlebarHeight
	return base
}

// titlebarStrip declares the TitlebarHeight-high strip at the top of the
// window: window controls (system traffic lights on macOS at the leading
// end; Viva-drawn minimize/maximize/close on Windows at the trailing end),
// logo, page navigation, global search, environment, import and user menu.
// Owned by the titlebar PR.
func (a *App) titlebarStrip(c ui.C) {
	leading, trailing := app.TitlebarInsets()
	ui.Row(c).FillWidth().Height(TitlebarHeight).Background(a.palette.Rail).MovesWindow().
		PadLeft(leading + Space4).PadRight(trailing + Space4).Gap(Space3).Items(ui.Center).Children(func() {
		ui.Text(c, a.S.Common.AppName).Bold()
		ui.Spacer(c)
		a.searchBox(c)
		a.envSelector(c)
		a.importButton(c)
		a.settingsButton(c)
	})
	ui.Divider(c)
}
