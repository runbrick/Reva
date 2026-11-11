package store

import (
	"path/filepath"
	"slices"
	"time"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
)

type historyService struct{ s *Store }

func (h historyService) List() []model.HistoryEntry {
	h.s.mu.RLock()
	defer h.s.mu.RUnlock()
	return cloneHistory(h.s.history)
}

func (h historyService) Add(entry model.HistoryEntry) error {
	h.s.mu.Lock()
	defer h.s.mu.Unlock()
	if entry.ID == "" {
		entry.ID = storage.NewID()
	}
	if entry.At.IsZero() {
		entry.At = time.Now().UTC()
	}
	entry.Request = entry.Request.Clone()
	entries := append([]model.HistoryEntry{entry}, h.s.history...)
	return h.s.saveHistory(trim(entries, h.s.settings.HistoryLimit))
}

func (h historyService) Delete(id string) error {
	h.s.mu.Lock()
	defer h.s.mu.Unlock()
	entries := slices.DeleteFunc(slices.Clone(h.s.history), func(e model.HistoryEntry) bool { return e.ID == id })
	return h.s.saveHistory(entries)
}

func (h historyService) Clear() error {
	h.s.mu.Lock()
	defer h.s.mu.Unlock()
	return h.s.saveHistory(nil)
}

func trim(entries []model.HistoryEntry, limit int) []model.HistoryEntry {
	if len(entries) > limit {
		return slices.Clone(entries[:limit])
	}
	return entries
}

func (s *Store) saveHistory(entries []model.HistoryEntry) error {
	if err := s.writeHistory(entries); err != nil {
		return err
	}
	s.history = entries
	return nil
}

func (s *Store) writeHistory(entries []model.HistoryEntry) error {
	records := make([]historyRecord, len(entries))
	for i, entry := range entries {
		records[i] = historyRecord{Version: model.SchemaVersion, HistoryEntry: entry}
	}
	return storage.WriteJSON(filepath.Join(s.dir, "history.json"), records)
}
