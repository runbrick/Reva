package gui

import (
	"strconv"
	"strings"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func (a *App) studioRail(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Width(RailWidth).FillHeight().Shrink(0).Background(a.palette.Rail).PadY(Space4).
		Gap(Space3).Items(ui.Center).FocusGroup(ui.FocusVertical).Label(a.S.Shell.RailLabel).Children(func() {
		a.studioRailButton(c.Key("requests"), a.S.Shell.APIs, icons.Send, a.Main == PageWorkbench && a.Section == SectionAPIs, func() { a.SelectSection(SectionAPIs) })
		a.studioRailButton(c.Key("environments"), a.S.Shell.Environments, icons.Layers, a.Main == PageWorkbench && a.Section == SectionEnvironments, a.openEnvironments)
		a.studioRailButton(c.Key("history"), a.S.Shell.History, icons.History, a.Main == PageHistory, func() { a.Main = PageHistory })
		a.studioRailButton(c.Key("toolbox"), a.S.Toolbox.Title, icons.Package, a.Main == PageToolbox, func() { a.Main = PageToolbox })
		a.studioRailButton(c.Key("settings"), a.S.Settings.Title, icons.Settings, a.Main == PageSettings, a.openSettings)
		if ui.Shortcut(c, ui.Primary, ",") {
			c.After(a.openSettings)
		}
		ui.Spacer(c)
		ui.Row(c).Radius(theme.Metrics.RadiusControl).Border(1, theme.Border).Children(func() { a.titlebarThemeButton(c) })
	})
	ui.Divider(c).Vertical()
}

func (a *App) studioRailButton(c ui.C, label string, icon gfx.Icon, selected bool, action func()) {
	theme := c.Theme()
	color := theme.MutedForeground
	hover := ui.Look{}.Background(theme.Hover)
	if selected {
		color = theme.Primary
		hover = ui.Look{}.Background(theme.Accent).Border(theme.Primary.WithAlpha(0.45))
	}
	button := ui.Row(c).Size(Space6*2, Space6*2).Center().Radius(theme.Metrics.RadiusControl).
		HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleButton, Selected: selected}).Tooltip(label).
		Hover(hover).Focus(ui.Look{}.Ring(2, theme.Ring))
	if selected {
		button.Background(a.palette.RailActive).Border(1, theme.Primary.WithAlpha(0.24))
	}
	button.Children(func() {
		glyph := ui.Icon(c, icon).Size(Space6, Space6).Color(color)
		if !selected {
			glyph.StateFrom(button).Hover(ui.Look{}.Color(theme.Foreground)).Focus(ui.Look{}.Color(theme.Foreground))
		}
	})
	if button.Clicked() {
		c.After(action)
	}
}

func studioCard(c ui.C, title string, icon gfx.Icon, body func()) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Shrink(0).Pad(Space3).Gap(Space2).Background(theme.Card).
		Border(1, theme.Border).Radius(theme.Metrics.RadiusSurface).Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Gap(Space2).Children(func() {
			ui.Icon(c, icon).Size(Space4, Space4).Color(theme.Foreground)
			ui.Text(c, title).Bold().Grow(1).Ellipsis()
		})
		ui.Divider(c)
		body()
	})
}

func studioDetail(c ui.C, label, value string) {
	ui.Row(c).FillWidth().Gap(Space3).Items(ui.Center).Children(func() {
		ui.Text(c, label).Width(Space6 * 4).Shrink(0).Muted().Ellipsis()
		ui.Text(c, value).Grow(1).MinWidth(0).Ellipsis()
	})
}

func (a *App) studioRequestInspector(c ui.C, tab *Tab) {
	theme, labels := c.Theme(), a.S.Request
	ui.Scroll(c).Width(studioInspectorWidth).FillHeight().Shrink(0).Scrollbars().Children(func() {
		ui.Column(c).FillWidth().Pad(Space1).PadRight(Space2).Gap(Space3).Children(func() {
			studioCard(c, labels.Details, icons.Info, func() {
				ui.Row(c).FillWidth().Items(ui.Center).Gap(Space1).Children(func() {
					a.MethodBadge(c, tab.Request.Method)
					ui.Text(c, a.requestTabTitle(tab)).Bold().Grow(1).Ellipsis()
				})
				if collection, found := a.Svc.Collections.Get(tab.CollectionID); found {
					studioDetail(c, a.S.Collections.Collection, collection.Name)
				}
				if env, found := a.activeEnvironment(a.environmentCollection()); found {
					studioDetail(c, a.S.Env.Title, env.Name)
				}
				studioDetail(c, labels.Params, strconv.Itoa(len(tab.Request.Params)))
				studioDetail(c, labels.Headers, strconv.Itoa(len(tab.Request.Headers)))
				if ui.Button(c, labels.Preview).Icon(icons.Eye).Outline().FillWidth().Clicked() {
					c.After(func() { a.requestTab(tab).previewOpen = true })
				}
			})
			if response := a.response.views[tab.ID]; response != nil && response.source == tab.Response && !tab.Sending && tab.Err == "" {
				studioCard(c, a.S.Response.Title, icons.Braces, func() {
					statusColor := a.palette.StatusColor(theme, tab.Response.StatusCode)
					ui.Text(c, tab.Response.Status).Bold().Color(statusColor).Ellipsis()
					studioDetail(c, a.S.Response.Duration, response.duration)
					studioDetail(c, a.S.Response.Size, response.size)
					if response.json && response.pretty != "" {
						ui.Paragraph(c, studioResponseExcerpt(response.pretty), 10).Mono().FillWidth().Pad(Space2).
							FontSize(theme.Metrics.FontSizeS).Background(theme.Muted).Radius(theme.Metrics.RadiusControl)
					}
				})
			}
		})
	})
}

func studioResponseExcerpt(value string) string {
	lines := 0
	for index, char := range value {
		if char == '\n' {
			lines++
		}
		if lines >= 8 || index >= 480 {
			return strings.TrimSpace(value[:index]) + "…"
		}
	}
	return value
}
