package i18n

import (
	"reflect"
	"testing"
)

// TestEveryStringIsTranslated fails when a field is set in one language but
// left empty in the other.
func TestEveryStringIsTranslated(t *testing.T) {
	check(t, "", reflect.ValueOf(zh), reflect.ValueOf(en))
}

func check(t *testing.T, path string, a, b reflect.Value) {
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			check(t, path+"."+a.Type().Field(i).Name, a.Field(i), b.Field(i))
		}
	case reflect.String:
		if (a.String() == "") != (b.String() == "") {
			t.Errorf("%s: zh=%q en=%q", path, a.String(), b.String())
		}
	}
}

func TestForFallsBackToChinese(t *testing.T) {
	if For("fr").Lang != Zh || For(En).Lang != En {
		t.Fatal("unexpected language selection")
	}
}
