package store

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
)

type environmentService struct{ s *Store }

func (e environmentService) List() []model.Environment {
	e.s.mu.RLock()
	defer e.s.mu.RUnlock()
	envs := make([]model.Environment, 0, len(e.s.environments))
	for _, env := range e.s.environments {
		envs = append(envs, cloneEnvironment(env))
	}
	slices.SortFunc(envs, func(a, b model.Environment) int {
		if n := strings.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		return strings.Compare(a.ID, b.ID)
	})
	return envs
}

func (e environmentService) Get(id string) (model.Environment, bool) {
	e.s.mu.RLock()
	defer e.s.mu.RUnlock()
	env, ok := e.s.environments[id]
	return cloneEnvironment(env), ok
}

func (e environmentService) Create(name string) (model.Environment, error) {
	env := model.Environment{Version: model.SchemaVersion, ID: storage.NewID(), Name: name, Variables: []model.KV{}}
	return env, e.Save(env)
}

func (e environmentService) Save(env model.Environment) error {
	e.s.mu.Lock()
	defer e.s.mu.Unlock()
	if err := validateID(env.ID); err != nil {
		return err
	}
	if err := validateName(env.Name); err != nil {
		return err
	}
	env = cloneEnvironment(env)
	env.Version = model.SchemaVersion
	if err := storage.WriteJSON(filepath.Join(e.s.dir, "environments", env.ID+".json"), env); err != nil {
		return err
	}
	e.s.environments[env.ID] = env
	return nil
}

func (e environmentService) Delete(id string) error {
	e.s.mu.Lock()
	defer e.s.mu.Unlock()
	if err := validateID(id); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(e.s.dir, "environments", id+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(e.s.environments, id)
	return nil
}
