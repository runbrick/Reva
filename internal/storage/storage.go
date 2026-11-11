// Package storage locates Reqo's data directory and reads and writes JSON
// files in it atomically. Services build on it; the UI never touches files.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// EnvHome overrides the data directory, for tests and portable installs.
const EnvHome = "REQO_HOME"

// Dir returns the data directory: $REQO_HOME if set, otherwise
// <os.UserConfigDir>/Reqo. It is created if missing.
//
// Layout:
//
//	settings.json
//	history.json
//	collections/<id>.json
//	environments/<id>.json
func Dir() (string, error) {
	dir := os.Getenv(EnvHome)
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("locate config dir: %w", err)
		}
		dir = filepath.Join(base, "Reqo")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// ReadJSON decodes the file at path into v. The error wraps os.ErrNotExist
// when the file does not exist.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

// WriteJSON writes v as indented JSON to path atomically.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, append(data, '\n'))
}

// WriteFileAtomic writes data to a temporary file next to path and renames
// it over path, so a crash never leaves a half-written file.
func WriteFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// NewID returns a random 16-hex-digit identifier.
func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
