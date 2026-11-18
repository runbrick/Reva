package themes

import "github.com/vivaui/viva/ui"

func modernTheme(dark bool) (ui.Theme, Palette) {
	if dark {
		return darkTheme()
	}
	return lightTheme()
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
