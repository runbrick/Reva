package i18n_test

import (
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
)

func TestOnboardingFieldsAreTranslated(t *testing.T) {
	fields := []struct {
		name string
		text func(i18n.Settings) string
	}{
		{"GettingStarted", func(s i18n.Settings) string { return s.GettingStarted }},
		{"GettingStartedDescription", func(s i18n.Settings) string { return s.GettingStartedDescription }},
		{"OpenGuide", func(s i18n.Settings) string { return s.OpenGuide }},
		{"ReplayGuide", func(s i18n.Settings) string { return s.ReplayGuide }},
		{"GuideTitle", func(s i18n.Settings) string { return s.GuideTitle }},
		{"GuideIntro", func(s i18n.Settings) string { return s.GuideIntro }},
		{"GuideRequestTitle", func(s i18n.Settings) string { return s.GuideRequestTitle }},
		{"GuideRequestDescription", func(s i18n.Settings) string { return s.GuideRequestDescription }},
		{"GuideVariablesTitle", func(s i18n.Settings) string { return s.GuideVariablesTitle }},
		{"GuideVariablesDescription", func(s i18n.Settings) string { return s.GuideVariablesDescription }},
		{"GuideDataTitle", func(s i18n.Settings) string { return s.GuideDataTitle }},
		{"GuideDataDescription", func(s i18n.Settings) string { return s.GuideDataDescription }},
		{"GuideDone", func(s i18n.Settings) string { return s.GuideDone }},
		{"GuideSkip", func(s i18n.Settings) string { return s.GuideSkip }},
		{"GuideSaveFailed", func(s i18n.Settings) string { return s.GuideSaveFailed }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
				if strings.TrimSpace(field.text(i18n.For(lang).Settings)) == "" {
					t.Errorf("%s: missing translation", lang)
				}
			}
			if field.text(i18n.For(i18n.Zh).Settings) == field.text(i18n.For(i18n.En).Settings) {
				t.Error("Chinese and English onboarding copy must each be localized")
			}
		})
	}
}

func TestOnboardingCopyExplainsUserChoices(t *testing.T) {
	for _, tc := range []struct {
		lang      i18n.Lang
		intro     []string
		request   []string
		variables []string
		data      []string
		saveError []string
	}{
		{
			lang:      i18n.Zh,
			intro:     []string{"随时关闭", "标记为已完成", "只关闭指南，不标记完成", "设置中重新查看"},
			request:   []string{"输入 URL", "选择 HTTP 方法", "手动点击", "curl", "Postman"},
			variables: []string{"环境", "{{name}}", "自己的配置", "凭据"},
			data:      []string{"本机", "无需账号", "按需配置", "加密备份", "同步"},
			saveError: []string{"完成状态", "重试"},
		},
		{
			lang:      i18n.En,
			intro:     []string{"Close this guide anytime", "Done marks it complete", "without marking it complete", "Replay it anytime from Settings"},
			request:   []string{"Enter a URL", "choose an HTTP method", "click Send manually", "curl", "Postman"},
			variables: []string{"environment", "{{name}}", "credentials", "your own configuration"},
			data:      []string{"this device", "no account required", "Encrypted backup and sync are optional"},
			saveError: []string{"guide completion", "try again"},
		},
	} {
		t.Run(string(tc.lang), func(t *testing.T) {
			s := i18n.For(tc.lang).Settings
			for _, section := range []struct {
				name string
				text string
				want []string
			}{
				{"GuideIntro", s.GuideIntro, append(tc.intro, s.GuideDone, s.GuideSkip)},
				{"GuideRequestDescription", s.GuideRequestDescription, tc.request},
				{"GuideVariablesDescription", s.GuideVariablesDescription, tc.variables},
				{"GuideDataDescription", s.GuideDataDescription, tc.data},
				{"GuideSaveFailed", s.GuideSaveFailed, tc.saveError},
			} {
				for _, want := range section.want {
					if !strings.Contains(section.text, want) {
						t.Errorf("%s must explain %q: %q", section.name, want, section.text)
					}
				}
			}
		})
	}
}
