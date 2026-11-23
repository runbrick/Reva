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
		theme.Background = ui.RGB(18, 24, 34)
		theme.Card = ui.RGB(31, 40, 54)
		theme.Foreground = ui.RGB(232, 238, 247)
		theme.Primary = ui.RGB(104, 171, 255)
		theme.PrimaryForeground = ui.RGB(10, 16, 26)
		theme.MutedForeground = ui.RGB(156, 171, 192)
		theme.Accent = ui.RGB(27, 49, 82)
		theme.AccentForeground = theme.Foreground
		theme.Border = ui.RGB(53, 67, 88)
		theme.Input = ui.RGB(72, 89, 115)
		theme.Muted = ui.RGB(35, 45, 61)
		theme.Hover = ui.RGB(45, 57, 76)
		theme.Secondary = ui.RGB(49, 62, 82)
		theme.Selection = theme.Primary.WithAlpha(0.28)
		theme.Success = ui.RGB(82, 214, 138)
		theme.Danger = ui.RGB(255, 122, 126)
		theme.DangerForeground = ui.RGB(10, 16, 26)
		palette.Rail = ui.RGB(13, 17, 23)
		palette.Sidebar = ui.RGB(24, 31, 43)
		palette.RailActive = ui.RGB(23, 44, 76)
		palette.TableHeader = ui.RGB(39, 50, 67)
		palette.Post = ui.RGB(251, 176, 82)
		palette.Put = ui.RGB(96, 179, 255)
		palette.Patch = ui.RGB(62, 216, 198)
		palette.Delete = ui.RGB(255, 126, 126)
		palette.Head = ui.RGB(186, 152, 255)
		palette.Warning = ui.RGB(245, 198, 76)
		palette.CodeKey = ui.RGB(255, 148, 134)
		palette.CodeString = ui.RGB(126, 186, 255)
		palette.CodeNumber = ui.RGB(94, 218, 224)
		palette.CodeLiteral = ui.RGB(204, 174, 255)
		palette.CodePunct = ui.RGB(214, 192, 140)
		palette.CodeLineNo = ui.RGB(130, 146, 170)
	} else {
		theme.Background = ui.RGB(241, 245, 249)
		theme.Card = ui.RGB(255, 255, 255)
		theme.Foreground = ui.RGB(15, 23, 42)
		theme.Primary = ui.RGB(22, 86, 214)
		theme.PrimaryForeground = ui.RGB(255, 255, 255)
		theme.MutedForeground = ui.RGB(71, 85, 105)
		theme.Accent = ui.RGB(224, 236, 255)
		theme.AccentForeground = ui.RGB(15, 38, 86)
		theme.Border = ui.RGB(210, 219, 231)
		theme.Input = ui.RGB(192, 204, 219)
		theme.Muted = ui.RGB(236, 241, 247)
		theme.Hover = ui.RGB(218, 227, 239)
		theme.Secondary = ui.RGB(226, 233, 242)
		theme.Selection = theme.Primary.WithAlpha(0.20)
		theme.Success = ui.RGB(11, 112, 56)
		theme.Danger = ui.RGB(185, 28, 28)
		theme.DangerForeground = ui.RGB(255, 255, 255)
		palette.Rail = ui.RGB(228, 235, 244)
		palette.Sidebar = ui.RGB(248, 250, 252)
		palette.RailActive = ui.RGB(216, 232, 255)
		palette.TableHeader = ui.RGB(234, 240, 247)
		palette.Post = ui.RGB(153, 65, 6)
		palette.Put = ui.RGB(20, 88, 196)
		palette.Patch = ui.RGB(11, 108, 104)
		palette.Delete = ui.RGB(185, 28, 28)
		palette.Head = ui.RGB(98, 54, 196)
		palette.Warning = ui.RGB(146, 78, 4)
		palette.CodeKey = ui.RGB(168, 36, 46)
		palette.CodeString = ui.RGB(20, 82, 184)
		palette.CodeNumber = ui.RGB(11, 108, 100)
		palette.CodeLiteral = ui.RGB(109, 40, 217)
		palette.CodePunct = ui.RGB(71, 85, 105)
		palette.CodeLineNo = ui.RGB(100, 116, 139)
	}
	theme.Ring = theme.Primary
	theme.CardForeground = theme.Foreground
	theme.CardElevated = theme.Card
	theme.Popover = theme.Card
	theme.PopoverForeground = theme.Foreground
	theme.SecondaryForeground = theme.Foreground
	theme.SelectionForeground = theme.Foreground
	palette.Get = theme.Success
	palette.Options = theme.MutedForeground
	palette.Surface = theme.Background
	return theme, palette
}
