package i18n_test

import (
	"testing"

	"github.com/runbrick/reva/internal/i18n"
)

func TestOnboardingActionCopy(t *testing.T) {
	for _, field := range []struct {
		name string
		text func(i18n.Settings) string
		zh   string
		en   string
	}{
		{"GuidePractice", func(settings i18n.Settings) string { return settings.GuidePractice }, "离线预览示例", "Preview sample offline"},
		{"GuidePracticeName", func(settings i18n.Settings) string { return settings.GuidePracticeName }, "示例请求", "Sample request"},
		{"GuideImport", func(settings i18n.Settings) string { return settings.GuideImport }, "导入已有请求", "Import existing requests"},
	} {
		t.Run(field.name, func(t *testing.T) {
			for _, language := range []i18n.Lang{i18n.Zh, i18n.En, "", "fr", "zh-CN"} {
				want := field.zh
				if language == i18n.En {
					want = field.en
				}
				if got := field.text(i18n.For(language).Settings); got != want {
					t.Errorf("language %q: got %q, want %q", language, got, want)
				}
			}
		})
	}
}
