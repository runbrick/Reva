package gui

import (
	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Design tokens shared by every pane. Panes take colors from the theme and
// these helpers, and lengths from Space*, never from literals of their own.
// Owned by the shell; a feature PR that needs a new token asks for it.

// Spacing scale, in logical pixels (multiples of 4).
const (
	Space1 float32 = 4
	Space2 float32 = 8
	Space3 float32 = 12
	Space4 float32 = 16
	Space6 float32 = 24
)

// Fixed sizes of the window shell.
const (
	TitlebarHeight float32 = 56  // custom titlebar strip
	RailWidth      float32 = 76  // left icon navigation rail
	SidebarWidth   float32 = 280 // initial width of the list pane
	TabBarHeight   float32 = 44
)

// Palette is the part of the look the Viva theme has no field for.
type Palette struct {
	// Rail is the navigation rail and titlebar background; Sidebar the list
	// pane beside it; Surface the editor area.
	Rail, Sidebar, Surface ui.Color
	// RailActive is the active rail item's background.
	RailActive ui.Color
	// TableHeader is the background of table header rows.
	TableHeader ui.Color
	// Methods color HTTP method badges.
	Get, Post, Put, Patch, Delete, Head, Options ui.Color
	// Warning is between success and danger (4xx, unresolved variables).
	Warning ui.Color
	// Code colors JSON syntax: keys, strings, numbers, literals, punctuation.
	CodeKey, CodeString, CodeNumber, CodeLiteral, CodePunct, CodeLineNo ui.Color
}

func lightTheme() (ui.Theme, Palette) {
	t := ui.LightTheme()
	t.Background = ui.RGB(255, 255, 255)
	t.Primary = ui.RGB(29, 96, 222)
	t.PrimaryForeground = ui.RGB(255, 255, 255)
	t.Ring = t.Primary
	t.Accent = ui.RGB(232, 241, 255)
	t.AccentForeground = t.Primary
	t.Border = ui.RGB(229, 232, 239)
	t.Input = ui.RGB(222, 226, 234)
	t.Muted = ui.RGB(244, 246, 250)
	t.MutedForeground = ui.RGB(110, 117, 132)
	t.Hover = ui.RGB(241, 244, 249)
	t.Secondary = ui.RGB(242, 244, 248)
	t.Selection = ui.RGB(200, 222, 255)
	t.Success = ui.RGB(17, 117, 79)
	t.Danger = ui.RGB(220, 38, 38)
	t.Metrics.RadiusControl = 8
	t.Metrics.RadiusSurface = 12
	t.Metrics.ControlHeight = 36
	return t, Palette{
		Rail: ui.RGB(240, 243, 248), Sidebar: ui.RGB(247, 249, 252), Surface: ui.RGB(255, 255, 255),
		RailActive: ui.RGB(226, 236, 252), TableHeader: ui.RGB(248, 250, 252),
		Get: t.Success, Post: ui.RGB(155, 83, 12), Put: ui.RGB(23, 101, 197),
		Patch: ui.RGB(13, 148, 136), Delete: ui.RGB(220, 38, 38), Head: ui.RGB(22, 163, 74),
		Options: ui.RGB(100, 116, 139), Warning: ui.RGB(217, 119, 6),
		CodeKey: ui.RGB(17, 24, 39), CodeString: ui.RGB(22, 119, 255), CodeNumber: ui.RGB(13, 148, 136),
		CodeLiteral: ui.RGB(124, 58, 237), CodePunct: ui.RGB(75, 85, 99), CodeLineNo: ui.RGB(156, 163, 175),
	}
}

func darkTheme() (ui.Theme, Palette) {
	t := ui.DarkTheme()
	t.Background = ui.RGB(17, 20, 27)
	t.Foreground = ui.RGB(229, 234, 243)
	t.Card = ui.RGB(23, 27, 36)
	t.Popover = ui.RGB(29, 34, 45)
	t.Primary = ui.RGB(34, 100, 229)
	t.PrimaryForeground = ui.RGB(255, 255, 255)
	t.Ring = ui.RGB(80, 151, 255)
	t.Accent = ui.RGB(24, 37, 62)
	t.AccentForeground = ui.RGB(98, 166, 255)
	t.Border = ui.RGB(37, 42, 54)
	t.Input = ui.RGB(53, 60, 76)
	t.Muted = ui.RGB(25, 29, 38)
	t.MutedForeground = ui.RGB(160, 170, 189)
	t.Hover = ui.RGB(34, 40, 53)
	t.Secondary = ui.RGB(34, 39, 51)
	t.Selection = ui.RGB(40, 72, 120)
	t.Success = ui.RGB(53, 216, 149)
	t.Danger = ui.RGB(248, 81, 73)
	t.Metrics.RadiusControl = 8
	t.Metrics.RadiusSurface = 12
	t.Metrics.ControlHeight = 36
	return t, Palette{
		Rail: ui.RGB(21, 25, 34), Sidebar: ui.RGB(16, 19, 26), Surface: t.Background,
		RailActive: t.Accent, TableHeader: t.Card,
		Get: t.Success, Post: ui.RGB(255, 180, 87), Put: ui.RGB(88, 166, 255),
		Patch: ui.RGB(45, 212, 191), Delete: ui.RGB(248, 113, 113), Head: ui.RGB(74, 222, 128),
		Options: ui.RGB(148, 163, 184), Warning: ui.RGB(251, 191, 36),
		CodeKey: ui.RGB(103, 179, 255), CodeString: ui.RGB(255, 170, 108), CodeNumber: ui.RGB(105, 212, 230),
		CodeLiteral: ui.RGB(196, 181, 253), CodePunct: ui.RGB(225, 195, 132), CodeLineNo: ui.RGB(132, 145, 167),
	}
}

// Palette returns the palette of the theme the App paints with.
func (a *App) Palette() *Palette { return &a.palette }

// MethodColor is the badge color of method m.
func (p *Palette) MethodColor(m model.Method) ui.Color {
	switch m {
	case model.MethodGet:
		return p.Get
	case model.MethodPost:
		return p.Post
	case model.MethodPut:
		return p.Put
	case model.MethodPatch:
		return p.Patch
	case model.MethodDelete:
		return p.Delete
	case model.MethodHead:
		return p.Head
	default:
		return p.Options
	}
}

// StatusColor is the color of an HTTP status code (0 means a transport error).
func (p *Palette) StatusColor(theme *ui.Theme, code int) ui.Color {
	switch {
	case code >= 200 && code < 300:
		return theme.Success
	case code >= 300 && code < 400:
		return p.Options
	case code >= 400 && code < 500:
		return p.Warning
	default:
		return theme.Danger
	}
}

// MethodBadge keeps request names aligned and colors the chip by method.
func (a *App) MethodBadge(c ui.C, m model.Method) ui.Box {
	theme := c.Theme()
	color := a.palette.MethodColor(m)
	return ui.Row(c).Width(Space6 * 2).Shrink(0).Items(ui.Center).Children(func() {
		ui.Text(c, string(m)).FontSize(theme.Metrics.FontSizeS).Bold().Color(color).
			Background(color.WithAlpha(0.10)).Radius(theme.Metrics.RadiusControl).PadX(Space1).Ellipsis()
	})
}

func statusDot(c ui.C, color ui.Color) {
	ui.Slot(c).Size(Space2, Space2).Radius(gfx.RadiusFull).Background(color).Shrink(0)
}
