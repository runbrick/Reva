package gui

import (
	"reflect"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestOnboardingWorkbenchEntryLifecycle(t *testing.T) {
	for _, language := range []i18n.Lang{i18n.Zh, i18n.En} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(string(language)+"/"+theme, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv(storage.EnvHome, dir)
				disk, err := store.Open(dir)
				authMust(t, err)
				settings := model.DefaultSettings()
				settings.Language, settings.Theme, settings.HistoryLimit = string(language), theme, 500
				authMust(t, disk.Settings().Save(settings))
				before := researchDisk(t, dir)
				text := i18n.For(language)
				s := text.Settings

				h, writes := onboardingWorkbenchHarness(t, dir)
				if h.App.Dark() != (theme == "dark") {
					t.Fatal("saved theme was not applied")
				}
				onboardingAssertClosed(t, h)
				entry := onboardingWorkbenchButton(t, h, s.GuideWorkbenchOpen)
				active, drafts := h.App.Active(), onboardingActionDrafts(h)
				if language == i18n.En {
					onboardingWorkbenchActivate(t, h, entry)
				} else {
					h.click(s.GuideWorkbenchOpen)
				}
				onboardingRegressionSnapshot(t, h, s)
				if h.App.Main != PageWorkbench || h.App.Section != SectionAPIs || h.App.Active() != active ||
					!reflect.DeepEqual(onboardingActionDrafts(h), drafts) {
					t.Fatal("opening the guide changed navigation or the original draft")
				}
				h.click(s.GuideSkip)
				onboardingAssertClosed(t, h)
				onboardingWorkbenchHidden(t, h, s.GuideWorkbenchOpen)
				assertTabsEditorURL(t, h, "")
				h.click(text.Tabs.New)
				onboardingWorkbenchHidden(t, h, s.GuideWorkbenchOpen)
				h.click(s.Title)
				h.click(s.OpenGuide)
				onboardingRegressionSnapshot(t, h, s)
				authMust(t, h.D.Key(gfx.KeyEscape, 0))
				onboardingAssertClosed(t, h)
				h.click(text.Shell.APIs)
				onboardingWorkbenchHidden(t, h, s.GuideWorkbenchOpen)
				onboardingWorkbenchUnwritten(t, h, writes, dir, before)
				h.D.Close()

				h, writes = onboardingWorkbenchHarness(t, dir)
				onboardingAssertClosed(t, h)
				onboardingWorkbenchButton(t, h, s.GuideWorkbenchOpen)
				h.click(s.GuideWorkbenchOpen)
				h.click(s.GuideDone)
				settings.OnboardingCompleted = true
				onboardingSavedDisk(t, h, dir, settings)
				onboardingAssertClosed(t, h)
				onboardingWorkbenchHidden(t, h, s.GuideWorkbenchOpen)
				if writes.saves != 1 {
					t.Fatalf("Done saved settings %d times, want 1", writes.saves)
				}
				completed := researchDisk(t, dir)
				before["settings.json"] = completed["settings.json"]
				if !reflect.DeepEqual(completed, before) {
					t.Fatal("Done changed disk data outside settings.json")
				}
				h.D.Close()

				h, writes = onboardingWorkbenchHarness(t, dir)
				onboardingAssertClosed(t, h)
				onboardingWorkbenchHidden(t, h, s.GuideWorkbenchOpen)
				h.click(s.Title)
				h.click(s.ReplayGuide)
				onboardingRegressionSnapshot(t, h, s)
				h.click(s.GuideSkip)
				onboardingAssertClosed(t, h)
				onboardingSavedDisk(t, h, dir, settings)
				onboardingWorkbenchUnwritten(t, h, writes, dir, completed)
			})
		}
	}
}

func TestOnboardingWorkbenchEntryYieldsToCurrentDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	h, writes := onboardingWorkbenchHarness(t, dir)
	before := researchDisk(t, dir)
	entry := h.App.S.Settings.GuideWorkbenchOpen
	onboardingAssertClosed(t, h)
	onboardingWorkbenchButton(t, h, entry)
	const editedURL = "https://example.invalid/edited"
	requestType(h, h.App.S.Request.URL, editedURL, false)
	assertTabsEditorURL(t, h, editedURL)
	onboardingWorkbenchHidden(t, h, entry)
	_, err := h.D.ShortcutChord(ui.Primary, 'a')
	authMust(t, err)
	authMust(t, h.D.Key(gfx.KeyBackspace, 0))
	assertTabsEditorURL(t, h, "")
	onboardingWorkbenchHidden(t, h, entry)

	h.click(h.App.S.Tabs.New)
	onboardingWorkbenchButton(t, h, entry)
	populated := model.NewRequest("populated-draft")
	populated.URL = "https://example.invalid/populated"
	h.App.OpenRequest(populated, "")
	studioResize(t, h, 960, 600)
	assertTabsEditorURL(t, h, populated.URL)
	onboardingWorkbenchHidden(t, h, entry)
	onboardingAssertClosed(t, h)
	onboardingWorkbenchUnwritten(t, h, writes, dir, before)
}

func TestOnboardingWorkbenchEntryOpensOfflineSample(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	h, writes := onboardingWorkbenchHarness(t, dir)
	before := researchDisk(t, dir)
	if _, exists := before["settings.json"]; exists {
		t.Fatal("opening the workbench for the first time saved settings")
	}
	s, request := h.App.S.Settings, h.App.S.Request
	drafts := onboardingActionDrafts(h)
	h.click(s.GuideWorkbenchOpen)
	h.click(s.GuidePractice)
	if h.shows(s.GuideTitle) || !h.shows(request.PreviewTitle) || !h.shows(onboardingSampleURL) {
		t.Fatal("workbench guide did not hand off to the offline sample preview")
	}
	h.click(request.PreviewClose)
	assertTabsEditorURL(t, h, onboardingSampleURL)
	if got := onboardingActionDrafts(h); len(got) != len(drafts)+1 || !reflect.DeepEqual(got[:len(drafts)], drafts) {
		t.Fatal("the sample did not create an independent draft")
	}
	onboardingWorkbenchHidden(t, h, s.GuideWorkbenchOpen)
	onboardingWorkbenchUnwritten(t, h, writes, dir, before)
}

func onboardingWorkbenchHarness(t *testing.T, dir string) (*harness, *onboardingSettingsProbe) {
	t.Helper()
	h := onboardingDiskHarness(t, dir)
	writes := &onboardingSettingsProbe{SettingsService: h.Svc.Settings}
	h.App.Svc.Settings = writes
	authMust(t, h.D.Resize(960, 600))
	studioResize(t, h, 960, 600)
	return h, writes
}

func onboardingWorkbenchButton(t *testing.T, h *harness, label string) gfx.Hit {
	t.Helper()
	snapshot := h.D.Snapshot()
	if snapshot.Width != 960 || snapshot.Height != 600 || !h.shows(label) {
		t.Fatalf("button %q not painted at 960x600", label)
	}
	for _, hit := range snapshot.Hits {
		if hit.Label != label {
			continue
		}
		bounds := hit.Bounds
		if hit.Role != gfx.RoleButton || hit.Disabled || !hit.Focusable || hit.SkipTab || hit.Layer != 0 ||
			bounds.Width <= 0 || bounds.Height <= 0 || bounds.X < 0 || bounds.Y < 0 ||
			bounds.X+bounds.Width > float32(snapshot.Width) || bounds.Y+bounds.Height > float32(snapshot.Height) {
			t.Fatalf("workbench entry is not an enabled and reachable button: %+v", hit)
		}
		if target, ok := h.D.HitTest(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); !ok || target != hit.ID {
			t.Fatalf("pointer at button center resolves to %v, want %v", target, hit.ID)
		}
		return hit
	}
	t.Fatalf("no accessible button named %q; have %q", label, h.hitLabels())
	return gfx.Hit{}
}

func onboardingWorkbenchActivate(t *testing.T, h *harness, hit gfx.Hit) {
	t.Helper()
	for range len(h.D.Snapshot().Hits) + 1 {
		if h.D.Snapshot().Focused == hit.ID {
			authMust(t, h.D.Key(gfx.KeyEnter, 0))
			return
		}
		authMust(t, h.D.Key(gfx.KeyTab, 0))
	}
	t.Fatalf("Tab could not focus the workbench entry %q", hit.Label)
}

func onboardingWorkbenchHidden(t *testing.T, h *harness, label string) {
	t.Helper()
	if h.shows(label) {
		t.Fatalf("workbench entry %q was painted", label)
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label {
			t.Fatalf("hidden workbench entry %q was still in the hit map", label)
		}
	}
}

func onboardingWorkbenchUnwritten(t *testing.T, h *harness, writes *onboardingSettingsProbe, dir string, before map[string]string) {
	t.Helper()
	h.pump()
	if writes.saves != 0 || !reflect.DeepEqual(researchDisk(t, dir), before) {
		t.Fatalf("workbench flow saved settings %d times or changed other disk data", writes.saves)
	}
}
