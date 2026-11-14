package gui

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

// Exercise the public Settings controls against the production store rather
// than inspecting the guide's private UI state or replacing its save method.
func TestOnboardingDiskLifecycle(t *testing.T) {
	for _, language := range []i18n.Lang{i18n.Zh, i18n.En} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(string(language)+"/"+theme, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv(storage.EnvHome, dir)
				h := onboardingDiskHarness(t, dir)
				original := model.DefaultSettings()
				original.Language, original.Theme = string(language), theme
				original.ActiveEnvID, original.HistoryLimit = "staging", 500
				if err := h.Svc.Settings.Save(original); err != nil {
					t.Fatal(err)
				}
				h.D.Close()
				h = onboardingDiskHarness(t, dir)
				s := i18n.For(language).Settings
				before := onboardingSettingsBytes(t, dir)
				if h.shows(s.GuideTitle) {
					t.Fatal("pending guide opened automatically")
				}
				h.click(s.Title)
				if !h.shows(s.OpenGuide) {
					t.Fatal("pending guide is unavailable from Settings")
				}
				if err := h.D.Resize(960, 600); err != nil {
					t.Fatal(err)
				}
				h.click(s.OpenGuide)
				onboardingRegressionSnapshot(t, h, s)
				h.click(s.GuideSkip)
				onboardingPendingDisk(t, h, dir, original, before)
				h.D.Close()

				// Recreate both App and Store after Skip, then dismiss with Escape.
				h = onboardingDiskHarness(t, dir)
				h.click(s.Title)
				h.click(s.OpenGuide)
				if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
				onboardingPendingDisk(t, h, dir, original, before)
				h.click(s.OpenGuide)
				// Another settings consumer saves while the guide is open. Done
				// must read these latest preferences, including non-UI values.
				latest := original
				latest.ActiveEnvID, latest.HistoryLimit = "production", 1000
				latest.RequestTimeoutSec = 120
				latest.NoFollowRedirects, latest.InsecureTLS = true, true
				if err := h.Svc.Settings.Save(latest); err != nil {
					t.Fatal(err)
				}
				h.click(s.GuideDone)
				latest.OnboardingCompleted = true
				onboardingSavedDisk(t, h, dir, latest)
				if h.shows(s.GuideTitle) || !h.shows(s.ReplayGuide) {
					t.Fatal("successful Done did not close the guide and offer replay")
				}
				h.D.Close()

				h = onboardingDiskHarness(t, dir)
				onboardingSavedDisk(t, h, dir, latest)
				if h.shows(s.GuideTitle) {
					t.Fatal("completed guide opened on restart")
				}
				h.click(s.Title)
				h.click(s.ReplayGuide)
				onboardingRegressionSnapshot(t, h, s)
				completed := onboardingSettingsBytes(t, dir)
				h.click(s.GuideSkip)
				onboardingSavedDisk(t, h, dir, latest)
				if h.shows(s.GuideTitle) || !bytes.Equal(completed, onboardingSettingsBytes(t, dir)) {
					t.Fatal("skipping replay changed saved settings or left guide open")
				}
			})
		}
	}
}

func TestOnboardingDiskWriteFailureCanRetry(t *testing.T) {
	for _, language := range []i18n.Lang{i18n.Zh, i18n.En} {
		t.Run(string(language), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv(storage.EnvHome, dir)
			h := onboardingDiskHarness(t, dir)
			original := model.DefaultSettings()
			original.Language = string(language)
			if err := h.Svc.Settings.Save(original); err != nil {
				t.Fatal(err)
			}
			h.D.Close()
			h = onboardingDiskHarness(t, dir)
			s := i18n.For(language).Settings
			h.click(s.Title)
			h.click(s.OpenGuide)
			if err := h.D.Resize(960, 600); err != nil {
				t.Fatal(err)
			}
			// A directory at the destination causes a real atomic-write failure
			// on both Windows and Unix, including when run with elevated rights.
			path := filepath.Join(dir, "settings.json")
			backup := filepath.Join(dir, "settings.saved")
			if err := os.Rename(path, backup); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				h.click(s.GuideDone)
				if !onboardingRegressionShows(h, s.GuideSaveFailed) || h.shows(s.ReplayGuide) {
					t.Fatal("failed write lacks localized retry feedback or falsely offers replay")
				}
				onboardingRegressionSnapshot(t, h, s)
				if h.Svc.Settings.Get() != original {
					t.Fatal("failed Done published completion in memory")
				}
				var saved model.Settings
				if err := storage.ReadJSON(backup, &saved); err != nil || saved != original {
					t.Fatalf("failed Done changed last durable preferences: %+v, %v", saved, err)
				}
				if onboardingRegressionShows(h, dir) {
					t.Fatal("guide exposed the raw filesystem error")
				}
			}
			if err := os.Remove(path); err != nil { // empty test-created directory
				t.Fatal(err)
			}
			if err := os.Rename(backup, path); err != nil {
				t.Fatal(err)
			}
			latest := original
			latest.ActiveEnvID, latest.RequestTimeoutSec = "changed-during-retry", 300
			latest.HistoryLimit, latest.NoFollowRedirects = 500, true
			if err := h.Svc.Settings.Save(latest); err != nil {
				t.Fatal(err)
			}
			h.click(s.GuideDone)
			latest.OnboardingCompleted = true
			onboardingSavedDisk(t, h, dir, latest)
			h.D.Close()
			h = onboardingDiskHarness(t, dir)
			onboardingSavedDisk(t, h, dir, latest)
			h.click(s.Title)
			h.click(s.ReplayGuide)
			if onboardingRegressionShows(h, s.GuideSaveFailed) {
				t.Fatal("reloaded replay retained a stale save error")
			}
			onboardingRegressionSnapshot(t, h, s)
		})
	}
}

func onboardingDiskHarness(t *testing.T, dir string) *harness {
	t.Helper()
	disk, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if errs := disk.LoadErrors(); len(errs) != 0 {
		t.Fatalf("store reload errors: %v", errs)
	}
	requests := &fake.Requests{Fn: func(context.Context, model.Request) (model.Response, error) {
		t.Error("onboarding sent a request")
		return model.Response{}, nil
	}}
	return newHarness(t, service.Services{
		Requests: requests, Settings: disk.Settings(), History: disk.History(),
		Collections: disk.Collections(), Environments: disk.Environments(),
	})
}

func onboardingSettingsBytes(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func onboardingPendingDisk(t *testing.T, h *harness, dir string, want model.Settings, before []byte) {
	t.Helper()
	onboardingSavedDisk(t, h, dir, want)
	if h.shows(h.App.S.Settings.GuideTitle) || !h.shows(h.App.S.Settings.OpenGuide) ||
		!bytes.Equal(before, onboardingSettingsBytes(t, dir)) {
		t.Fatal("dismissal did not leave the guide closed and durable settings untouched")
	}
}

func onboardingSavedDisk(t *testing.T, h *harness, dir string, want model.Settings) {
	t.Helper()
	var saved model.Settings
	if err := storage.ReadJSON(filepath.Join(dir, "settings.json"), &saved); err != nil {
		t.Fatal(err)
	}
	if saved != want || h.Svc.Settings.Get() != want {
		t.Fatalf("settings mismatch: disk=%+v memory=%+v want=%+v", saved, h.Svc.Settings.Get(), want)
	}
}

func onboardingRegressionShows(h *harness, text string) bool {
	var painted strings.Builder
	for _, label := range h.D.Snapshot().Labels {
		painted.WriteString(label.Value)
	}
	compact := func(s string) string { return strings.Join(strings.Fields(s), "") }
	return strings.Contains(compact(painted.String()), compact(text))
}

func onboardingRegressionSnapshot(t *testing.T, h *harness, s i18n.Settings) {
	t.Helper()
	for _, text := range []string{s.GuideTitle, s.GuideIntro, s.GuideRequestTitle, s.GuideRequestDescription,
		s.GuideVariablesTitle, s.GuideVariablesDescription, s.GuideDataTitle, s.GuideDataDescription, s.GuideDone, s.GuideSkip} {
		if !onboardingRegressionShows(h, text) {
			t.Errorf("painted guide missing localized copy %q", text)
		}
	}
	snapshot := h.D.Snapshot()
	var layer uint16
	for _, label := range snapshot.Labels {
		if label.Value == s.GuideTitle {
			layer = label.Layer
		}
	}
	if layer == 0 {
		t.Fatal("guide title is not painted on a modal layer")
	}
	// Check only the modal: shell controls behind it are not guide content.
	modal := app.Snapshot{Width: snapshot.Width, Height: snapshot.Height}
	for i, text := range snapshot.Texts {
		if text.Layer >= layer {
			modal.Texts = append(modal.Texts, text)
			modal.TextWidths = append(modal.TextWidths, snapshot.TextWidths[i])
		}
	}
	for _, hit := range snapshot.Hits {
		if hit.Layer >= layer {
			modal.Hits = append(modal.Hits, hit)
		}
	}
	for _, problem := range headless.CheckLayout(modal) {
		t.Errorf("guide snapshot layout: %+v", problem)
	}
}
