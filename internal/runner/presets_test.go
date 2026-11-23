package runner

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRunnerPresetsRoundTripAndCollectionIsolation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	if loaded, err := ListPresets("first"); err != nil || len(loaded) != 0 {
		t.Fatalf("new preset store = %v, %v", loaded, err)
	}
	if _, err := os.Stat(filepath.Join(dir, presetFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("listing created the preset file: %v", err)
	}
	preset := Preset{Name: "  冒烟 Smoke  ", FolderID: "folder", EnvironmentID: "sandbox", StopOnFailure: true, UseCookieSession: true}
	_, id, err := SavePreset("first", preset)
	if err != nil || id == "" {
		t.Fatalf("create preset = %q, %v", id, err)
	}
	preset.ID, preset.Name = id, "冒烟 Smoke"
	if loaded, err := ListPresets("first"); err != nil || !slices.Equal(loaded, []Preset{preset}) {
		t.Fatalf("reopened preset store = %v, %v", loaded, err)
	}
	if _, _, err := SavePreset("first", Preset{Name: "冒烟 smoke"}); !errors.Is(err, ErrDuplicatePresetName) {
		t.Fatalf("duplicate preset = %v", err)
	}
	if _, _, err := SavePreset("first", Preset{Name: " \t "}); !errors.Is(err, ErrEmptyPresetName) {
		t.Fatalf("empty preset name = %v", err)
	}
	other := Preset{Name: preset.Name}
	_, otherID, err := SavePreset("second", other)
	if err != nil || otherID == "" || otherID == id {
		t.Fatalf("create same name in another collection = %q, %v", otherID, err)
	}
	other.ID = otherID
	if _, err := DeletePreset("second", id); !errors.Is(err, ErrPresetNotFound) {
		t.Fatalf("cross-collection delete = %v", err)
	}
	preset.Name, preset.FolderID, preset.UseCookieSession = "Renamed", "", false
	updated, updatedID, err := SavePreset("first", preset)
	if err != nil || id != updatedID || !slices.Equal(updated, []Preset{preset}) {
		t.Fatalf("updated preset = %v, %q, %v", updated, updatedID, err)
	}
	if _, err := DeletePreset("first", id); err != nil {
		t.Fatal(err)
	}
	if loaded, err := ListPresets("first"); err != nil || len(loaded) != 0 {
		t.Fatalf("deleted preset remains: %v, %v", loaded, err)
	}
	if loaded, err := ListPresets("second"); err != nil || !slices.Equal(loaded, []Preset{other}) {
		t.Fatalf("other collection changed: %v, %v", loaded, err)
	}
}

func TestRunnerPresetsDoNotOverwriteUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	path := filepath.Join(dir, presetFileName)
	for _, raw := range []string{`{"corrupt`, `{"version":2,"collections":{}}`, `{"version":1,"collections":{"collection":[{}]}}`} {
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ListPresets("collection"); err == nil {
			t.Fatal("read accepted corrupt or unsupported data")
		}
		if _, _, err := SavePreset("collection", Preset{Name: "Smoke"}); err == nil {
			t.Fatal("save accepted corrupt or unsupported data")
		}
		if _, err := DeletePreset("collection", "missing"); err == nil {
			t.Fatal("delete accepted corrupt or unsupported data")
		}
		if after, err := os.ReadFile(path); err != nil || string(after) != raw {
			t.Fatalf("runner presets changed on failure: %q, %v", after, err)
		}
	}
}
