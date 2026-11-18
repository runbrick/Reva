package gui

import (
	"fmt"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/gui/themes"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type studioSettingsSection int

const (
	studioAppearance studioSettingsSection = iota
	studioRequests
	studioData
	studioAbout
)

func (a *App) studioSettingsPage(c ui.C, mode string, appearance, requests, data, about func()) {
	theme, labels := c.Theme(), a.S.Settings
	sections := [...]struct {
		label string
		icon  gfx.Icon
		view  func()
	}{
		{labels.Appearance, icons.Sun, appearance},
		{labels.Requests, icons.Send, requests},
		{labels.Data, icons.HardDrive, data},
		{labels.About, icons.Info, about},
	}
	ui.Row(c).Fill().Children(func() {
		ui.Column(c).Width(Space6 * 8).Shrink(0).FillHeight().Pad(Space3).Gap(Space2).
			Background(a.palette.Sidebar).FocusGroup(ui.FocusVertical).Children(func() {
			ui.Text(c, labels.Title).FontSize(theme.Metrics.FontSizeL).Bold().Pad(Space2)
			ui.Divider(c)
			for index, section := range sections {
				selected := a.settings.section == studioSettingsSection(index)
				color := theme.Foreground
				button := ui.ButtonBase(c.KeyInt(index), section.label).FillWidth().Height(theme.Metrics.ControlHeight).
					Justify(ui.Start).Gap(Space2).PadX(Space3).Radius(theme.Metrics.RadiusControl).
					Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring))
				if selected {
					button.Background(theme.Accent)
					color = theme.AccentForeground
				}
				button.Children(func() {
					ui.Icon(c, section.icon).Size(Space4, Space4).Color(color)
					ui.Text(c, section.label).Color(color).Ellipsis()
				})
				if button.Clicked() {
					c.After(func() { a.settings.section = studioSettingsSection(index) })
				}
			}
		})
		ui.Divider(c).Vertical()
		section := a.settings.section
		ui.Scroll(c.KeyInt(int(section))).Grow(1).MinWidth(0).FillHeight().Scrollbars().Children(func() {
			ui.Column(c).FillWidth().Pad(Space4).Gap(Space4).Children(func() {
				ui.Row(c).FillWidth().Items(ui.Center).Gap(Space2).Children(func() {
					ui.Text(c, sections[section].label).FontSize(theme.Metrics.FontSizeL).Bold().Grow(1).Ellipsis()
					a.onboardingEntry(c)
				})
				if section != studioAppearance {
					sections[section].view()
					return
				}
				if c.Bounds().Width >= Space6*55 {
					ui.Row(c).FillWidth().Items(ui.Start).Gap(Space3).Children(func() {
						ui.Column(c.Key("preferences")).Basis(0).Grow(1).MinWidth(0).Gap(Space3).Children(func() {
							appearance()
							requests()
						})
						ui.Column(c.Key("preview")).Basis(0).Grow(1).MinWidth(0).Gap(Space3).Children(func() {
							a.studioThemePreview(c, mode)
							about()
							data()
						})
					})
					return
				}
				appearance()
				a.studioThemePreview(c, mode)
			})
		})
	})
}

func (a *App) studioThemePreview(c ui.C, mode string) {
	theme, labels := c.Theme(), a.S.Settings
	studioCard(c, labels.ThemePreview, icons.Eye, func() {
		ui.Row(c).FillWidth().Gap(Space3).FocusGroup(ui.FocusHorizontal).Children(func() {
			for index, option := range settingsThemes[:2] {
				label := a.settingsThemeLabel(option)
				previewTheme, palette, _ := themes.Resolve(themes.Studio, option == "dark")
				selected := mode == option || mode == "system" && a.Dark() == (option == "dark")
				kc := c.KeyInt(index)
				tile := ui.Column(kc).Basis(0).Grow(1).MinWidth(0).Pad(Space1).Gap(Space1).Items(ui.Center).
					Radius(theme.Metrics.RadiusSurface).HitWith(ui.ID(kc), fmt.Sprintf(labels.PreviewTheme, label), gfx.HitOptions{Role: gfx.RoleButton, Selected: selected}).
					Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring))
				if selected {
					tile.Border(1, theme.Primary).Background(theme.Accent)
				}
				tile.Children(func() {
					ui.Slot(c).FillWidth().Height(Space6 * 4).Radius(theme.Metrics.RadiusSurface).Clip().
						Draw(func(context *ui.DrawContext) { drawStudioThemePreview(context, previewTheme, palette) })
					ui.Text(c, label).FontSize(theme.Metrics.FontSizeS).Bold().Ellipsis()
				})
				if tile.Clicked() {
					current := a.Svc.Settings.Get()
					current.Theme = themes.Encode(themes.Studio, option)
					a.saveSettingsAfter(c, current)
				}
			}
		})
	})
}

func drawStudioThemePreview(context *ui.DrawContext, theme ui.Theme, palette Palette) {
	frame := context.Rect
	fill := func(left, top, width, height float32, color ui.Color) {
		context.Canvas.Fill(gfx.Rect{X: frame.X + left, Y: frame.Y + top, Width: width, Height: height}, color)
	}
	fill(0, 0, frame.Width, frame.Height, theme.Background)
	fill(0, 0, frame.Width, Space4, theme.Card)
	fill(Space1, Space1, Space2, Space2, theme.Primary)
	fill(Space4, Space1, frame.Width*0.36, Space2, theme.Secondary)
	fill(0, Space4, Space6*2, frame.Height-Space4, palette.Sidebar)
	fill(0, Space4, Space4, frame.Height-Space4, palette.Rail)
	for index := range 3 {
		fill(Space1, Space6+float32(index)*Space3, Space2, Space1, theme.MutedForeground.WithAlpha(0.6))
		fill(Space6, Space6+float32(index)*Space3, Space4, Space1, theme.Input)
	}
	left := Space6*2 + Space1
	width := max(0, frame.Width-left-Space1)
	fill(left, Space6, width, Space2, theme.Secondary)
	fill(left+width*0.76, Space6, width*0.24, Space2, theme.Primary)
	fill(left, Space6+Space3, width, Space4, theme.Card)
	fill(left, Space6*2+Space1, width, Space6+Space3, theme.Card)
	fill(left+width*0.10, Space6*2+Space2, width*0.30, Space1, palette.CodeKey)
	fill(left+width*0.16, Space6*2+Space4, width*0.48, Space1, palette.CodeString)
	fill(left+width*0.10, Space6*3, width*0.24, Space1, palette.CodeNumber)
}
