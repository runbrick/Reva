package themes

import "github.com/vivaui/viva/ui"

func Resolve(dark bool) (ui.Theme, Palette) {
	theme := ui.LightTheme()
	palette := Palette{}
	if dark {
		theme = ui.DarkTheme()
	}
	theme.Metrics.ControlHeight = 34
	theme.Metrics.RadiusSurface = 8
	theme.Metrics.RadiusControl = 8
	if dark {
		theme.Background = ui.RGB(17, 21, 27)
		theme.Card = ui.RGB(31, 36, 45)
		theme.Foreground = ui.RGB(226, 231, 238)
		theme.PrimaryForeground = theme.Background
		theme.MutedForeground = ui.RGB(158, 168, 182)
		theme.Accent = ui.RGB(44, 50, 60)
		theme.Border = ui.RGB(51, 60, 74)
		theme.Input = ui.RGB(67, 78, 95)
		theme.Muted = ui.RGB(37, 43, 53)
		theme.Secondary = ui.RGB(60, 70, 87)
		theme.Selection = ui.RGB(110, 160, 240).WithAlpha(0.30)
		theme.Success = ui.RGB(98, 206, 136)
		theme.Danger = ui.RGB(240, 120, 120)
		palette.Rail = ui.RGB(24, 28, 35)
		palette.TableHeader = theme.Muted
		palette.Post = ui.RGB(255, 180, 87)
		palette.Put = ui.RGB(88, 166, 255)
		palette.Patch = ui.RGB(45, 212, 191)
		palette.Delete = ui.RGB(248, 113, 113)
		palette.Head = ui.RGB(74, 222, 128)
		palette.Warning = ui.RGB(232, 192, 72)
		palette.CodeKey = ui.RGB(250, 141, 119)
		palette.CodeString = ui.RGB(126, 177, 255)
		palette.CodeNumber = ui.RGB(105, 212, 230)
		palette.CodeLiteral = ui.RGB(196, 181, 253)
		palette.CodePunct = ui.RGB(225, 195, 132)
		palette.CodeLineNo = ui.RGB(132, 145, 167)
	} else {
		theme.Background = ui.RGB(240, 240, 243)
		theme.Card = ui.RGB(248, 248, 250)
		theme.Foreground = ui.RGB(38, 38, 42)
		theme.PrimaryForeground = theme.Card
		theme.MutedForeground = ui.RGB(94, 94, 100)
		theme.Accent = ui.RGB(226, 226, 231)
		theme.Border = ui.RGB(220, 220, 225)
		theme.Input = ui.RGB(204, 204, 210)
		theme.Muted = ui.RGB(235, 235, 239)
		theme.Secondary = ui.RGB(221, 221, 226)
		theme.Selection = ui.RGB(36, 98, 198).WithAlpha(0.22)
		theme.Success = ui.RGB(0, 128, 58)
		theme.Danger = ui.RGB(214, 40, 40)
		palette.Rail = ui.RGB(231, 231, 235)
		palette.TableHeader = theme.Muted
		palette.Post = ui.RGB(167, 63, 8)
		palette.Put = ui.RGB(23, 101, 197)
		palette.Patch = ui.RGB(13, 148, 136)
		palette.Delete = ui.RGB(220, 38, 38)
		palette.Head = ui.RGB(22, 163, 74)
		palette.Warning = ui.RGB(150, 90, 4)
		palette.CodeKey = ui.RGB(172, 46, 46)
		palette.CodeString = ui.RGB(23, 89, 180)
		palette.CodeNumber = ui.RGB(13, 148, 136)
		palette.CodeLiteral = ui.RGB(124, 58, 237)
		palette.CodePunct = ui.RGB(75, 85, 99)
		palette.CodeLineNo = ui.RGB(134, 134, 140)
	}
	theme.Primary = theme.Foreground
	theme.Ring = theme.Primary
	theme.AccentForeground = theme.Foreground
	theme.CardForeground = theme.Foreground
	theme.CardElevated = theme.Card
	theme.Popover = theme.Card
	theme.PopoverForeground = theme.Foreground
	theme.SecondaryForeground = theme.Foreground
	theme.SelectionForeground = theme.Foreground
	theme.Hover = theme.Muted
	palette.Get = theme.Success
	palette.Options = theme.MutedForeground
	palette.Sidebar = palette.Rail
	palette.Surface = theme.Background
	palette.RailActive = theme.Accent
	return theme, palette
}
