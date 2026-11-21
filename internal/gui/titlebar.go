package gui

import (
	"runtime"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// titlebarState is retained for the shell's feature state slot.
type titlebarState struct{ collectionsOpen bool }

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
	activeID := a.workspaceCollectionID()
	name := labels.Title
	if collection, found := a.Svc.Collections.Get(activeID); found {
		name = collection.Name
	}
	trigger := ui.ButtonBase(c, labels.Collection).Width(Space6*7).Height(theme.Metrics.ControlHeight).Shrink(0).
		Radius(theme.Metrics.RadiusControl).Border(1, theme.Border).Background(theme.Card).
		PadX(Space2).Gap(Space2).Tooltip(name).Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring)).Children(func() {
		ui.Icon(c, icons.Folder).Size(Space4, Space4).Color(theme.MutedForeground)
		ui.Text(c, name).Grow(1).Ellipsis()
		ui.Icon(c, icons.ChevronDown).Size(Space4, Space4).Color(theme.MutedForeground)
	})
	if trigger.Clicked() {
		a.titlebar.collectionsOpen = !a.titlebar.collectionsOpen
	}
	ui.Menu(c, trigger, &a.titlebar.collectionsOpen, func(menu *ui.MenuItems) {
		menu.WidthRange(Space6*7, Space6*16)
		for _, collection := range a.Svc.Collections.List() {
			if menu.Item(collection.Name).Icon(icons.Folder).Chosen(collection.ID == activeID).Clicked() {
				c.After(func() { a.focusWorkspaceCollection(collection.ID) })
			}
		}
		if activeID != "" {
			menu.Separator()
			if menu.Item(labels.Settings).Icon(icons.Settings).Clicked() {
				c.After(func() {
					a.openCollectionSettings(collectionNode{collectionID: activeID}, collectionSettingsGeneral)
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
	ui.Row(c).Gap(Space2).Items(ui.Center).Shrink(0).Children(func() {
		ui.Row(c).Size(Space6+Space1, Space6+Space1).Radius(theme.Metrics.RadiusControl).Background(theme.Primary).
			Center().Children(func() {
			ui.Icon(c, icons.Send).Size(Space4, Space4).Color(theme.PrimaryForeground)
		})
		ui.Text(c, a.S.Common.AppName).Bold().FontSize(theme.Metrics.FontSizeL - Space1)
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
