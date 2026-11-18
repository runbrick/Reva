package gui

import (
	"github.com/runbrick/reva/internal/gui/themes"
	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

const (
	Space1 float32 = 4
	Space2 float32 = 8
	Space3 float32 = 12
	Space4 float32 = 16
	Space6 float32 = 24
)

const (
	TitlebarHeight float32 = 56
	RailWidth      float32 = 76
	SidebarWidth   float32 = 280
	TabBarHeight   float32 = 44
)

type Palette = themes.Palette

func (a *App) Palette() *Palette { return &a.palette }

func (a *App) desktopLayout() bool { return a.themeLayout == themes.LayoutDesktop }

func (a *App) studioLayout() bool { return a.themeLayout == themes.LayoutStudio }

const studioInspectorWidth = Space6 * 11

func (a *App) studioInspectorFits(c ui.C) bool {
	return a.studioLayout() && c.Bounds().Width-a.sideSplit.Pos-RailWidth >= studioInspectorWidth+Space6*29
}

func (a *App) railWidth() float32 {
	if a.desktopLayout() {
		return 0
	}
	return RailWidth
}

func (a *App) tabBarHeight() float32 {
	if a.desktopLayout() {
		return Space6 + Space4
	}
	return TabBarHeight
}

func (a *App) MethodBadge(c ui.C, method model.Method) ui.Box {
	theme := c.Theme()
	color := a.palette.MethodColor(method)
	return ui.Row(c).Width(Space6 * 2).Shrink(0).Items(ui.Center).Children(func() {
		ui.Text(c, string(method)).FontSize(theme.Metrics.FontSizeS).Bold().Color(color).
			Background(color.WithAlpha(0.10)).Radius(theme.Metrics.RadiusControl).PadX(Space1).Ellipsis()
	})
}

func statusDot(c ui.C, color ui.Color) {
	ui.Slot(c).Size(Space2, Space2).Radius(gfx.RadiusFull).Background(color).Shrink(0)
}
