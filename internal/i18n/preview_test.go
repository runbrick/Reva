package i18n

import (
	"reflect"
	"strings"
	"testing"
)

func TestPreviewTranslationsPresentInBothLanguages(t *testing.T) {
	for _, lang := range []Lang{Zh, En} {
		v := reflect.ValueOf(For(lang).Request)
		for i := 0; i < v.NumField(); i++ {
			if strings.HasPrefix(v.Type().Field(i).Name, "Preview") && strings.TrimSpace(v.Field(i).String()) == "" {
				t.Errorf("%s missing %s", lang, v.Type().Field(i).Name)
			}
		}
	}
	if !strings.Contains(requestZh.PreviewPhase, "前置脚本之前") || !strings.Contains(requestEn.PreviewPhase, "BEFORE pre-request scripts") {
		t.Fatal("preview timing must be explicit in both languages")
	}
	for _, text := range []string{requestZh.PreviewMaskHelp, requestEn.PreviewMaskHelp} {
		if !strings.Contains(text, "[masked]") {
			t.Fatal("mask notation must be explained")
		}
	}
}
