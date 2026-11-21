package responseexample

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/runbrick/reva/internal/storage"
)

const maxFileBytes = 6*MaxBodyBytes + 8192

type Key struct {
	CollectionID string
	RequestID    string
}

type record struct {
	Version int `json:"version"`
	Sample
}

func (key Key) path() (string, error) {
	if key.CollectionID == "" || key.RequestID == "" || strings.ContainsRune(key.CollectionID, 0) || strings.ContainsRune(key.RequestID, 0) {
		return "", ErrInvalid
	}
	dir, err := storage.Dir()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(key.CollectionID + "\x00" + key.RequestID))
	return filepath.Join(dir, "examples", hex.EncodeToString(digest[:])+".json"), nil
}

func Load(key Key) (Sample, bool, error) {
	path, err := key.path()
	if err != nil {
		return Sample{}, false, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Sample{}, false, nil
	}
	if err != nil {
		return Sample{}, false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return Sample{}, false, err
	}
	if len(data) > maxFileBytes {
		return Sample{}, false, ErrInvalid
	}
	var saved record
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&saved); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF || saved.Version != 1 || !saved.Sample.valid() {
		return Sample{}, false, ErrInvalid
	}
	return saved.Sample, true, nil
}

func Save(key Key, sample Sample) error {
	if !sample.valid() {
		return ErrInvalid
	}
	path, err := key.path()
	if err != nil {
		return err
	}
	return storage.WriteJSON(path, record{Version: 1, Sample: sample})
}

func Delete(key Key) error {
	path, err := key.path()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func WriteMapping(path string, reviewed []byte) error {
	return storage.WriteFileAtomic(path, reviewed)
}
