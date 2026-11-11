// Package store implements the local, disk-backed services. Each mutation is
// persisted before it becomes visible to readers of this Store.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/storage"
)

// ErrNotFound indicates that a collection, environment, folder or item is absent.
var ErrNotFound = errors.New("not found")

// Store holds a snapshot of the data directory, shared by its four services.
// A Store is safe for concurrent use; open only one Store per directory.
type Store struct {
	mu           sync.RWMutex
	dir          string
	settings     model.Settings
	history      []model.HistoryEntry
	collections  map[string]model.Collection
	environments map[string]model.Environment
	loadErrors   []error
}

// historyRecord keeps history.json an array while recording the schema version
// without changing the HistoryEntry contract shared with other features.
type historyRecord struct {
	Version int `json:"version"`
	model.HistoryEntry
}

// Open loads all local data. Bad individual files are left untouched and
// reported by LoadErrors; errors preparing the data directories are fatal.
func Open(dir string) (*Store, error) {
	s := &Store{
		dir: dir, settings: model.DefaultSettings(),
		collections:  make(map[string]model.Collection),
		environments: make(map[string]model.Environment),
	}
	for _, sub := range []string{"", "collections", "environments"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	var settings model.Settings
	if s.read("settings.json", &settings) {
		s.settings = defaults(settings)
	}
	var records []historyRecord
	if s.read("history.json", &records) {
		for _, record := range records {
			s.history = append(s.history, record.HistoryEntry)
		}
		s.history = trim(s.history, s.settings.HistoryLimit)
	}
	if err := s.loadObjects("collections", func(path string) error {
		var col model.Collection
		if err := storage.ReadJSON(path, &col); err != nil {
			return err
		}
		if err := validateCollection(col); err != nil {
			return err
		}
		if filepath.Base(path) != col.ID+".json" {
			return errors.New("collection ID does not match filename")
		}
		s.collections[col.ID] = col
		return nil
	}); err != nil {
		return nil, err
	}
	if err := s.loadObjects("environments", func(path string) error {
		var env model.Environment
		if err := storage.ReadJSON(path, &env); err != nil {
			return err
		}
		if err := validateID(env.ID); err != nil {
			return err
		}
		if err := validateName(env.Name); err != nil {
			return err
		}
		if filepath.Base(path) != env.ID+".json" {
			return errors.New("environment ID does not match filename")
		}
		s.environments[env.ID] = env
		return nil
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) read(name string, value any) bool {
	if err := storage.ReadJSON(filepath.Join(s.dir, name), value); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.loadErrors = append(s.loadErrors, fmt.Errorf("load %s: %w", name, err))
		}
		return false
	}
	return true
}

func (s *Store) loadObjects(sub string, load func(string) error) error {
	entries, err := os.ReadDir(filepath.Join(s.dir, sub))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.dir, sub, entry.Name())
		if err := load(path); err != nil {
			s.loadErrors = append(s.loadErrors, fmt.Errorf("load %s: %w", path, err))
		}
	}
	return nil
}

// LoadErrors returns the individual files that could not be loaded at Open.
func (s *Store) LoadErrors() []error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.loadErrors)
}

func (s *Store) History() service.HistoryService          { return historyService{s} }
func (s *Store) Collections() service.CollectionService   { return collectionService{s} }
func (s *Store) Environments() service.EnvironmentService { return environmentService{s} }
func (s *Store) Settings() service.SettingsService        { return settingsService{s} }

func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("name must not be empty")
	}
	return nil
}

// IDs become filenames, so reject separators and volume/stream syntax even on
// platforms where some of those characters would be valid filenames.
func validateID(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\:<>"|?*`+"\x00") || strings.HasSuffix(id, ".") || strings.HasSuffix(id, " ") {
		return errors.New("invalid ID")
	}
	return nil
}

func cloneCollection(col model.Collection) model.Collection {
	col.Items = cloneItems(col.Items)
	col.Variables = slices.Clone(col.Variables)
	return col
}

func cloneItems(items []model.Item) []model.Item {
	items = slices.Clone(items)
	for i := range items {
		items[i].Items = cloneItems(items[i].Items)
		if items[i].Request != nil {
			req := items[i].Request.Clone()
			items[i].Request = &req
		}
	}
	return items
}

func cloneEnvironment(env model.Environment) model.Environment {
	env.Variables = slices.Clone(env.Variables)
	return env
}

func cloneHistory(entries []model.HistoryEntry) []model.HistoryEntry {
	entries = slices.Clone(entries)
	for i := range entries {
		entries[i].Request = entries[i].Request.Clone()
	}
	return entries
}
