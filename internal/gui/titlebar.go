package gui

import (
	"runtime"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// titlebarState is retained for the shell's feature state slot.
type titlebarState struct{}

// windowConfig sets the window chrome on base: a custom titlebar Reva paints
// itself. Owned by the titlebar PR.
func (a *App) windowConfig(base app.Config) app.Config {
	base.WindowMode = app.WindowCustomTitlebar
	base.TitlebarHeight = TitlebarHeight
	base.WindowControls = func() app.WindowControlsStyle {
		theme, s := a.theme, a.S.Titlebar
		return app.WindowControlsStyle{
			Foreground: theme.MutedForeground, Hover: theme.Hover, Pressed: theme.Muted,
			Close: theme.Danger, CloseForeground: theme.DangerForeground,
			Minimize: s.Minimize, Maximize: s.Maximize, Restore: s.Restore, CloseName: s.Close,
		}
	}
	return base
}

// titlebarInsets uses Viva on the host and its v0.6.0 constants when previewing
// another platform. macOS reserves 70 px; Windows and Linux reserve three buttons.
func (a *App) titlebarInsets() (leading, trailing float32) {
	if a.Platform == runtime.GOOS {
		return app.TitlebarInsets()
	}
	if a.Platform == "darwin" {
		return 70, 0
	}
	return 0, 3 * app.WindowControlWidth
}

// titlebarStrip keeps content clear of native controls. Only the flexible empty
// space moves the window; Viva supplies dragging and double-click maximize/zoom.
func (a *App) titlebarStrip(c ui.C) {
	theme := c.Theme()
	leading, trailing := a.titlebarInsets()
	// Free nav label space before the search slot contracts at the minimum width.
	compact := c.Bounds().Width < 1200
	gap := Space3
	if compact {
		gap = Space1
	}
	ui.Row(c).FillWidth().Height(TitlebarHeight).Shrink(0).Background(a.palette.Sidebar).
		PadLeft(leading + Space4).PadRight(trailing + Space4).Gap(gap).Items(ui.Center).
		Draw(drawTitlebarBorder).Children(func() {
		ui.Row(c).Gap(Space2).Items(ui.Center).Shrink(0).Children(func() {
			ui.Row(c).Size(Space6, Space6).Radius(theme.Metrics.RadiusControl).Background(theme.Primary).
				Center().Children(func() {
				ui.Icon(c, icons.Send).Size(Space4, Space4).Color(theme.PrimaryForeground)
			})
			ui.Text(c, a.S.Common.AppName).Bold()
		})
		a.titlebarNavigation(c, compact)
		ui.Divider(c).Vertical().Height(Space6)
		if ui.Button(c, a.S.Titlebar.NewRequestTab).Icon(icons.Plus).IconOnly().Ghost().Tooltip(a.S.Titlebar.NewRequestTab).Clicked() {
			c.After(func() { a.NewTab(); a.SelectSection(SectionAPIs) })
		}
		ui.Row(c).Grow(1).MinWidth(Space4).FillHeight().MovesWindow()
		// Search may declare a fixed preferred width. This slot yields space to
		// the tools, clipping both paint and hits before the window controls.
		ui.Row(c).Shrink(1).MinWidth(0).Clip().Children(func() { a.searchBox(c) })
		ui.Row(c).Gap(gap).Items(ui.Center).Shrink(0).Children(func() {
			a.envSelector(c)
			a.importButton(c)
			ui.Divider(c).Vertical().Height(Space6)
			a.titlebarThemeButton(c)
			a.settingsButton(c)
		})
	})
}

func (a *App) titlebarThemeButton(c ui.C) {
	label, icon := a.S.Titlebar.DarkTheme, icons.Moon
	if a.Dark() {
		label, icon = a.S.Titlebar.LightTheme, icons.Sun
	}
	if ui.Button(c, label).Icon(icon).IconOnly().Ghost().Tooltip(label).Clicked() {
		c.After(a.toggleTheme)
	}
}

func drawTitlebarBorder(d *ui.DrawContext) {
	r := d.Rect
	r.Y, r.Height = r.Y+r.Height-1, 1
	d.Canvas.Fill(r, d.Theme.Border)
}

func drawTitlebarUnderline(d *ui.DrawContext) {
	r := d.Rect
	r.Y, r.Height = r.Y+r.Height-2, 2
	d.Canvas.Fill(r, d.Theme.Primary)
}

func (a *App) titlebarNavigation(c ui.C, compact bool) {
	theme := c.Theme()
	ui.Row(c).FillHeight().Shrink(0).Gap(Space1).FocusGroup(ui.FocusHorizontal).
		Label(a.S.Shell.NavLabel).Children(func() {
		for _, p := range Pages {
			if p == PageSettings {
				continue
			}
			kc := c.KeyInt(int(p))
			label, icon := a.pageLabel(p), icons.House
			switch p {
			case PageHistory:
				icon = icons.Clock
			}
			fg := theme.MutedForeground
			if a.Main == p {
				fg = theme.AccentForeground
			}
			item := ui.Row(kc).FillHeight().PadX(Space3).Gap(Space2).Items(ui.Center).
				Hover(ui.Look{}.Background(theme.Hover)).
				Focus(ui.Look{}.Ring(2, theme.Ring.WithAlpha(0.5)))
			if a.Main == p {
				item.DrawOver(drawTitlebarUnderline)
			}
			item.Children(func() {
				// ID is declared inside the keyed row's scope: Part names the
				// next element in the current scope, rather than a copied C.
				item.Hit(ui.ID(kc.Part("page")), label)
				ui.Icon(kc, icon).Size(Space4, Space4).Color(fg)
				if !compact {
					ui.Text(kc, label).Bold().Color(fg)
				}
			})
			item.Tooltip(label)
			if item.Clicked() {
				a.Main = p
			}
		}
	})
}
