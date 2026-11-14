package model_test

import (
	"encoding/json"
	"testing"

	"github.com/runbrick/reva/internal/model"
)

func TestOnboardingIncompleteByDefault(t *testing.T) {
	if (model.Settings{}).OnboardingCompleted {
		t.Fatal("zero settings must leave onboarding incomplete")
	}
	if model.DefaultSettings().OnboardingCompleted {
		t.Fatal("default settings must leave onboarding incomplete")
	}
}

func TestOnboardingSettingsJSONCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want bool
	}{
		{"legacy", `{"version":1,"theme":"dark","language":"en","historyLimit":17}`, false},
		{"incomplete", `{"version":1,"theme":"dark","language":"en","historyLimit":17,"onboardingCompleted":false}`, false},
		{"completed", `{"version":1,"theme":"dark","language":"en","historyLimit":17,"onboardingCompleted":true}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got model.Settings
			if err := json.Unmarshal([]byte(tt.data), &got); err != nil {
				t.Fatal(err)
			}
			want := model.Settings{Version: 1, Theme: "dark", Language: "en", HistoryLimit: 17, OnboardingCompleted: tt.want}
			if got != want {
				t.Fatalf("decoded settings = %+v, want %+v", got, want)
			}
		})
	}
}

func TestOnboardingSettingsJSONRoundTrip(t *testing.T) {
	for _, completed := range []bool{false, true} {
		settings := model.DefaultSettings()
		settings.OnboardingCompleted = completed
		data, err := json.Marshal(settings)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		value, present := fields["onboardingCompleted"]
		if present != completed || (completed && string(value) != "true") {
			t.Fatalf("completed=%v: unexpected onboardingCompleted field in %s", completed, data)
		}
		var got model.Settings
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got != settings {
			t.Fatalf("round-trip settings = %+v, want %+v", got, settings)
		}
	}
}
