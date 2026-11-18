package gui

import (
	"runtime"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// titlebarState is retained for the shell's feature state slot.
type titlebarState struct{ collectionsOpen bool }

const desktopToolbarHeight = Space6 + Space4

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
	if a.studioLayout() {
		a.studioTitlebar(c)
		return
	}
	if a.desktopLayout() {
		a.desktopTitlebar(c)
		return
	}
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
		a.titlebarBrand(c)
		a.titlebarNavigation(c, compact)
		ui.Divider(c).Vertical().Height(Space6)
		a.titlebarNewRequest(c)
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

func (a *App) studioTitlebar(c ui.C) {
	theme := c.Theme()
	leading, trailing := a.titlebarInsets()
	ui.Row(c).FillWidth().Height(TitlebarHeight).Shrink(0).Background(theme.Card).
		PadLeft(leading + Space4).PadRight(trailing + Space4).Gap(Space3).Items(ui.Center).
		Draw(drawTitlebarBorder).Children(func() {
		a.titlebarBrand(c)
		if !a.requestPreviewOpen() {
			a.studioCollectionPicker(c)
		}
		a.envSelector(c)
		ui.Row(c).Grow(1).MinWidth(Space4).FillHeight().MovesWindow()
		ui.Row(c).Shrink(1).MinWidth(0).Clip().Children(func() { a.searchBox(c) })
		if c.Bounds().Width >= Space6*52 {
			ui.Row(c).Grow(1).MinWidth(Space4).FillHeight().MovesWindow()
		}
		label := a.S.Import.Title
		if ui.Button(c, label).Icon(icons.Download).IconOnly().Outline().Tooltip(label).Clicked() {
			c.After(a.openImportDialog)
		}
		ui.Row(c).Width(Space4).FillHeight().MovesWindow()
		a.declareSettingsEffects(c)
	})
}

func (a *App) studioCollectionPicker(c ui.C) {
	theme, labels := c.Theme(), a.S.Collections
	activeID := a.Active().CollectionID
	name := labels.Title
	if collection, found := a.Svc.Collections.Get(activeID); found {
		name = collection.Name
	}
	trigger := ui.ButtonBase(c, labels.Collection).Width(Space6*7).Height(theme.Metrics.ControlHeight).Shrink(0).
		Radius(theme.Metrics.RadiusControl).Border(1, theme.Border).Background(theme.Card).
		PadX(Space2).Gap(Space2).Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring)).Children(func() {
		ui.Icon(c, icons.Folder).Size(Space4, Space4).Color(theme.MutedForeground)
		ui.Text(c, name).Grow(1).Ellipsis()
		ui.Icon(c, icons.ChevronDown).Size(Space4, Space4).Color(theme.MutedForeground)
	})
	if trigger.Clicked() {
		a.titlebar.collectionsOpen = !a.titlebar.collectionsOpen
	}
	ui.Menu(c, trigger, &a.titlebar.collectionsOpen, func(menu *ui.MenuItems) {
		for _, collection := range a.Svc.Collections.List() {
			if menu.Item(collection.Name).Icon(icons.Folder).Chosen(collection.ID == activeID).Clicked() {
				c.After(func() {
					a.openCollectionSettings(collectionNode{collectionID: collection.ID}, collectionSettingsGeneral)
				})
			}
		}
		if menu.Item(labels.NewCollection).Icon(icons.Plus).Clicked() {
			c.After(func() {
				a.SelectSection(SectionAPIs)
				a.collectionNameDialog(collectionCreate, collectionNode{})
			})
		}
	})
}

func (a *App) titlebarBrand(c ui.C) {
	theme := c.Theme()
	size := Space6
	if a.studioLayout() {
		size += Space1
	}
	ui.Row(c).Gap(Space2).Items(ui.Center).Shrink(0).Children(func() {
		ui.Row(c).Size(size, size).Radius(theme.Metrics.RadiusControl).Background(theme.Primary).
			Center().Children(func() {
			ui.Icon(c, icons.Send).Size(Space4, Space4).Color(theme.PrimaryForeground)
		})
		name := ui.Text(c, a.S.Common.AppName).Bold()
		if a.studioLayout() {
			name.FontSize(theme.Metrics.FontSizeL - Space1)
		}
	})
}

func (a *App) titlebarNewRequest(c ui.C) {
	label := a.S.Titlebar.NewRequestTab
	if ui.Button(c, label).Icon(icons.Plus).IconOnly().Ghost().Tooltip(label).Clicked() {
		c.After(func() { a.NewTab(); a.SelectSection(SectionAPIs) })
	}
}

func (a *App) desktopTitlebar(c ui.C) {
	theme := c.Theme()
	leading, trailing := a.titlebarInsets()
	ui.Row(c).FillWidth().Height(TitlebarHeight).Shrink(0).Background(a.palette.Rail).
		PadLeft(leading + Space4).PadRight(trailing + Space4).Gap(Space4).Items(ui.Center).
		Draw(drawTitlebarBorder).Children(func() {
		a.titlebarBrand(c)
		a.titlebarNavigation(c, false)
		ui.Row(c).Grow(1).FillHeight().MovesWindow()
	})
	ui.Row(c).FillWidth().Height(desktopToolbarHeight).Shrink(0).Background(a.palette.TableHeader).
		PadX(Space4).Gap(Space2).Items(ui.Center).Draw(drawTitlebarBorder).Children(func() {
		a.titlebarNewRequest(c)
		active := a.Active()
		canSave := a.Main == PageWorkbench && a.Section == SectionAPIs && active.Kind == requestTabKind && !a.requestPreviewOpen()
		label := a.S.Common.Save
		if ui.Button(c, label).Icon(icons.Save).IconOnly().Ghost().Tooltip(label).Disabled(!canSave).Clicked() {
			c.After(func() { a.RequestSave(active) })
		}
		a.importButton(c)
		ui.Divider(c).Vertical().Height(Space6)
		for _, section := range [...]Section{SectionAPIs, SectionEnvironments} {
			label := a.sectionLabel(section)
			button := ui.Button(c.KeyInt(int(section)), label).Icon(a.sectionIcon(section)).Ghost().Tooltip(label).Shrink(0)
			if a.Main == PageWorkbench && a.Section == section {
				button.Secondary().Background(a.palette.RailActive).Color(theme.AccentForeground)
			}
			if button.Clicked() {
				c.After(func() { a.SelectSection(section) })
			}
		}
		ui.Spacer(c)
		ui.Row(c).Shrink(1).MinWidth(0).Clip().Children(func() { a.searchBox(c) })
		a.envSelector(c)
		ui.Divider(c).Vertical().Height(Space6)
		a.titlebarThemeButton(c)
		a.settingsButton(c)
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
