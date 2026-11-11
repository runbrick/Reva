package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteThenReadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "x.json")
	if err := WriteJSON(path, map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	var out map[string]int
	if err := ReadJSON(path, &out); err != nil {
		t.Fatal(err)
	}
	if out["a"] != 1 {
		t.Fatalf("got %v", out)
	}
}

func TestReadMissingIsNotExist(t *testing.T) {
	var v any
	err := ReadJSON(filepath.Join(t.TempDir(), "nope.json"), &v)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want ErrNotExist, got %v", err)
	}
}

func TestDirHonorsEnv(t *testing.T) {
	want := filepath.Join(t.TempDir(), "home")
	t.Setenv(EnvHome, want)
	got, err := Dir()
	if err != nil || got != want {
		t.Fatalf("Dir() = %q, %v; want %q", got, err, want)
	}
}
