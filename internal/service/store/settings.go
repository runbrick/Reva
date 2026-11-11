package store

import (
	"errors"
	"path/filepath"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
)

type settingsService struct{ s *Store }

func defaults(settings model.Settings) model.Settings {
	d := model.DefaultSettings()
	settings.Version = model.SchemaVersion
	if settings.Theme == "" {
		settings.Theme = d.Theme
	}
	if settings.Language == "" {
		settings.Language = d.Language
	}
	if settings.HistoryLimit <= 0 {
		settings.HistoryLimit = d.HistoryLimit
	}
	if settings.RequestTimeoutSec <= 0 {
		settings.RequestTimeoutSec = d.RequestTimeoutSec
	}
	return settings
}

func (v settingsService) Get() model.Settings {
	v.s.mu.RLock()
	defer v.s.mu.RUnlock()
	return v.s.settings
}

func (v settingsService) Save(settings model.Settings) error {
	v.s.mu.Lock()
	defer v.s.mu.Unlock()
	settings = defaults(settings)
	path := filepath.Join(v.s.dir, "settings.json")
	if err := storage.WriteJSON(path, settings); err != nil {
		return err
	}
	if len(v.s.history) > settings.HistoryLimit {
		entries := trim(v.s.history, settings.HistoryLimit)
		if err := v.s.saveHistory(entries); err != nil {
			// Restore the old preferences when persisting the trim fails.
			return errors.Join(err, storage.WriteJSON(path, v.s.settings))
		}
	}
	v.s.settings = settings
	return nil
}
