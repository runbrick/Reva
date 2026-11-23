package themes

import (
	"math"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestAppearanceModeReadsLegacySettings(t *testing.T) {
	for stored, want := range map[string]string{
		"": "light", "light": "light", "dark": "dark", "system": "system",
		"modern:light": "light", "classic:dark": "dark", "studio:system": "system",
		"future:dark": "dark", "classic:future": "light",
	} {
		if got := Mode(stored); got != want {
			t.Errorf("Mode(%q) = %q, want %q", stored, got, want)
		}
	}
}

func TestAppearanceChangesColorsWithoutChangingMetrics(t *testing.T) {
	light, lightPalette := Resolve(false)
	dark, darkPalette := Resolve(true)
	if light.Metrics != dark.Metrics {
		t.Fatal("appearance changed layout metrics")
	}
	if light.Primary == dark.Primary {
		t.Fatal("light and dark share the same primary color")
	}
	if light.Primary == light.Foreground || dark.Primary == dark.Foreground {
		t.Fatal("primary action still mirrors the neutral foreground")
	}
	if !isBlueAccent(light.Primary) || !isBlueAccent(dark.Primary) {
		t.Fatalf("primary actions must use blue accents: light=%+v dark=%+v", light.Primary, dark.Primary)
	}
	if lightPalette.Get == lightPalette.Post || darkPalette.Get == darkPalette.Post {
		t.Fatal("HTTP methods lost their distinct colors")
	}
}

func TestThemeHierarchyAndContrast(t *testing.T) {
	for _, variant := range []struct {
		name string
		dark bool
	}{
		{name: "light", dark: false},
		{name: "dark", dark: true},
	} {
		t.Run(variant.name, func(t *testing.T) {
			theme, palette := Resolve(variant.dark)

			if palette.Surface != theme.Background {
				t.Errorf("palette.Surface %+v != theme.Background %+v", palette.Surface, theme.Background)
			}
			assertDistinctColors(t, "surfaces", []namedColor{
				{"rail", palette.Rail},
				{"sidebar", palette.Sidebar},
				{"surface", palette.Surface},
				{"card", theme.Card},
				{"tableHeader", palette.TableHeader},
			}, 0.02)

			if palette.RailActive == palette.Rail || palette.RailActive == theme.Hover {
				t.Errorf("rail active %+v must differ from rail %+v and hover %+v", palette.RailActive, palette.Rail, theme.Hover)
			}
			if palette.RailActive.Blue <= palette.RailActive.Red {
				t.Errorf("rail active %+v should carry a blue tint", palette.RailActive)
			}
			if !isBlueAccent(theme.Ring) {
				t.Errorf("focus ring %+v should be a blue accent", theme.Ring)
			}

			assertContrast(t, "primary button", theme.PrimaryForeground, theme.Primary, 4.5)
			assertContrast(t, "selected rail icon/text", theme.Primary, palette.RailActive, 4.5)
			assertContrast(t, "selected pill text", theme.Primary, theme.Accent, 4.5)
			assertContrast(t, "primary text on card", theme.Primary, theme.Card, 4.5)
			assertContrast(t, "accent text on rail active", theme.AccentForeground, palette.RailActive, 4.5)
			assertContrast(t, "accent text on accent", theme.AccentForeground, theme.Accent, 4.5)

			for _, surface := range []namedColor{
				{"card", theme.Card},
				{"background", theme.Background},
				{"sidebar", palette.Sidebar},
				{"rail", palette.Rail},
				{"tableHeader", palette.TableHeader},
			} {
				assertContrast(t, "foreground on "+surface.name, theme.Foreground, surface.color, 4.5)
				assertContrast(t, "muted on "+surface.name, theme.MutedForeground, surface.color, 4.5)
			}

			if theme.Success == palette.Warning || theme.Success == theme.Danger || palette.Warning == theme.Danger {
				t.Fatal("semantic status colors must be distinct")
			}
			if theme.Success == theme.Primary || palette.Warning == theme.Primary || theme.Danger == theme.Primary {
				t.Fatal("semantic status colors must stay distinct from primary blue")
			}
			for _, code := range []int{200, 302, 404, 500} {
				status := palette.StatusColor(&theme, code)
				badgeBg := status.WithAlpha(0.12).Over(theme.Card)
				assertContrast(t, "status badge", status, badgeBg, 4.5)
				assertContrast(t, "status on card", status, theme.Card, 4.5)
			}

			for _, method := range model.Methods {
				color := palette.MethodColor(method)
				for _, surface := range []namedColor{
					{"card", theme.Card},
					{"sidebar", palette.Sidebar},
					{"selected request", theme.Accent},
				} {
					badgeBackground := color.WithAlpha(0.10).Over(surface.color)
					assertContrast(t, string(method)+" badge on "+surface.name, color, badgeBackground, 4.5)
				}
			}

			editorBg := components.TextEntryRecipe(theme, gfx.State{}).Fill
			codeTokens := []namedColor{
				{"codeKey", palette.CodeKey},
				{"codeString", palette.CodeString},
				{"codeNumber", palette.CodeNumber},
				{"codeLiteral", palette.CodeLiteral},
				{"codePunct", palette.CodePunct},
			}
			assertDistinctColors(t, "JSON syntax tokens", codeTokens, 0.05)
			for _, token := range codeTokens {
				assertContrast(t, token.name+" on editor", token.color, editorBg, 4.5)
			}
			assertContrast(t, "codeLineNo on editor", palette.CodeLineNo, editorBg, 3.0)
		})
	}
}

type namedColor struct {
	name  string
	color ui.Color
}

func assertDistinctColors(t *testing.T, group string, colors []namedColor, minDistance float64) {
	t.Helper()
	for i := range colors {
		for j := i + 1; j < len(colors); j++ {
			dr := float64(colors[i].color.Red - colors[j].color.Red)
			dg := float64(colors[i].color.Green - colors[j].color.Green)
			db := float64(colors[i].color.Blue - colors[j].color.Blue)
			if dist := math.Sqrt(dr*dr + dg*dg + db*db); dist < minDistance {
				t.Errorf("%s: %s and %s are too close (distance=%.3f, want >= %.3f)", group, colors[i].name, colors[j].name, dist, minDistance)
			}
		}
	}
}

func isBlueAccent(c ui.Color) bool {
	return c.Blue > c.Red+0.15 && c.Blue > c.Green+0.08
}

func assertContrast(t *testing.T, label string, fg, bg ui.Color, minimum float64) {
	t.Helper()
	ratio := contrastRatio(fg, bg)
	if ratio < minimum {
		t.Errorf("%s contrast = %.2f:1, want >= %.1f:1 (fg=%+v bg=%+v)", label, ratio, minimum, fg, bg)
	}
}

func contrastRatio(fg, bg ui.Color) float64 {
	effectiveFg := fg.Over(bg)
	l1 := relativeLuminance(effectiveFg)
	l2 := relativeLuminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(c ui.Color) float64 {
	linear := func(channel float32) float64 {
		v := float64(channel)
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.Red) + 0.7152*linear(c.Green) + 0.0722*linear(c.Blue)
}
