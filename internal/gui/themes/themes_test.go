package themes

import "testing"

func TestSettingsCompatibility(t *testing.T) {
	for _, test := range []struct {
		stored, mode string
		style        Style
	}{
		{"", "light", Modern},
		{"light", "light", Modern},
		{"dark", "dark", Modern},
		{"system", "system", Modern},
		{"classic:light", "light", Classic},
		{"classic:dark", "dark", Classic},
		{"classic:system", "system", Classic},
		{"future:dark", "dark", Modern},
		{"classic:future", "light", Classic},
	} {
		style, mode := Decode(test.stored)
		if style != test.style || mode != test.mode {
			t.Errorf("Decode(%q) = %q, %q; want %q, %q", test.stored, style, mode, test.style, test.mode)
		}
		if savedStyle, savedMode := Decode(Encode(style, mode)); savedStyle != style || savedMode != mode {
			t.Errorf("style and mode did not round-trip for %q", test.stored)
		}
	}
	for _, mode := range []string{"light", "dark", "system"} {
		if value := Encode(Modern, mode); value != mode {
			t.Errorf("modern mode %q saved as %q", mode, value)
		}
	}
}
