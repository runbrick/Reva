// Package service declares the contracts between Reva's UI and its Go
// back end. The GUI depends only on these interfaces, so each pane can be
// tested with fakes; the concrete implementations live in sub-packages
// (service/request, service/history, service/collection, service/environment,
// service/settings) and are wired together in cmd/reva.
//
// Every method is safe to call from the UI thread. Methods that touch the
// network take a context and must return promptly when it is cancelled.
// Methods that touch the disk are expected to be fast (small JSON files)
// and may be called directly from a click handler.
package service

import (
	"context"

	"github.com/runbrick/reva/internal/model"
)

// RequestService sends HTTP requests.
type RequestService interface {
	// Send resolves {{variables}} in req from vars, applies req.Auth, builds
	// the body for req.Body.Mode, sends it and returns the response with its
	// duration measured from just before the request is written until the
	// body has been read. A transport failure (DNS, refused, timeout,
	// cancelled) is returned as an error; any HTTP status is a Response.
	Send(ctx context.Context, req model.Request, vars []model.KV) (model.Response, error)
}

// HistoryService keeps the most recently sent requests.
type HistoryService interface {
	// List returns entries newest first.
	List() []model.HistoryEntry
	// Add records an entry, assigning ID and At when empty, and drops the
	// oldest entries beyond the configured limit. It persists immediately.
	Add(e model.HistoryEntry) error
	// Delete removes one entry by ID.
	Delete(id string) error
	// Clear removes every entry.
	Clear() error
}

// CollectionService stores collections, each as one JSON file.
type CollectionService interface {
	// List returns every collection sorted by name.
	List() []model.Collection
	Get(id string) (model.Collection, bool)
	Create(name string) (model.Collection, error)
	// Save writes col, replacing the stored collection with the same ID.
	Save(col model.Collection) error
	Delete(id string) error
	// AddFolder adds an empty folder under parentID ("" = collection root).
	AddFolder(collectionID, parentID, name string) (model.Item, error)
	// SaveRequest stores req under parentID ("" = root). If an item with
	// req.ID already exists anywhere in the collection it is replaced in
	// place; otherwise a new item is appended (assigning an ID if empty).
	SaveRequest(collectionID, parentID string, req model.Request) (model.Request, error)
	// RenameItem renames a folder or request.
	RenameItem(collectionID, itemID, name string) error
	// DeleteItem removes a folder (with its contents) or a request.
	DeleteItem(collectionID, itemID string) error
	// Import stores an already-built collection (e.g. from importer),
	// assigning a fresh ID.
	Import(col model.Collection) (model.Collection, error)
}

// EnvironmentService stores environments, each as one JSON file.
type EnvironmentService interface {
	// List returns every environment sorted by name.
	List() []model.Environment
	Get(id string) (model.Environment, bool)
	Create(name string) (model.Environment, error)
	Save(env model.Environment) error
	Delete(id string) error
}

// SettingsService loads and saves the user's preferences.
type SettingsService interface {
	Get() model.Settings
	Save(s model.Settings) error
}

// Services bundles every service the GUI uses.
type Services struct {
	Requests     RequestService
	History      HistoryService
	Collections  CollectionService
	Environments EnvironmentService
	Settings     SettingsService
}
