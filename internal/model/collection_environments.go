package model

import "slices"

type CollectionEnvironmentScope struct {
	VisibleIDs []string      `json:"visibleIds,omitzero"`
	Local      []Environment `json:"local,omitempty"`
	ActiveID   string        `json:"activeId,omitempty"`
}

func (scope *CollectionEnvironmentScope) Clone() *CollectionEnvironmentScope {
	if scope == nil {
		return nil
	}
	copy := *scope
	copy.VisibleIDs = slices.Clone(scope.VisibleIDs)
	copy.Local = slices.Clone(scope.Local)
	for index := range copy.Local {
		copy.Local[index].Variables = slices.Clone(copy.Local[index].Variables)
	}
	return &copy
}

func (scope *CollectionEnvironmentScope) Shows(id string) bool {
	return id != "" && (scope == nil || scope.VisibleIDs == nil || slices.Contains(scope.VisibleIDs, id))
}

func (scope *CollectionEnvironmentScope) FindLocal(id string) (Environment, bool) {
	if scope != nil && id != "" {
		for _, env := range scope.Local {
			if env.ID == id {
				return env, true
			}
		}
	}
	return Environment{}, false
}
