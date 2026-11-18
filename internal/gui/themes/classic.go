package themes

import "github.com/vivaui/viva/ui"

func classicTheme(dark bool) (ui.Theme, Palette) {
	theme, palette := modernTheme(dark)
	theme.Metrics.ControlHeight = 32
	theme.Metrics.RadiusControl = 4
	theme.Metrics.RadiusSurface = 4
	theme.Metrics.PadControlX = 8
	theme.Metrics.GapItem = 4
	if dark {
		theme.Background = ui.RGB(30, 32, 36)
		theme.Card = ui.RGB(35, 37, 41)
		theme.Popover = ui.RGB(40, 43, 49)
		theme.Border = ui.RGB(63, 68, 77)
		theme.Input = ui.RGB(80, 86, 96)
		theme.Muted = ui.RGB(41, 44, 50)
		theme.Secondary = ui.RGB(47, 50, 55)
		theme.Hover = ui.RGB(52, 57, 66)
		theme.Primary = ui.RGB(25, 98, 229)
		theme.Ring = ui.RGB(122, 177, 255)
		theme.Accent = ui.RGB(34, 59, 92)
		theme.AccentForeground = theme.Ring
		theme.Selection = ui.RGB(42, 72, 115)
		palette.Rail = ui.RGB(36, 39, 44)
		palette.Sidebar = ui.RGB(33, 35, 40)
		palette.TableHeader = theme.Muted
	} else {
		theme.Background = ui.RGB(255, 255, 255)
		theme.Card = ui.RGB(252, 252, 253)
		theme.Border = ui.RGB(208, 214, 223)
		theme.Input = ui.RGB(204, 211, 222)
		theme.Muted = ui.RGB(241, 243, 246)
		theme.Secondary = ui.RGB(235, 238, 242)
		theme.Hover = ui.RGB(229, 235, 244)
		theme.Primary = ui.RGB(0, 92, 230)
		theme.Ring = theme.Primary
		theme.Accent = ui.RGB(216, 233, 253)
		theme.AccentForeground = ui.RGB(0, 74, 185)
		theme.Selection = ui.RGB(190, 219, 254)
		palette.Rail = ui.RGB(235, 237, 241)
		palette.Sidebar = ui.RGB(247, 248, 250)
		palette.TableHeader = ui.RGB(240, 242, 245)
		palette.CodeKey = ui.RGB(0, 61, 165)
		palette.CodeString = ui.RGB(167, 60, 11)
		palette.CodeNumber = ui.RGB(10, 106, 74)
		palette.CodeLineNo = theme.MutedForeground
	}
	palette.Surface = theme.Background
	palette.RailActive = theme.Accent
	return theme, palette
}
