package gui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/vivaui/viva/gfx"
)

func TestOnboardingGuideIsOptionalAndReplayable(t *testing.T) {
	for _, language := range []string{"zh", "en"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(language+"/"+theme, func(t *testing.T) {
				t.Setenv(storage.EnvHome, t.TempDir())
				svc := fake.New()
				original := model.DefaultSettings()
				original.Language, original.Theme = language, theme
				if err := svc.Settings.Save(original); err != nil {
					t.Fatal(err)
				}
				settings := &onboardingSettingsProbe{SettingsService: svc.Settings}
				svc.Settings = settings
				svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
					t.Error("the guide must not send requests")
					return model.Response{}, nil
				}
				h := newHarness(t, svc)
				request := h.App.Active().Request
				if h.App.settings.guideOpen || h.shows(h.App.S.Settings.GuideTitle) {
					t.Fatal("guide opened automatically on launch")
				}
				h.click(h.App.S.Settings.Title)
				if h.App.settings.guideOpen || !h.shows(h.App.S.Settings.GettingStarted) ||
					!h.shows(h.App.S.Settings.OpenGuide) || !h.shows(h.App.S.Settings.DataDirectory) {
					t.Fatal("Settings should offer the guide while retaining its initial controls")
				}
				if err := h.D.Resize(960, 600); err != nil {
					t.Fatal(err)
				}
				h.click(h.App.S.Settings.OpenGuide)
				onboardingAssertGuide(t, h)
				h.click(h.App.S.Settings.GuideSkip)
				onboardingAssertClosed(t, h)
				h.click(h.App.S.Settings.OpenGuide)
				if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
					t.Fatal(err)
				}
				onboardingAssertClosed(t, h)
				if settings.saves != 0 || svc.Settings.Get() != original {
					t.Fatal("opening, skipping or dismissing the guide changed settings")
				}
				h.click(h.App.S.Settings.OpenGuide)
				h.click(h.App.S.Settings.GuideDone)
				onboardingAssertClosed(t, h)
				want := original
				want.OnboardingCompleted = true
				if settings.saves != 1 || svc.Settings.Get() != want || !h.shows(h.App.S.Settings.ReplayGuide) {
					t.Fatal("Done did not persist completion and offer replay")
				}
				if !reflect.DeepEqual(h.App.Active().Request, request) || len(svc.History.List()) != 0 {
					t.Fatal("guide changed the request or created history")
				}
				// Completion survives a new app, and replay never needs a reset.
				h = newHarness(t, svc)
				request = h.App.Active().Request
				if h.App.settings.guideOpen {
					t.Fatal("completed guide opened on restart")
				}
				h.click(h.App.S.Settings.Title)
				h.click(h.App.S.Settings.ReplayGuide)
				onboardingAssertGuide(t, h)
				h.click(h.App.S.Settings.GuideSkip)
				onboardingAssertClosed(t, h)
				if settings.saves != 1 || svc.Settings.Get() != want {
					t.Fatal("skipping replay changed completion or other preferences")
				}
				if !reflect.DeepEqual(h.App.Active().Request, request) || len(svc.History.List()) != 0 {
					t.Fatal("guide changed the request or created history")
				}
			})
		}
	}
}

func TestOnboardingGuideSaveFailureCanRetryPreservingLatestSettings(t *testing.T) {
	for _, language := range []string{"zh", "en"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv(storage.EnvHome, t.TempDir())
			svc := fake.New()
			original := model.DefaultSettings()
			original.Language = language
			if err := svc.Settings.Save(original); err != nil {
				t.Fatal(err)
			}
			settings := &onboardingSettingsProbe{SettingsService: svc.Settings, fail: true}
			svc.Settings = settings
			h := newHarness(t, svc)
			h.click(h.App.S.Settings.Title)
			h.click(h.App.S.Settings.OpenGuide)
			if err := h.D.Resize(960, 600); err != nil {
				t.Fatal(err)
			}
			h.click(h.App.S.Settings.GuideDone)
			if !h.App.settings.guideOpen || !onboardingShowsWrapped(h, h.App.S.Settings.GuideSaveFailed) ||
				settings.saves != 1 || svc.Settings.Get() != original {
				t.Fatal("failed save should leave the guide open with localized feedback and unchanged settings")
			}
			onboardingAssertGuide(t, h)
			// Another feature may have saved preferences after the guide was painted.
			latest := original
			latest.Version, latest.Theme, latest.ActiveEnvID = 7, "system", "new-environment"
			latest.HistoryLimit, latest.RequestTimeoutSec = 500, 120
			latest.NoFollowRedirects, latest.InsecureTLS = true, true
			if err := settings.SettingsService.Save(latest); err != nil {
				t.Fatal(err)
			}
			settings.fail = false
			h.click(h.App.S.Settings.GuideDone)
			onboardingAssertClosed(t, h)
			latest.OnboardingCompleted = true
			if settings.saves != 2 || svc.Settings.Get() != latest || !h.shows(h.App.S.Settings.ReplayGuide) {
				t.Fatal("retry did not save completion using the latest preferences")
			}
			h.click(h.App.S.Settings.ReplayGuide)
			if h.App.settings.guideError != "" || onboardingShowsWrapped(h, h.App.S.Settings.GuideSaveFailed) {
				t.Fatal("replaying a saved guide retained a stale save error")
			}
		})
	}
}

type onboardingSettingsProbe struct {
	service.SettingsService
	saves int
	fail  bool
}

func (s *onboardingSettingsProbe) Save(settings model.Settings) error {
	s.saves++
	if s.fail {
		return errors.New("settings write failed")
	}
	return s.SettingsService.Save(settings)
}

func onboardingAssertGuide(t *testing.T, h *harness) {
	t.Helper()
	s := &h.App.S.Settings
	for _, text := range []string{s.GuideTitle, s.GuideIntro, s.GuideRequestTitle, s.GuideRequestDescription,
		s.GuideVariablesTitle, s.GuideVariablesDescription, s.GuideDataTitle, s.GuideDataDescription} {
		if !onboardingShowsWrapped(h, text) {
			t.Errorf("guide does not display %q", text)
		}
	}
	// Footer actions must remain reachable, including at the minimum window size.
	for _, label := range []string{s.GuideDone, s.GuideSkip} {
		found := false
		for _, hit := range h.D.Snapshot().Hits {
			if hit.Label == label {
				found = true
				if hit.Bounds.Y < 0 || hit.Bounds.Y+hit.Bounds.Height > float32(h.D.Snapshot().Height) {
					t.Errorf("guide action %q falls outside the viewport: %+v", label, hit.Bounds)
				}
			}
		}
		if !found {
			t.Errorf("guide action %q is missing", label)
		}
	}
}

func onboardingAssertClosed(t *testing.T, h *harness) {
	t.Helper()
	if h.App.settings.guideOpen || h.shows(h.App.S.Settings.GuideTitle) {
		t.Fatal("guide is still open")
	}
}

func onboardingShowsWrapped(h *harness, text string) bool {
	var painted strings.Builder
	for _, label := range h.D.Snapshot().Labels {
		painted.WriteString(label.Value)
	}
	return strings.Contains(strings.Join(strings.Fields(painted.String()), ""), strings.Join(strings.Fields(text), ""))
}
