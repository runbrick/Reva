package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
)

func TestOnboardingSettingsLoadCompatibility(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want model.Settings
	}{
		{"new", "", model.DefaultSettings()},
		{"legacy", `{"version":1,"theme":"dark","language":"en","activeEnvId":"staging","historyLimit":17,"noFollowRedirects":true,"insecureTls":true}`, model.Settings{
			Version: 1, Theme: "dark", Language: "en", ActiveEnvID: "staging", HistoryLimit: 17,
			RequestTimeoutSec: 60, NoFollowRedirects: true, InsecureTLS: true,
		}},
		{"completed with missing preferences", `{"version":1,"onboardingCompleted":true}`, model.Settings{
			Version: 1, Theme: "light", Language: "zh", HistoryLimit: 200, RequestTimeoutSec: 60, OnboardingCompleted: true,
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv(storage.EnvHome, dir)
			path := filepath.Join(dir, "settings.json")
			if tt.data != "" {
				must(t, os.WriteFile(path, []byte(tt.data), 0o600))
			}
			s := openStore(t, dir)
			if got := s.Settings().Get(); got != tt.want {
				t.Fatalf("loaded settings = %+v, want %+v", got, tt.want)
			}
			must(t, s.Settings().Save(s.Settings().Get()))
			if got := openStore(t, dir).Settings().Get(); got != tt.want {
				t.Fatalf("reopened settings = %+v, want %+v", got, tt.want)
			}
			var fields map[string]json.RawMessage
			must(t, storage.ReadJSON(path, &fields))
			if string(fields["version"]) != "1" {
				t.Fatalf("schema version changed: %s", fields["version"])
			}
			value, present := fields["onboardingCompleted"]
			if present != tt.want.OnboardingCompleted || (present && string(value) != "true") {
				t.Fatalf("unexpected persisted completion field: %s (present=%v)", value, present)
			}
		})
	}
}

func TestOnboardingCompletionSaveDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	s := openStore(t, dir)
	// Saving sparse preferences must fill defaults without resetting completion.
	must(t, s.Settings().Save(model.Settings{OnboardingCompleted: true}))
	want := model.DefaultSettings()
	want.OnboardingCompleted = true
	if got := s.Settings().Get(); got != want {
		t.Fatalf("saved settings = %+v, want %+v", got, want)
	}
	s = openStore(t, dir)
	if got := s.Settings().Get(); got != want {
		t.Fatalf("reopened settings = %+v, want %+v", got, want)
	}
	// An explicit false remains representable after previously completing setup.
	want.OnboardingCompleted = false
	must(t, s.Settings().Save(want))
	if got := openStore(t, dir).Settings().Get(); got != want {
		t.Fatalf("reset completion did not persist: %+v", got)
	}
}

func TestOnboardingCompletionPreservesPreferences(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	s := openStore(t, dir)
	want := model.Settings{
		Version: 1, Theme: "dark", Language: "en", ActiveEnvID: "staging", HistoryLimit: 17,
		RequestTimeoutSec: 12, NoFollowRedirects: true, InsecureTLS: true,
	}
	must(t, s.Settings().Save(want))
	s = openStore(t, dir)
	settings := s.Settings().Get()
	settings.OnboardingCompleted = true
	must(t, s.Settings().Save(settings))
	want.OnboardingCompleted = true
	s = openStore(t, dir)
	if got := s.Settings().Get(); got != want {
		t.Fatalf("completing onboarding changed preferences: got %+v, want %+v", got, want)
	}
	settings = s.Settings().Get()
	settings.Theme, settings.Language, settings.ActiveEnvID = "system", "zh", "production"
	settings.HistoryLimit, settings.RequestTimeoutSec = 29, 23
	settings.NoFollowRedirects, settings.InsecureTLS = false, false
	must(t, s.Settings().Save(settings))
	if got := openStore(t, dir).Settings().Get(); got != settings {
		t.Fatalf("changing preferences lost completion or other settings: got %+v, want %+v", got, settings)
	}
}
