package i18n

import (
	"reflect"
	"strings"
	"testing"
)

func TestCollectionSettingsTranslationsComplete(t *testing.T) {
	for _, lang := range []Lang{Zh, En} {
		t.Run(string(lang), func(t *testing.T) {
			value := reflect.ValueOf(For(lang).Collections)
			for i := 0; i < value.NumField(); i++ {
				text := value.Field(i).String()
				if strings.TrimSpace(text) == "" || strings.Contains(text, "????") {
					t.Errorf("Collections.%s missing translation", value.Type().Field(i).Name)
				}
			}
		})
	}
}
