package themes

import "testing"

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
	if light.Primary != light.Foreground || dark.Primary != dark.Foreground || light.Primary == dark.Primary {
		t.Fatal("primary action does not follow the neutral foreground")
	}
	if lightPalette.Get == lightPalette.Post || darkPalette.Get == darkPalette.Post {
		t.Fatal("HTTP methods lost their distinct colors")
	}
}
