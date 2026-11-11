package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
)

type collectionService struct{ s *Store }

func (c collectionService) List() []model.Collection {
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	cols := make([]model.Collection, 0, len(c.s.collections))
	for _, col := range c.s.collections {
		cols = append(cols, cloneCollection(col))
	}
	slices.SortFunc(cols, func(a, b model.Collection) int {
		if n := strings.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		return strings.Compare(a.ID, b.ID)
	})
	return cols
}

func (c collectionService) Get(id string) (model.Collection, bool) {
	c.s.mu.RLock()
	defer c.s.mu.RUnlock()
	col, ok := c.s.collections[id]
	return cloneCollection(col), ok
}

func (c collectionService) Create(name string) (model.Collection, error) {
	col := model.Collection{Version: model.SchemaVersion, ID: storage.NewID(), Name: name, Items: []model.Item{}}
	return col, c.Save(col)
}

func (c collectionService) Save(col model.Collection) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	return c.s.saveCollection(cloneCollection(col))
}

func (s *Store) saveCollection(col model.Collection) error {
	if err := validateCollection(col); err != nil {
		return err
	}
	col.Version = model.SchemaVersion
	if err := storage.WriteJSON(filepath.Join(s.dir, "collections", col.ID+".json"), col); err != nil {
		return err
	}
	s.collections[col.ID] = col
	return nil
}

func validateCollection(col model.Collection) error {
	if err := validateID(col.ID); err != nil {
		return err
	}
	if err := validateName(col.Name); err != nil {
		return err
	}
	var validateItems func([]model.Item) error
	validateItems = func(items []model.Item) error {
		for _, item := range items {
			if err := validateName(item.Name); err != nil {
				return err
			}
			if item.Request != nil {
				if err := validateName(item.Request.Name); err != nil {
					return err
				}
			}
			if err := validateItems(item.Items); err != nil {
				return err
			}
		}
		return nil
	}
	return validateItems(col.Items)
}

func (c collectionService) Delete(id string) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	if err := validateID(id); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(c.s.dir, "collections", id+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(c.s.collections, id)
	return nil
}

func (c collectionService) AddFolder(collectionID, parentID, name string) (model.Item, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	col, ok := c.s.collections[collectionID]
	if !ok {
		return model.Item{}, ErrNotFound
	}
	if err := validateName(name); err != nil {
		return model.Item{}, err
	}
	col = cloneCollection(col)
	item := model.Item{Type: model.ItemFolder, ID: storage.NewID(), Name: name}
	col.Items, ok = model.InsertItem(col.Items, parentID, item)
	if !ok {
		return model.Item{}, ErrNotFound
	}
	return item, c.s.saveCollection(col)
}

func (c collectionService) SaveRequest(collectionID, parentID string, req model.Request) (model.Request, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	col, ok := c.s.collections[collectionID]
	if !ok {
		return req, ErrNotFound
	}
	if err := validateName(req.Name); err != nil {
		return req, err
	}
	col = cloneCollection(col)
	req = req.Clone()
	if req.ID == "" {
		req.ID = storage.NewID()
	}
	if item := model.FindItem(col.Items, req.ID); item != nil {
		if item.Type != model.ItemRequest {
			return req, errors.New("request ID belongs to a folder")
		}
		item.Name, item.Request = req.Name, &req
	} else {
		col.Items, ok = model.InsertItem(col.Items, parentID, model.Item{
			Type: model.ItemRequest, ID: req.ID, Name: req.Name, Request: &req,
		})
		if !ok {
			return req, ErrNotFound
		}
	}
	// The returned request must not share its slices with the stored tree.
	return req.Clone(), c.s.saveCollection(col)
}

func (c collectionService) RenameItem(collectionID, itemID, name string) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	if err := validateName(name); err != nil {
		return err
	}
	col, ok := c.s.collections[collectionID]
	if !ok {
		return ErrNotFound
	}
	col = cloneCollection(col)
	item := model.FindItem(col.Items, itemID)
	if item == nil {
		return ErrNotFound
	}
	item.Name = name
	if item.Request != nil {
		item.Request.Name = name
	}
	return c.s.saveCollection(col)
}

func (c collectionService) DeleteItem(collectionID, itemID string) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	col, ok := c.s.collections[collectionID]
	if !ok {
		return ErrNotFound
	}
	col = cloneCollection(col)
	col.Items, ok = model.RemoveItem(col.Items, itemID)
	if !ok {
		return ErrNotFound
	}
	return c.s.saveCollection(col)
}

func (c collectionService) Import(col model.Collection) (model.Collection, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	col = cloneCollection(col)
	col.ID, col.Version = storage.NewID(), model.SchemaVersion
	var reID func([]model.Item)
	reID = func(items []model.Item) {
		for i := range items {
			items[i].ID = storage.NewID()
			if items[i].Request != nil {
				// Request items use the same ID as their request, so SaveRequest
				// can find and replace an imported request in place.
				items[i].Request.ID = items[i].ID
			}
			reID(items[i].Items)
		}
	}
	reID(col.Items)
	return cloneCollection(col), c.s.saveCollection(col)
}
