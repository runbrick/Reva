package gui

import (
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
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

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
	h.click("中文")
	if h.App.S.Lang != i18n.Zh || !h.shows("数据目录") {
		t.Fatal("switching back did not show Chinese settings")
	}
	if got := h.Svc.Settings.Get().Language; got != "zh" {
		t.Fatalf("saved language = %q, want zh", got)
	}
}

func TestSettingsThemeAppliesAndSavesImmediately(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	h := newHarness(t)
	h.click(h.App.S.Settings.Title)
	h.click(h.App.S.Settings.Dark)
	if h.App.theme.Background != ui.DarkTheme().Background {
		t.Fatal("dark theme did not apply immediately")
	}
	if got := h.Svc.Settings.Get().Theme; got != "dark" {
		t.Fatalf("saved theme = %q, want dark", got)
	}
	h.click(h.App.S.Settings.Light)
	if h.App.theme.Background != ui.LightTheme().Background || h.Svc.Settings.Get().Theme != "light" {
		t.Fatal("light theme did not apply and save immediately")
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
	h.click("Close")
	if h.shows("Data directory") {
		t.Fatal("settings dialog did not close")
	}
	h.click("Settings")
	settingsAssertSelected(t, h, "Dark")
	settingsAssertSelected(t, h, "English")
	settingsAssertLimit(t, h, "500")

	// Read the service again when opening, including changes from another feature.
	h.click("Close")
	s := h.Svc.Settings.Get()
	s.Theme, s.HistoryLimit = "light", 1000
	if err := h.Svc.Settings.Save(s); err != nil {
		t.Fatal(err)
	}
	h.click("Settings")
	settingsAssertSelected(t, h, "Light")
	settingsAssertLimit(t, h, "1000")
}

func TestSettingsShortcutOpensDialog(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	h := newHarness(t)
	if h.shows(h.App.S.Settings.DataDirectory) {
		t.Fatal("settings dialog is open before the shortcut")
	}
	if handled, err := h.D.ShortcutChord(ui.Primary, ','); err != nil || !handled {
		t.Fatalf("settings shortcut: handled = %t, error = %v", handled, err)
	}
	if !h.shows(h.App.S.Settings.DataDirectory) {
		t.Fatal("settings shortcut did not open the dialog")
	}
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if h.shows(h.App.S.Settings.DataDirectory) {
		t.Fatal("Escape did not close settings")
	}
}

func TestSettingsSaveErrorShowsToastAndKeepsPreferences(t *testing.T) {
	t.Setenv(storage.EnvHome, t.TempDir())
	svc := fake.New()
	svc.Settings = &settingsSaveFailure{Settings: svc.Settings.(*fake.Settings)}
	h := newHarness(t, svc)
	h.click(h.App.S.Settings.Title)
	for _, label := range []string{h.App.S.Settings.Dark, "English"} {
		h.click(label)
		if !h.shows(h.App.S.Common.Error) || !h.shows("settings cannot be saved") {
			t.Fatal("save failure did not show an error toast")
		}
		if h.Svc.Settings.Get() != model.DefaultSettings() || h.App.S.Lang != i18n.Zh ||
			h.App.theme.Background != ui.LightTheme().Background {
			t.Fatal("failed save changed the active or persisted settings")
		}
	}
	settingsAssertSelected(t, h, h.App.S.Settings.Light)
	settingsAssertSelected(t, h, "中文")
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
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Settings.HistoryLimit && hit.Value == value {
			return
		}
	}
	t.Fatalf("history limit does not show %q", value)
}

func BenchmarkSettingsDialogPaint(b *testing.B) {
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
		page.Draw(canvas, ui.Rect{Width: 1280, Height: 800}, a.settingsDialog)
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
