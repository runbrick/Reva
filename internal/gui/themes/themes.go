// Package themes registers Reva's built-in colors, metrics and layout profiles.
package themes

import (
	"strings"

	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/ui"
)

type Style string

const (
	Modern  Style = "modern"
	Classic Style = "classic"
	Studio  Style = "studio"
)

type Layout uint8

const (
	LayoutRail Layout = iota
	LayoutDesktop
	LayoutStudio
)

type definition struct {
	style  Style
	layout Layout
	paint  func(dark bool) (ui.Theme, Palette)
}

var builtins = [...]definition{
	{Modern, LayoutRail, modernTheme},
	{Classic, LayoutDesktop, classicTheme},
	{Studio, LayoutStudio, studioTheme},
}

func Styles() []Style {
	styles := make([]Style, len(builtins))
	for index, entry := range builtins {
		styles[index] = entry.style
	}
	return styles
}

func lookup(style Style) definition {
	for _, entry := range builtins {
		if entry.style == style {
			return entry
		}
	}
	return builtins[0]
}

func Resolve(style Style, dark bool) (ui.Theme, Palette, Layout) {
	entry := lookup(style)
	theme, palette := entry.paint(dark)
	return theme, palette, entry.layout
}

// Decode preserves the unprefixed settings used before style selection existed.
func Decode(value string) (Style, string) {
	style, mode, qualified := strings.Cut(value, ":")
	if !qualified {
		return Modern, normalizeMode(value)
	}
	return lookup(Style(style)).style, normalizeMode(mode)
}

func Encode(style Style, mode string) string {
	mode = normalizeMode(mode)
	if entry := lookup(style); entry.style != Modern {
		return string(entry.style) + ":" + mode
	}
	return mode
}

func normalizeMode(mode string) string {
	switch mode {
	case "dark", "system":
		return mode
	default:
		return "light"
	}
}

type Palette struct {
	Rail, Sidebar, Surface, RailActive, TableHeader                     ui.Color
	Get, Post, Put, Patch, Delete, Head, Options, Warning               ui.Color
	CodeKey, CodeString, CodeNumber, CodeLiteral, CodePunct, CodeLineNo ui.Color
}

func (p *Palette) MethodColor(method model.Method) ui.Color {
	switch method {
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
