// Package themes defines Reva's light and dark appearance.
package themes

import (
	"strings"

	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/ui"
)

// Mode preserves the appearance in settings saved with a layout prefix.
func Mode(value string) string {
	if _, mode, qualified := strings.Cut(value, ":"); qualified {
		value = mode
	}
	switch value {
	case "dark", "system":
		return value
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
