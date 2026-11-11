// Package fake provides in-memory implementations of every service
// interface, for GUI tests and for running the app before the disk-backed
// services exist. They are not safe for concurrent use except Requests.
package fake

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/runbrick/reqo/internal/vars"
)

// New returns a full set of in-memory services.
func New() service.Services {
	return service.Services{
		Requests:     &Requests{},
		History:      &History{Limit: 200},
		Collections:  &Collections{},
		Environments: &Environments{},
		Settings:     &Settings{S: model.DefaultSettings()},
	}
}

// ErrNotFound is returned for unknown IDs.
var ErrNotFound = errors.New("not found")

// Requests answers every request without touching the network. If Fn is
// set it is called instead; otherwise it returns 200 with a small JSON body
// echoing the method and resolved URL.
type Requests struct {
	Fn func(ctx context.Context, req model.Request) (model.Response, error)
}

func (r *Requests) Send(ctx context.Context, req model.Request, vs []model.KV) (model.Response, error) {
	req = vars.Apply(req, vs)
	if r.Fn != nil {
		return r.Fn(ctx, req)
	}
	body := []byte(`{"method":"` + string(req.Method) + `","url":"` + req.URL + `"}`)
	return model.Response{
		StatusCode: 200, Status: "200 OK", Proto: "HTTP/1.1",
		Headers:  []model.KV{{Key: "Content-Type", Value: "application/json"}},
		Body:     body,
		Size:     int64(len(body)),
		Duration: 12 * time.Millisecond,
		URL:      req.URL,
	}, nil
}

// History keeps entries in memory, newest first.
type History struct {
	Entries []model.HistoryEntry
	Limit   int
}

func (h *History) List() []model.HistoryEntry { return slices.Clone(h.Entries) }

func (h *History) Add(e model.HistoryEntry) error {
	if e.ID == "" {
		e.ID = storage.NewID()
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	h.Entries = append([]model.HistoryEntry{e}, h.Entries...)
	if h.Limit > 0 && len(h.Entries) > h.Limit {
		h.Entries = h.Entries[:h.Limit]
	}
	return nil
}

func (h *History) Delete(id string) error {
	h.Entries = slices.DeleteFunc(h.Entries, func(e model.HistoryEntry) bool { return e.ID == id })
	return nil
}

func (h *History) Clear() error { h.Entries = nil; return nil }

// Collections keeps collections in memory.
type Collections struct {
	Cols []model.Collection
}

func (s *Collections) List() []model.Collection {
	out := slices.Clone(s.Cols)
	slices.SortFunc(out, func(a, b model.Collection) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (s *Collections) find(id string) *model.Collection {
	for i := range s.Cols {
		if s.Cols[i].ID == id {
			return &s.Cols[i]
		}
	}
	return nil
}

func (s *Collections) Get(id string) (model.Collection, bool) {
	if c := s.find(id); c != nil {
		return *c, true
	}
	return model.Collection{}, false
}

func (s *Collections) Create(name string) (model.Collection, error) {
	col := model.Collection{Version: model.SchemaVersion, ID: storage.NewID(), Name: name}
	s.Cols = append(s.Cols, col)
	return col, nil
}

func (s *Collections) Save(col model.Collection) error {
	if c := s.find(col.ID); c != nil {
		*c = col
		return nil
	}
	s.Cols = append(s.Cols, col)
	return nil
}

func (s *Collections) Delete(id string) error {
	s.Cols = slices.DeleteFunc(s.Cols, func(c model.Collection) bool { return c.ID == id })
	return nil
}

func (s *Collections) AddFolder(colID, parentID, name string) (model.Item, error) {
	c := s.find(colID)
	if c == nil {
		return model.Item{}, ErrNotFound
	}
	item := model.Item{Type: model.ItemFolder, ID: storage.NewID(), Name: name}
	items, ok := model.InsertItem(c.Items, parentID, item)
	if !ok {
		return model.Item{}, ErrNotFound
	}
	c.Items = items
	return item, nil
}

func (s *Collections) SaveRequest(colID, parentID string, req model.Request) (model.Request, error) {
	c := s.find(colID)
	if c == nil {
		return req, ErrNotFound
	}
	if req.ID != "" {
		if it := model.FindItem(c.Items, req.ID); it != nil && it.Type == model.ItemRequest {
			r := req.Clone()
			it.Request, it.Name = &r, r.Name
			return req, nil
		}
	} else {
		req.ID = storage.NewID()
	}
	r := req.Clone()
	items, ok := model.InsertItem(c.Items, parentID, model.Item{Type: model.ItemRequest, ID: req.ID, Name: req.Name, Request: &r})
	if !ok {
		return req, ErrNotFound
	}
	c.Items = items
	return req, nil
}

func (s *Collections) RenameItem(colID, itemID, name string) error {
	c := s.find(colID)
	if c == nil {
		return ErrNotFound
	}
	it := model.FindItem(c.Items, itemID)
	if it == nil {
		return ErrNotFound
	}
	it.Name = name
	if it.Request != nil {
		it.Request.Name = name
	}
	return nil
}

func (s *Collections) DeleteItem(colID, itemID string) error {
	c := s.find(colID)
	if c == nil {
		return ErrNotFound
	}
	items, ok := model.RemoveItem(c.Items, itemID)
	if !ok {
		return ErrNotFound
	}
	c.Items = items
	return nil
}

func (s *Collections) Import(col model.Collection) (model.Collection, error) {
	col.ID, col.Version = storage.NewID(), model.SchemaVersion
	s.Cols = append(s.Cols, col)
	return col, nil
}

// Environments keeps environments in memory.
type Environments struct {
	Envs []model.Environment
}

func (s *Environments) List() []model.Environment {
	out := slices.Clone(s.Envs)
	slices.SortFunc(out, func(a, b model.Environment) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (s *Environments) Get(id string) (model.Environment, bool) {
	for _, e := range s.Envs {
		if e.ID == id {
			return e, true
		}
	}
	return model.Environment{}, false
}

func (s *Environments) Create(name string) (model.Environment, error) {
	env := model.Environment{Version: model.SchemaVersion, ID: storage.NewID(), Name: name}
	s.Envs = append(s.Envs, env)
	return env, nil
}

func (s *Environments) Save(env model.Environment) error {
	for i := range s.Envs {
		if s.Envs[i].ID == env.ID {
			s.Envs[i] = env
			return nil
		}
	}
	s.Envs = append(s.Envs, env)
	return nil
}

func (s *Environments) Delete(id string) error {
	s.Envs = slices.DeleteFunc(s.Envs, func(e model.Environment) bool { return e.ID == id })
	return nil
}

// Settings keeps settings in memory.
type Settings struct{ S model.Settings }

func (s *Settings) Get() model.Settings         { return s.S }
func (s *Settings) Save(v model.Settings) error { s.S = v; return nil }
