package gui

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestTitlebarKeepsPlatformInsetsClear(t *testing.T) {
	for _, platform := range []string{"darwin", "windows", "linux"} {
		for _, lang := range []string{"zh", "en"} {
			for _, width := range []int{960, 1280, 1440} {
				t.Run(fmt.Sprintf("%s/%s/%d", platform, lang, width), func(t *testing.T) {
					h := newHarness(t)
					h.App.Platform = platform
					settings := h.Svc.Settings.Get()
					settings.Language = lang
					h.App.ApplySettings(settings)
					if err := h.D.Resize(width, 800); err != nil {
						t.Fatal(err)
					}
					// Viva v0.6.0 app/window_controls_darwin.go reserves 70 px.
					// app/window_controls_viva.go reserves 3*46 = 138 px.
					leading, trailing := float32(0), float32(138)
					if platform == "darwin" {
						leading, trailing = 70, 0
					}
					if platform == runtime.GOOS {
						leading, trailing = app.TitlebarInsets()
					}
					for _, hit := range h.D.Snapshot().Hits {
						r := hit.Bounds
						if r.Y >= TitlebarHeight || r.Width <= 0 || r.Height <= 0 {
							continue
						}
						if r.X < leading || r.X+r.Width > float32(width)-trailing {
							t.Errorf("%q at %+v overlaps window controls (insets %g, %g)", hit.Label, r, leading, trailing)
						}
					}
					// An overlap check must also ensure controls remain available.
					for _, p := range Pages {
						titlebarClick(t, h, h.App.pageLabel(p), false)
						if h.App.Main != p {
							t.Fatalf("navigation %v did not switch page", p)
						}
					}
					titlebarClick(t, h, h.App.S.Titlebar.UserMenu, false)
					if !h.App.titlebar.menuOpen {
						t.Fatal("avatar menu is unreachable")
					}
				})
			}
		}
	}
}

func TestTitlebarNavigationShowsPages(t *testing.T) {
	h := newHarness(t)
	for _, p := range []MainPage{PageHistory, PageSettings, PageWorkbench} {
		titlebarClick(t, h, h.App.pageLabel(p), false)
		if h.App.Main != p {
			t.Fatalf("Main = %v, want %v", h.App.Main, p)
		}
		text := h.App.S.Request.Send
		switch p {
		case PageHistory:
			text = h.App.S.History.Empty
		case PageSettings:
			text = h.App.S.Settings.Title
		}
		found := false
		for _, label := range h.D.Snapshot().Labels {
			if label.Bounds.Y >= TitlebarHeight && label.Value == text {
				found = true
			}
		}
		if !found {
			t.Errorf("page %v did not show %q below titlebar", p, text)
		}
	}
}

func TestTitlebarMenuSavesThemeAndLanguage(t *testing.T) {
	h := newHarness(t)
	initial := h.Svc.Settings.Get()
	for _, choice := range []struct{ value, label string }{
		{"dark", h.App.S.Titlebar.Dark}, {"light", h.App.S.Titlebar.Light}, {"system", h.App.S.Titlebar.System},
	} {
		titlebarClick(t, h, h.App.S.Titlebar.UserMenu, false)
		titlebarClick(t, h, choice.label, true)
		if got := h.Svc.Settings.Get().Theme; got != choice.value {
			t.Fatalf("saved theme = %q, want %q", got, choice.value)
		}
		if choice.value != "system" && h.App.Dark() != (choice.value == "dark") {
			t.Fatalf("Dark() = %v for theme %q", h.App.Dark(), choice.value)
		}
		if h.App.titlebar.menuOpen {
			t.Fatal("choosing theme left menu open")
		}
	}
	titlebarClick(t, h, h.App.S.Titlebar.UserMenu, false)
	titlebarClick(t, h, h.App.S.Titlebar.English, true)
	if h.App.S.Lang != i18n.En || h.Svc.Settings.Get().Language != "en" {
		t.Fatal("English was not saved and applied")
	}
	titlebarClick(t, h, h.App.S.Titlebar.UserMenu, false)
	titlebarClick(t, h, h.App.S.Titlebar.Chinese, true)
	if h.App.S.Lang != i18n.Zh || h.Svc.Settings.Get().Language != "zh" {
		t.Fatal("Chinese was not saved and applied")
	}
	want := initial
	want.Theme, want.Language = "system", "zh"
	if got := h.Svc.Settings.Get(); got != want {
		t.Fatalf("menu changed unrelated settings: got %+v, want %+v", got, want)
	}
}

func TestTitlebarMenuSettingsAndAbout(t *testing.T) {
	h := newHarness(t)
	titlebarClick(t, h, h.App.S.Titlebar.UserMenu, false)
	titlebarClick(t, h, h.App.S.Titlebar.Settings, true)
	if h.App.Main != PageSettings {
		t.Fatal("menu Settings did not select the Settings page")
	}
	titlebarClick(t, h, h.App.S.Titlebar.UserMenu, false)
	titlebarClick(t, h, h.App.S.Titlebar.About, true)
	if !h.shows(h.App.S.Titlebar.About) || !h.shows(h.App.S.Common.AppName) {
		t.Fatal("About did not show app name toast")
	}
}

type titlebarFailingSettings struct{ value model.Settings }

func (s *titlebarFailingSettings) Get() model.Settings { return s.value }
func (s *titlebarFailingSettings) Save(model.Settings) error {
	return errors.New("settings cannot be saved")
}

func TestTitlebarMenuSaveFailureKeepsTheme(t *testing.T) {
	svc := fake.New()
	settings := svc.Settings.Get()
	settings.Theme = "light"
	svc.Settings = &titlebarFailingSettings{value: settings}
	h := newHarness(t, svc)
	titlebarClick(t, h, h.App.S.Titlebar.UserMenu, false)
	titlebarClick(t, h, h.App.S.Titlebar.Dark, true)
	if h.App.Dark() || h.Svc.Settings.Get().Theme != "light" {
		t.Fatal("unsaved theme was applied")
	}
	if !h.shows("settings cannot be saved") {
		t.Fatal("save error was not shown")
	}
}

func TestTitlebarWindowControlsFollowThemeAndLanguage(t *testing.T) {
	h := newHarness(t)
	config := h.App.windowConfig(app.Config{})
	if config.WindowMode != app.WindowCustomTitlebar || config.TitlebarHeight != 48 || config.WindowControls == nil {
		t.Fatal("custom titlebar configuration is incomplete")
	}
	settings := h.Svc.Settings.Get()
	settings.Theme, settings.Language = "light", "zh"
	h.App.ApplySettings(settings)
	light := config.WindowControls()
	if light.Minimize != "最小化" || light.Maximize != "最大化" || light.Restore != "还原" || light.CloseName != "关闭" {
		t.Fatalf("window controls are not Chinese: %+v", light)
	}
	settings.Theme, settings.Language = "dark", "en"
	h.App.ApplySettings(settings)
	dark := config.WindowControls()
	if light.Foreground == dark.Foreground {
		t.Fatal("window control foreground did not follow theme change")
	}
	if dark.Foreground != h.App.theme.MutedForeground || dark.Hover != h.App.theme.Hover ||
		dark.Pressed != h.App.theme.Muted || dark.Close != h.App.theme.Danger || dark.CloseForeground != h.App.theme.DangerForeground {
		t.Fatal("window control style does not use current theme")
	}
	if dark.Minimize != "Minimize" || dark.Maximize != "Maximize" || dark.Restore != "Restore" || dark.CloseName != "Close" {
		t.Fatalf("window controls did not follow English switch: %+v", dark)
	}
}

// titlebarClick distinguishes the strip's Settings hit from the rail, page and
// popup's identically named Settings controls.
func titlebarClick(t *testing.T, h *harness, label string, menu bool) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		r := hit.Bounds
		if hit.Label == label && (r.Y >= TitlebarHeight) == menu && (!menu || hit.Layer > 0) && r.Width > 0 && r.Height > 0 {
			if err := h.D.Click(r.X+r.Width/2, r.Y+r.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no titlebar hit %q (menu=%v); have %q", label, menu, h.hitLabels())
}

func BenchmarkTitlebarPaint(b *testing.B) {
	a := New(fake.New())
	page := ui.NewPage(ui.PageConfig{View: a.titlebarStrip, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(d.Close)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := d.Resize(1280+i%2, 800); err != nil {
			b.Fatal(err)
		}
	}
}
