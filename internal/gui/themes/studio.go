package themes

import "github.com/vivaui/viva/ui"

func studioTheme(dark bool) (ui.Theme, Palette) {
	theme, palette := modernTheme(dark)
	theme.Metrics.ControlHeight = 34
	theme.Metrics.RadiusSurface = 8
	if dark {
		theme.Background = ui.RGB(20, 22, 26)
		theme.Card = ui.RGB(27, 30, 35)
		theme.Popover = ui.RGB(35, 38, 43)
		theme.Foreground = ui.RGB(241, 242, 245)
		theme.MutedForeground = ui.RGB(165, 172, 184)
		theme.Primary = ui.RGB(255, 128, 38)
		theme.PrimaryForeground = ui.RGB(29, 20, 14)
		theme.Accent = ui.RGB(72, 47, 30)
		theme.AccentForeground = ui.RGB(255, 169, 96)
		theme.Border = ui.RGB(49, 53, 60)
		theme.Input = ui.RGB(64, 69, 79)
		theme.Muted = ui.RGB(31, 34, 39)
		theme.Secondary = ui.RGB(40, 43, 49)
		theme.Hover = ui.RGB(43, 46, 53)
		theme.Selection = ui.RGB(96, 57, 33)
		palette.Rail = ui.RGB(19, 21, 25)
		palette.Sidebar = ui.RGB(23, 25, 29)
		palette.TableHeader = ui.RGB(32, 35, 40)
		palette.CodeKey = ui.RGB(250, 141, 119)
		palette.CodeString = ui.RGB(126, 177, 255)
	} else {
		theme.Background = ui.RGB(247, 248, 251)
		theme.Card = ui.RGB(252, 253, 254)
		theme.Popover = ui.RGB(255, 255, 255)
		theme.Foreground = ui.RGB(25, 32, 43)
		theme.MutedForeground = ui.RGB(92, 101, 116)
		theme.Primary = ui.RGB(246, 113, 28)
		theme.PrimaryForeground = ui.RGB(29, 20, 14)
		theme.Accent = ui.RGB(253, 237, 222)
		theme.AccentForeground = ui.RGB(167, 63, 8)
		theme.Border = ui.RGB(226, 230, 237)
		theme.Input = ui.RGB(214, 220, 231)
		theme.Muted = ui.RGB(244, 246, 249)
		theme.Secondary = ui.RGB(239, 242, 246)
		theme.Hover = ui.RGB(239, 242, 247)
		theme.Selection = ui.RGB(255, 218, 183)
		theme.Success = ui.RGB(0, 128, 58)
		palette.Rail = ui.RGB(246, 248, 251)
		palette.Sidebar = ui.RGB(246, 247, 250)
		palette.TableHeader = ui.RGB(240, 242, 246)
		palette.Get = theme.Success
		palette.Post = theme.AccentForeground
		palette.CodeKey = ui.RGB(172, 46, 46)
		palette.CodeString = ui.RGB(23, 89, 180)
	}
	theme.Ring = theme.AccentForeground
	palette.Surface = theme.Background
	palette.RailActive = theme.Accent
	return theme, palette
}
