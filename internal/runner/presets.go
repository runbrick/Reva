package runner

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/runbrick/reva/internal/storage"
)

type Preset struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	FolderID         string `json:"folderId,omitempty"`
	EnvironmentID    string `json:"environmentId,omitempty"`
	StopOnFailure    bool   `json:"stopOnFailure"`
	UseCookieSession bool   `json:"useCookieSession"`
}

var (
	ErrEmptyPresetName     = errors.New("empty runner preset name")
	ErrDuplicatePresetName = errors.New("duplicate runner preset name")
	ErrPresetNotFound      = errors.New("runner preset not found")
	invalidPresetFile      = errors.New("invalid runner preset file")
	presetsMu              sync.Mutex
)

const presetFileName = "runner-presets.json"

type presetFile struct {
	Version     int                 `json:"version"`
	Collections map[string][]Preset `json:"collections"`
}

func ListPresets(collectionID string) ([]Preset, error) {
	presetsMu.Lock()
	defer presetsMu.Unlock()
	file, _, err := readPresets()
	return file.Collections[collectionID], err
}

func SavePreset(collectionID string, preset Preset) ([]Preset, string, error) {
	preset.Name = strings.TrimSpace(preset.Name)
	if preset.Name == "" {
		return nil, "", ErrEmptyPresetName
	}
	presetsMu.Lock()
	defer presetsMu.Unlock()
	file, path, err := readPresets()
	if err != nil {
		return nil, "", err
	}
	presets := file.Collections[collectionID]
	index := -1
	for offset, saved := range presets {
		if saved.ID == preset.ID {
			index = offset
		} else if strings.EqualFold(saved.Name, preset.Name) {
			return nil, "", ErrDuplicatePresetName
		}
	}
	switch {
	case preset.ID == "":
		preset.ID = storage.NewID()
		presets = append(presets, preset)
	case index == -1:
		return nil, "", ErrPresetNotFound
	default:
		presets[index] = preset
	}
	file.Collections[collectionID] = presets
	if err := storage.WriteJSON(path, file); err != nil {
		return nil, "", err
	}
	return presets, preset.ID, nil
}

func DeletePreset(collectionID, id string) ([]Preset, error) {
	presetsMu.Lock()
	defer presetsMu.Unlock()
	file, path, err := readPresets()
	if err != nil {
		return nil, err
	}
	presets := file.Collections[collectionID]
	index := slices.IndexFunc(presets, func(preset Preset) bool { return preset.ID == id })
	if index < 0 {
		return nil, ErrPresetNotFound
	}
	presets = slices.Delete(presets, index, index+1)
	if len(presets) == 0 {
		delete(file.Collections, collectionID)
	} else {
		file.Collections[collectionID] = presets
	}
	if err := storage.WriteJSON(path, file); err != nil {
		return nil, err
	}
	return presets, nil
}

func readPresets() (presetFile, string, error) {
	dir, err := storage.Dir()
	if err != nil {
		return presetFile{}, "", err
	}
	path := filepath.Join(dir, presetFileName)
	var file presetFile
	if err := storage.ReadJSON(path, &file); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return presetFile{Version: 1, Collections: make(map[string][]Preset)}, path, nil
		}
		return presetFile{}, "", err
	}
	if file.Version != 1 || file.Collections == nil {
		return presetFile{}, "", invalidPresetFile
	}
	for _, presets := range file.Collections {
		ids := make(map[string]struct{}, len(presets))
		for _, preset := range presets {
			if _, duplicate := ids[preset.ID]; duplicate || preset.ID == "" || strings.TrimSpace(preset.Name) == "" {
				return presetFile{}, "", invalidPresetFile
			}
			ids[preset.ID] = struct{}{}
		}
	}
	return file, path, nil
}
