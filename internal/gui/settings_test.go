package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/runbrick/reqo/internal/update"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestSettingsCheckUpdates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		release update.Release
		err     error
		want    string
	}{
		{"new release", update.Release{Version: "0.3.0", URL: "https://github.com/runbrick/Reva/releases/tag/v0.3.0"}, nil, "New version available"},
		{"current release", update.Release{Version: settingsVersion, URL: "https://github.com/runbrick/Reva/releases/tag/v0.2.0"}, nil, "You're up to date"},
		{"no release", update.Release{}, update.ErrNoRelease, "No published release yet"},
		{"offline", update.Release{}, errors.New("connection failed"), "connection failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(storage.EnvHome, t.TempDir())
			h := newHarness(t)
			h.App.settings.check = func(context.Context) (update.Release, error) { return tc.release, tc.err }
			h.click(h.App.S.Settings.Title)
			h.click("English")
			if err := h.D.Scroll(gfx.ScrollEvent{X: 800, Y: 600, DeltaY: 1000, Precise: true}); err != nil {
				t.Fatal(err)
			}
			h.click(h.App.S.Settings.CheckUpdates)
			h.waitFor(tc.want, func() bool { return h.shows(tc.want) })
			if tc.name == "new release" {
				h.click(h.App.S.Settings.OpenRelease)
				if opened, ok := h.D.LastOpenedURL(); !ok || opened != tc.release.URL {
					t.Fatalf("opened URL %q (ok=%t), want %q", opened, ok, tc.release.URL)
				}
			}
		})
	}
}

func TestSettingsLanguageAppliesAndSavesImmediately(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	h := newHarness(t)
	h.click(h.App.S.Settings.Title)
	h.click("English")
	if h.App.S.Lang != i18n.En || !h.shows("Data directory") {
		t.Fatal("switching language did not immediately show English settings")
	}
	if got := h.Svc.Settings.Get().Language; got != "en" {
		t.Fatalf("saved language = %q, want en", got)
	}
	h.App.ApplySettings(h.Svc.Settings.Get())
	settingsRepaint(t, h)
	settingsAssertSelected(t, h, "English")
	h.click("中文")
	if h.App.S.Lang != i18n.Zh || !h.shows("数据目录") {
		t.Fatal("switching back did not show Chinese settings")
	}
	if got := h.Svc.Settings.Get().Language; got != "zh" {
		t.Fatalf("saved language = %q, want zh", got)
	}
	h.App.ApplySettings(h.Svc.Settings.Get())
	settingsRepaint(t, h)
	settingsAssertSelected(t, h, "中文")
}

func TestSettingsThemeAppliesAndSavesImmediately(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	h := newHarness(t)
	h.click(h.App.S.Settings.Title)
	h.click(h.App.S.Settings.Dark)
	if !h.App.Dark() {
		t.Fatal("dark theme did not apply immediately")
	}
	if got := h.Svc.Settings.Get().Theme; got != "dark" {
		t.Fatalf("saved theme = %q, want dark", got)
	}
	h.App.ApplySettings(h.Svc.Settings.Get())
	settingsRepaint(t, h)
	settingsAssertSelected(t, h, h.App.S.Settings.Dark)
	h.click(h.App.S.Settings.Light)
	if h.App.Dark() || h.Svc.Settings.Get().Theme != "light" {
		t.Fatal("light theme did not apply and save immediately")
	}
	h.App.ApplySettings(h.Svc.Settings.Get())
	settingsRepaint(t, h)
	settingsAssertSelected(t, h, h.App.S.Settings.Light)
}

func TestSettingsSystemThemeFollowsAppearance(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	h := newHarness(t)
	h.D.SetSystemAppearance(app.AppearanceLight)
	h.click(h.App.S.Settings.Title)
	h.click(h.App.S.Settings.System)
	if h.App.Dark() || h.Svc.Settings.Get().Theme != "system" {
		t.Fatal("follow system did not immediately apply and save")
	}
	h.App.ApplySettings(h.Svc.Settings.Get())
	for _, appearance := range []app.Appearance{app.AppearanceDark, app.AppearanceLight} {
		h.D.SetSystemAppearance(appearance)
		settingsRepaint(t, h)
		if h.App.Dark() != (appearance == app.AppearanceDark) || h.D.NativeAppearance() != appearance {
			t.Fatalf("appearance %v did not update application and native theme", appearance)
		}
		settingsAssertSelected(t, h, h.App.S.Settings.System)
		if h.Svc.Settings.Get().Theme != "system" {
			t.Fatal("system appearance change overwrote the saved preference")
		}
	}
	// An explicit choice stops following changes to the OS appearance.
	h.click(h.App.S.Settings.Light)
	h.D.SetSystemAppearance(app.AppearanceDark)
	settingsRepaint(t, h)
	if h.App.Dark() {
		t.Fatal("explicit light theme followed system appearance")
	}
}

func TestSettingsHistoryLimitsSaveAndPreserveOtherSettings(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	svc := fake.New()
	original := model.DefaultSettings()
	original.ActiveEnvID = "selected-environment"
	if err := svc.Settings.Save(original); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	h.click(h.App.S.Settings.Title)
	for _, limit := range []int{50, 100, 200, 500, 1000} {
		h.click(h.App.S.Settings.HistoryLimit)
		h.click(strconv.Itoa(limit))
		want := original
		want.HistoryLimit = limit
		if got := h.Svc.Settings.Get(); got != want {
			t.Fatalf("saved settings = %+v, want %+v", got, want)
		}
		h.App.ApplySettings(h.Svc.Settings.Get())
		settingsRepaint(t, h)
		settingsAssertLimit(t, h, strconv.Itoa(limit))
	}
}

func TestSettingsRequestDefaultsSaveImmediately(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	svc := fake.New()
	want := model.DefaultSettings()
	want.ActiveEnvID = "selected-environment"
	if err := svc.Settings.Save(want); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	h.click(h.App.S.Settings.Title)
	assertSaved := func() {
		t.Helper()
		if got := svc.Settings.Get(); got != want {
			t.Fatalf("saved settings = %+v, want %+v", got, want)
		}
		h.App.ApplySettings(svc.Settings.Get())
		settingsRepaint(t, h)
		settingsAssertValue(t, h, h.App.S.Settings.Timeout, strconv.Itoa(want.RequestTimeoutSec))
		settingsAssertChecked(t, h, h.App.S.Settings.FollowRedirects, !want.NoFollowRedirects)
		settingsAssertChecked(t, h, h.App.S.Settings.VerifyTLS, !want.InsecureTLS)
	}
	for _, seconds := range []int{10, 30, 60, 120, 300} {
		h.click(h.App.S.Settings.Timeout)
		h.click(strconv.Itoa(seconds))
		want.RequestTimeoutSec = seconds
		assertSaved()
	}
	for range 2 {
		h.click(h.App.S.Settings.FollowRedirects)
		want.NoFollowRedirects = !want.NoFollowRedirects
		assertSaved()
		h.click(h.App.S.Settings.VerifyTLS)
		want.InsecureTLS = !want.InsecureTLS
		assertSaved()
		if settingsShowsWrapped(h, h.App.S.Settings.TLSWarning) != want.InsecureTLS {
			t.Fatal("TLS warning does not match certificate verification preference")
		}
	}
}

func TestSettingsLegacyZeroTimeoutDisplaysEffectiveDefault(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	svc := fake.New()
	s := model.DefaultSettings()
	s.RequestTimeoutSec = 0
	if err := svc.Settings.Save(s); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	h.click(h.App.S.Settings.Title)
	settingsAssertValue(t, h, h.App.S.Settings.Timeout, "60")
	h.click(h.App.S.Settings.FollowRedirects)
	if svc.Settings.Get().RequestTimeoutSec != 0 {
		t.Fatal("changing another preference rewrote the inherited timeout")
	}
}

func TestSettingsReopeningShowsCurrentValues(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	h := newHarness(t)
	h.click(h.App.S.Settings.Title)
	h.click("English")
	h.click("Dark")
	h.click("History limit")
	h.click("500")
	h.click(h.App.S.Shell.APIs)
	if h.shows("Data directory") {
		t.Fatal("navigating to APIs did not leave settings")
	}
	h.click("Settings")
	settingsAssertSelected(t, h, "Dark")
	settingsAssertSelected(t, h, "English")
	settingsAssertLimit(t, h, "500")

	// Read the service again when opening, including changes from another feature.
	h.click(h.App.S.Shell.APIs)
	s := h.Svc.Settings.Get()
	s.Theme, s.HistoryLimit = "light", 1000
	if err := h.Svc.Settings.Save(s); err != nil {
		t.Fatal(err)
	}
	h.click("Settings")
	settingsAssertSelected(t, h, "Light")
	settingsAssertLimit(t, h, "1000")
}

func TestSettingsShortcutOpensPage(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	h := newHarness(t)
	if h.shows(h.App.S.Settings.DataDirectory) {
		t.Fatal("settings page is open before the shortcut")
	}
	if handled, err := h.D.ShortcutChord(ui.Primary, ','); err != nil || !handled {
		t.Fatalf("settings shortcut: handled = %t, error = %v", handled, err)
	}
	if h.App.Main != PageSettings || !h.shows(h.App.S.Settings.DataDirectory) {
		t.Fatal("settings shortcut did not open the page")
	}
	h.App.Main = PageHistory
	settingsRepaint(t, h)
	if handled, err := h.D.ShortcutChord(ui.Primary, ','); err != nil || !handled || h.App.Main != PageSettings {
		t.Fatalf("shortcut from History: handled = %t, error = %v, page = %v", handled, err, h.App.Main)
	}
}

func TestSettingsSaveErrorShowsToastAndKeepsPreferences(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	svc := fake.New()
	svc.Settings = &settingsSaveFailure{Settings: svc.Settings.(*fake.Settings)}
	h := newHarness(t, svc)
	h.click(h.App.S.Settings.Title)
	for _, label := range []string{h.App.S.Settings.Dark, "English", h.App.S.Settings.FollowRedirects, h.App.S.Settings.VerifyTLS} {
		h.click(label)
		if !h.shows(h.App.S.Common.Error) || !h.shows("settings cannot be saved") {
			t.Fatal("save failure did not show an error toast")
		}
		if h.Svc.Settings.Get() != model.DefaultSettings() || h.App.S.Lang != i18n.Zh ||
			h.App.Dark() {
			t.Fatal("failed save changed the active or persisted settings")
		}
	}
	settingsAssertSelected(t, h, h.App.S.Settings.Light)
	settingsAssertSelected(t, h, "中文")
	settingsAssertChecked(t, h, h.App.S.Settings.FollowRedirects, true)
	settingsAssertChecked(t, h, h.App.S.Settings.VerifyTLS, true)
}

func TestSettingsDataDirectoryAndOpenFolder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	h := newHarness(t)
	h.click(h.App.S.Settings.Title)
	if !settingsShowsWrapped(h, dir) {
		t.Fatalf("data directory is not displayed: %+v", h.D.Snapshot().Texts)
	}
	for range 2 {
		h.click(h.App.S.Settings.OpenFolder)
		h.waitFor("folder reveal", func() bool {
			path, ok := h.D.LastRevealedPath()
			return ok && path == dir && h.App.settings.reveal == nil
		})
	}
}

func TestSettingsDataDirectoryErrorIsShown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(storage.EnvHome, filepath.Join(path, "data"))
	h := newHarness(t)
	h.click(h.App.S.Settings.Title)
	if h.App.settings.dirError == nil || !settingsShowsWrapped(h, h.App.settings.dirError.Error()) {
		t.Fatal("data directory failure is not displayed")
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Settings.OpenFolder {
			t.Fatal("folder button is shown for an unavailable directory")
		}
	}
}

func TestSettingsOpenFolderErrorShowsToast(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "data")
	t.Setenv(storage.EnvHome, dir)
	h := newHarness(t)
	h.click(h.App.S.Settings.Title)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Settings.OpenFolder)
	h.waitFor("folder error", func() bool { return h.App.settings.reveal == nil })
	if !h.shows(h.App.S.Common.Error) || !settingsShowsWrapped(h, dir) {
		t.Fatal("folder reveal failure did not show an error toast")
	}
}

func TestSettingsPageBothLanguagesAndThemes(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	for _, lang := range []string{"zh", "en"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(lang+"/"+theme, func(t *testing.T) {
				svc := fake.New()
				s := model.DefaultSettings()
				s.Language, s.Theme = lang, theme
				if err := svc.Settings.Save(s); err != nil {
					t.Fatal(err)
				}
				h := newHarness(t, svc)
				// The rail enters the same page without the titlebar action.
				settingsClickRail(t, h, h.App.S.Settings.Title)
				for _, label := range []string{h.App.S.Settings.Appearance, h.App.S.Settings.Requests,
					h.App.S.Settings.History, h.App.S.Settings.Data} {
					if !h.shows(label) {
						t.Errorf("settings page does not show %q", label)
					}
				}
				settingsAssertLayout(t, h)
				if err := h.D.Scroll(gfx.ScrollEvent{X: 800, Y: 600, DeltaY: 1000, Precise: true}); err != nil {
					t.Fatal(err)
				}
				for _, label := range []string{h.App.S.Settings.About, h.App.S.Common.AppName, settingsVersion} {
					if !h.shows(label) {
						t.Errorf("scrolling settings does not show %q", label)
					}
				}
				settingsAssertLayout(t, h)
			})
		}
	}
}

func TestSettingsPageScrollsAtMinimumWindowSize(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	h := newHarness(t)
	h.click(h.App.S.Settings.Title)
	h.click("English")
	if err := h.D.Resize(960, 600); err != nil {
		t.Fatal(err)
	}
	settingsAssertLayout(t, h)
	h.click(h.App.S.Settings.VerifyTLS)
	if !settingsShowsWrapped(h, h.App.S.Settings.TLSWarning) {
		t.Fatal("TLS warning is not visible at the minimum window size")
	}
	if err := h.D.Scroll(gfx.ScrollEvent{X: 800, Y: 500, DeltaY: 1000, Precise: true}); err != nil {
		t.Fatal(err)
	}
	settingsAssertLayout(t, h)
	if !h.shows(settingsVersion) {
		t.Fatal("About is not reachable by scrolling at the minimum window size")
	}
	h.click(h.App.S.Settings.OpenFolder)
	h.waitFor("folder reveal in a small window", func() bool {
		_, ok := h.D.LastRevealedPath()
		return ok && h.App.settings.reveal == nil
	})
}

func settingsAssertLayout(t *testing.T, h *harness) {
	t.Helper()
	for _, problem := range headless.CheckLayout(h.D.Snapshot()) {
		// The settings feature owns the main page; the shell owns the rail and titlebar.
		if problem.Bounds.X >= RailWidth && problem.Bounds.Y >= TitlebarHeight {
			t.Errorf("settings layout: %+v", problem)
		}
	}
}

func settingsClickRail(t *testing.T, h *harness, label string) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label && hit.Bounds.X < RailWidth {
			b := hit.Bounds
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no rail item named %q", label)
}

type settingsSaveFailure struct{ *fake.Settings }

func settingsShowsWrapped(h *harness, text string) bool {
	var painted strings.Builder
	for _, label := range h.D.Snapshot().Labels {
		painted.WriteString(label.Value)
	}
	// Wrapping may consume whitespace at a line boundary.
	return strings.Contains(strings.Join(strings.Fields(painted.String()), ""),
		strings.Join(strings.Fields(text), ""))
}

func (s *settingsSaveFailure) Save(model.Settings) error {
	return errors.New("settings cannot be saved")
}

func settingsAssertSelected(t *testing.T, h *harness, label string) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label && (hit.Selected || hit.Checked) {
			return
		}
	}
	t.Fatalf("%q is not selected", label)
}

func settingsAssertLimit(t *testing.T, h *harness, value string) {
	t.Helper()
	settingsAssertValue(t, h, h.App.S.Settings.HistoryLimit, value)
}

func settingsAssertValue(t *testing.T, h *harness, label, value string) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label && hit.Value == value {
			return
		}
	}
	t.Fatalf("%q does not show %q", label, value)
}

func settingsAssertChecked(t *testing.T, h *harness, label string, checked bool) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label && hit.Checked == checked {
			return
		}
	}
	t.Fatalf("%q does not have checked = %t", label, checked)
}

func settingsRepaint(t *testing.T, h *harness) {
	t.Helper()
	snapshot := h.D.Snapshot()
	if err := h.D.Resize(snapshot.Width, snapshot.Height); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkSettingsPagePaint(b *testing.B) {
	b.Setenv(storage.EnvHome, b.TempDir())
	a := New(fake.New())
	a.openSettings()
	page := ui.NewPage(ui.PageConfig{Theme: &a.theme})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len(value)) * size / 2
	})
	paint := func() {
		canvas.Reset()
		canvas.BeginFrame()
		page.Draw(canvas, ui.Rect{Width: 1280, Height: 800}, a.settingsPage)
		canvas.EndFrame()
	}
	for range 10 {
		paint()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		paint()
	}
}
