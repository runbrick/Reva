package store_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
)

// Settings saves can also persist a history trim. Completion must not become
// durable if that second write fails, even after settings.json was written.
func TestOnboardingCompletionRollsBackWhenHistoryPersistenceFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	s := openStore(t, dir)
	original := model.DefaultSettings()
	original.Theme, original.Language, original.ActiveEnvID = "dark", "en", "production"
	must(t, s.Settings().Save(original))
	for _, id := range []string{"first", "second"} {
		must(t, s.History().Add(model.HistoryEntry{ID: id, Request: request(id)}))
	}
	history := s.History().List()
	settingsPath := filepath.Join(dir, "settings.json")
	before, err := os.ReadFile(settingsPath)
	must(t, err)
	historyPath, backup := filepath.Join(dir, "history.json"), filepath.Join(dir, "history.saved")
	must(t, os.Rename(historyPath, backup))
	must(t, os.Mkdir(historyPath, 0o755))
	completed := original
	completed.OnboardingCompleted, completed.HistoryLimit = true, 1
	if err := s.Settings().Save(completed); err == nil {
		t.Fatal("expected history persistence failure")
	}
	after, err := os.ReadFile(settingsPath)
	must(t, err)
	if !bytes.Equal(before, after) || s.Settings().Get() != original {
		t.Fatal("failed history write persisted or published onboarding completion")
	}
	if !reflect.DeepEqual(s.History().List(), history) {
		t.Fatal("failed completion save trimmed history in memory")
	}
	must(t, os.Remove(historyPath)) // empty test-created directory
	must(t, os.Rename(backup, historyPath))
	reloaded := openStore(t, dir)
	if reloaded.Settings().Get() != original || !reflect.DeepEqual(reloaded.History().List(), history) {
		t.Fatal("reload after failed completion lost pending status, preferences or history")
	}
	must(t, reloaded.Settings().Save(completed))
	reloaded = openStore(t, dir)
	if reloaded.Settings().Get() != completed {
		t.Fatal("retry did not persist completion with all requested preferences")
	}
	if got := reloaded.History().List(); !reflect.DeepEqual(got, history[:1]) {
		t.Fatalf("retry did not persist the requested history limit: %+v", got)
	}
}
