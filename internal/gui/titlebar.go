package gui

import (
	"runtime"

	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// titlebarState owns the avatar menu's open state.
type titlebarState struct{ menuOpen bool }

// windowConfig sets the window chrome on base: a custom titlebar Reqo paints
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
	ui.Row(c).FillWidth().Height(TitlebarHeight).Shrink(0).Background(a.palette.Rail).
		PadLeft(leading + Space4).PadRight(trailing + Space4).Gap(gap).Items(ui.Center).
		Draw(drawTitlebarBorder).Children(func() {
		ui.Row(c).Gap(Space2).Items(ui.Center).Shrink(0).Children(func() {
			ui.Row(c).Size(28, 28).Radius(theme.Metrics.RadiusControl).Background(theme.Primary).
				Center().Children(func() {
				ui.Icon(c, icons.Send).Size(Space4, Space4).Color(theme.PrimaryForeground)
			})
			ui.Text(c, a.S.Common.AppName).Bold()
		})
		a.titlebarNavigation(c, compact)
		ui.Row(c).Grow(1).MinWidth(Space4).FillHeight().MovesWindow()
		// Search may declare a fixed preferred width. This slot yields space to
		// the tools, clipping both paint and hits before the window controls.
		ui.Row(c).Shrink(1).MinWidth(0).Clip().Children(func() { a.searchBox(c) })
		ui.Row(c).Gap(Space2).Items(ui.Center).Shrink(0).Children(func() {
			a.envSelector(c)
			a.importButton(c)
			a.settingsButton(c)
			a.titlebarUserMenu(c)
		})
	})
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
			kc := c.KeyInt(int(p))
			label, icon := a.pageLabel(p), icons.House
			switch p {
			case PageHistory:
				icon = icons.Clock
			case PageSettings:
				icon = icons.Settings
			}
			fg := theme.MutedForeground
			if a.Main == p {
				fg = theme.Primary
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

func (a *App) titlebarUserMenu(c ui.C) {
	theme, s := c.Theme(), a.S.Titlebar
	kc := c.Part("user-menu")
	avatar := ui.Row(kc).Gap(Space1).Pad(Space1).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).Hit(ui.ID(kc), s.UserMenu).
		Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring.WithAlpha(0.5))).
		Children(func() {
			ui.Row(kc).Size(32, 32).Radius(Space4).Background(theme.Accent).Center().Children(func() {
				ui.Icon(kc, icons.User).Size(Space4, Space4).Color(theme.AccentForeground)
			})
			ui.Icon(kc, icons.ChevronDown).Size(Space4, Space4).Color(theme.MutedForeground)
		})
	if avatar.Clicked() {
		a.titlebar.menuOpen = !a.titlebar.menuOpen
	}
	ui.Menu(kc, avatar, &a.titlebar.menuOpen, func(menu *ui.MenuItems) {
		if menu.Item(s.Settings).Icon(icons.Settings).Clicked() {
			a.Main = PageSettings
		}
		menu.Separator()
		for _, choice := range [...]struct{ value, label string }{
			{"light", s.Light}, {"dark", s.Dark}, {"system", s.System},
		} {
			if menu.Item(choice.label).Checked(a.themeName == choice.value).Clicked() {
				settings := a.Svc.Settings.Get()
				settings.Theme = choice.value
				a.titlebarSaveSettings(c, settings)
			}
		}
		menu.Separator()
		for _, choice := range [...]struct{ value, label string }{
			{"zh", s.Chinese}, {"en", s.English},
		} {
			if menu.Item(choice.label).Checked(string(a.S.Lang) == choice.value).Clicked() {
				settings := a.Svc.Settings.Get()
				settings.Language = choice.value
				a.titlebarSaveSettings(c, settings)
			}
		}
		menu.Separator()
		if menu.Item(s.About).Clicked() {
			a.Toast(s.About, a.S.Common.AppName, ui.ToastInfo)
		}
	})
}

func (a *App) titlebarSaveSettings(c ui.C, settings model.Settings) {
	c.After(func() {
		if err := a.Svc.Settings.Save(settings); err != nil {
			a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
			return
		}
		a.ApplySettings(settings)
	})
}
