package gui

import (
	"github.com/runbrick/reqo/internal/model"
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
	TitlebarHeight float32 = 48  // custom titlebar strip
	RailWidth      float32 = 72  // left icon navigation rail
	SidebarWidth   float32 = 280 // initial width of the list pane
	TabBarHeight   float32 = 40
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
	t.Primary = ui.RGB(22, 119, 255)
	t.PrimaryForeground = ui.RGB(255, 255, 255)
	t.Ring = ui.RGB(22, 119, 255)
	t.Accent = ui.RGB(232, 241, 255)
	t.AccentForeground = ui.RGB(22, 119, 255)
	t.Border = ui.RGB(229, 232, 239)
	t.Input = ui.RGB(222, 226, 234)
	t.Muted = ui.RGB(244, 246, 250)
	t.MutedForeground = ui.RGB(110, 117, 132)
	t.Hover = ui.RGB(241, 244, 249)
	t.Secondary = ui.RGB(242, 244, 248)
	t.Selection = ui.RGB(200, 222, 255)
	t.Success = ui.RGB(22, 163, 74)
	t.Danger = ui.RGB(220, 38, 38)
	t.Metrics.RadiusControl = 8
	t.Metrics.RadiusSurface = 12
	t.Metrics.ControlHeight = 36
	return t, Palette{
		Rail: ui.RGB(240, 243, 248), Sidebar: ui.RGB(247, 249, 252), Surface: ui.RGB(255, 255, 255),
		RailActive: ui.RGB(226, 236, 252), TableHeader: ui.RGB(248, 250, 252),
		Get: ui.RGB(22, 119, 255), Post: ui.RGB(234, 128, 16), Put: ui.RGB(124, 58, 237),
		Patch: ui.RGB(13, 148, 136), Delete: ui.RGB(220, 38, 38), Head: ui.RGB(22, 163, 74),
		Options: ui.RGB(100, 116, 139), Warning: ui.RGB(217, 119, 6),
		CodeKey: ui.RGB(17, 24, 39), CodeString: ui.RGB(22, 119, 255), CodeNumber: ui.RGB(13, 148, 136),
		CodeLiteral: ui.RGB(124, 58, 237), CodePunct: ui.RGB(75, 85, 99), CodeLineNo: ui.RGB(156, 163, 175),
	}
}

func darkTheme() (ui.Theme, Palette) {
	t := ui.DarkTheme()
	t.Background = ui.RGB(24, 26, 31)
	t.Card = ui.RGB(30, 33, 40)
	t.Popover = ui.RGB(34, 37, 45)
	t.Primary = ui.RGB(64, 150, 255)
	t.PrimaryForeground = ui.RGB(255, 255, 255)
	t.Ring = ui.RGB(64, 150, 255)
	t.Accent = ui.RGB(30, 52, 86)
	t.AccentForeground = ui.RGB(120, 180, 255)
	t.Border = ui.RGB(48, 52, 62)
	t.Input = ui.RGB(56, 60, 72)
	t.Muted = ui.RGB(36, 39, 47)
	t.MutedForeground = ui.RGB(150, 156, 170)
	t.Hover = ui.RGB(40, 44, 54)
	t.Secondary = ui.RGB(40, 44, 54)
	t.Selection = ui.RGB(40, 72, 120)
	t.Success = ui.RGB(52, 199, 89)
	t.Danger = ui.RGB(248, 81, 73)
	t.Metrics.RadiusControl = 8
	t.Metrics.RadiusSurface = 12
	t.Metrics.ControlHeight = 36
	return t, Palette{
		Rail: ui.RGB(20, 22, 27), Sidebar: ui.RGB(27, 30, 36), Surface: ui.RGB(24, 26, 31),
		RailActive: ui.RGB(32, 50, 80), TableHeader: ui.RGB(30, 33, 40),
		Get: ui.RGB(77, 163, 255), Post: ui.RGB(251, 146, 60), Put: ui.RGB(167, 139, 250),
		Patch: ui.RGB(45, 212, 191), Delete: ui.RGB(248, 113, 113), Head: ui.RGB(74, 222, 128),
		Options: ui.RGB(148, 163, 184), Warning: ui.RGB(251, 191, 36),
		CodeKey: ui.RGB(226, 232, 240), CodeString: ui.RGB(125, 185, 255), CodeNumber: ui.RGB(94, 234, 212),
		CodeLiteral: ui.RGB(196, 181, 253), CodePunct: ui.RGB(160, 168, 184), CodeLineNo: ui.RGB(100, 108, 124),
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

// MethodBadge declares method m as colored bold text, fixed width so names
// line up in lists.
func (a *App) MethodBadge(c ui.C, m model.Method) ui.Box {
	theme := c.Theme()
	return ui.Text(c, string(m)).FontSize(theme.Metrics.FontSizeS).Bold().
		Color(a.palette.MethodColor(m)).Width(44).Shrink(0)
}
