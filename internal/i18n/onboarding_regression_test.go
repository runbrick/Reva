package i18n_test

import (
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
)

// Legacy settings may omit language, and imported settings may use an unknown
// language. The guide must remain usable through the Chinese fallback.
func TestOnboardingCopyFallsBackToChinese(t *testing.T) {
	guide := func(s i18n.Settings) [15]string {
		return [15]string{s.GettingStarted, s.GettingStartedDescription, s.OpenGuide, s.ReplayGuide,
			s.GuideTitle, s.GuideIntro, s.GuideRequestTitle, s.GuideRequestDescription,
			s.GuideVariablesTitle, s.GuideVariablesDescription, s.GuideDataTitle,
			s.GuideDataDescription, s.GuideDone, s.GuideSkip, s.GuideSaveFailed}
	}
	want := guide(i18n.For(i18n.Zh).Settings)
	for _, language := range []i18n.Lang{"", "fr", "zh-CN"} {
		t.Run(string(language), func(t *testing.T) {
			if got := guide(i18n.For(language).Settings); got != want {
				t.Fatalf("guide copy for %q did not fall back to Chinese: %q", language, got)
			}
		})
	}
}
