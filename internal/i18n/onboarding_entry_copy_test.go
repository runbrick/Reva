package i18n_test

import (
	"testing"

	"github.com/runbrick/reva/internal/i18n"
)

func TestOnboardingWorkbenchEntryCopy(t *testing.T) {
	for _, language := range []i18n.Lang{i18n.Zh, i18n.En, "", "fr", "zh-CN"} {
		want := "开始使用"
		if language == i18n.En {
			want = "Getting started"
		}
		if got := i18n.For(language).Settings.GuideWorkbenchOpen; got != want {
			t.Errorf("language %q: got %q, want %q", language, got, want)
		}
	}
}
