package gui

import (
	"reflect"
	"slices"
	"strings"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/ui"
)

type collectionEnvironmentDraft struct {
	scope, saved *model.CollectionEnvironmentScope
	checks       map[string]*bool
	editedID     string
	table        kvTableState
}

func newCollectionEnvironmentDraft(scope *model.CollectionEnvironmentScope) collectionEnvironmentDraft {
	clone := scope.Clone()
	if clone == nil {
		clone = &model.CollectionEnvironmentScope{}
	}
	clone.ActiveID = ""
	return collectionEnvironmentDraft{scope: clone, saved: clone.Clone(), checks: make(map[string]*bool)}
}

func (d *collectionEnvironmentDraft) dirty() bool { return !reflect.DeepEqual(d.scope, d.saved) }

func (d *collectionEnvironmentDraft) valid() bool {
	for _, env := range d.scope.Local {
		if strings.TrimSpace(env.Name) == "" {
			return false
		}
	}
	return true
}

func (a *App) applyCollectionEnvironmentDraft(col model.Collection, draft *collectionEnvironmentDraft) model.Collection {
	active, _ := a.activeEnvironment(col)
	col.EnvironmentScope = draft.scope.Clone()
	for index := range col.EnvironmentScope.Local {
		env := &col.EnvironmentScope.Local[index]
		env.Name = strings.TrimSpace(env.Name)
	}
	if _, ok := a.environmentFor(col, active.ID); ok {
		col.EnvironmentScope.ActiveID = active.ID
	}
	return col
}

func (a *App) collectionEnvironments(c ui.C, d *collectionEnvironmentDraft) {
	s := a.S.Collections
	envs := a.availableEnvironments(model.Collection{EnvironmentScope: d.scope})
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		ui.Wrap(c, s.LocalEnvironments).Bold().Grow(1).MinWidth(0)
		if ui.Button(c, s.NewLocalEnvironment).Small().Outline().Icon(icons.Plus).Clicked() {
			c.After(func() { d.addLocal(a.S.Env.NewEnvironment) })
		}
	})
	for index := range d.scope.Local {
		env := &d.scope.Local[index]
		ui.Row(c.Key("local:" + env.ID)).FillWidth().Items(ui.Center).Gap(Space2).Children(func() {
			a.collectionEnvironmentCheckbox(c, d, *env, envs)
			edit, icon := a.S.Env.Edit, icons.Pencil
			if d.editedID == env.ID {
				edit, icon = a.S.Env.Close, icons.X
			}
			if ui.Button(c, edit).Small().Ghost().Icon(icon).IconOnly().Tooltip(edit).Clicked() {
				c.After(func() { d.toggleEditor(env.ID) })
			}
		})
		if d.editedID == env.ID {
			a.collectionLocalEnvironmentEditor(c.Key("editor:"+env.ID), d, env)
		}
	}
	ui.Divider(c)
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		ui.Wrap(c, s.SharedEnvironments).Bold().Grow(1).MinWidth(0)
		if ui.Button(c, a.S.Env.Manage).Small().Ghost().Icon(icons.Globe).Clicked() {
			c.After(a.openEnvironments)
		}
	})
	for _, env := range envs {
		if _, local := d.scope.FindLocal(env.ID); !local {
			a.collectionEnvironmentCheckbox(c.Key("shared:"+env.ID), d, env, envs)
		}
	}
}

func (a *App) collectionEnvironmentCheckbox(c ui.C, d *collectionEnvironmentDraft, env model.Environment, envs []model.Environment) {
	checked := d.checks[env.ID]
	if checked == nil {
		checked = new(bool)
		d.checks[env.ID] = checked
	}
	*checked = d.scope.Shows(env.ID)
	name := env.Name
	if strings.TrimSpace(name) == "" {
		name = a.S.Env.NewEnvironment
	}
	if !ui.Checkbox(c, checked, name).Label(name).MinWidth(0).Changed() {
		return
	}
	if d.scope.VisibleIDs == nil {
		d.scope.VisibleIDs = make([]string, 0, len(envs))
		for _, candidate := range envs {
			if candidate.ID != "" {
				d.scope.VisibleIDs = append(d.scope.VisibleIDs, candidate.ID)
			}
		}
	}
	if *checked {
		d.scope.VisibleIDs = append(d.scope.VisibleIDs, env.ID)
	} else {
		d.scope.VisibleIDs = slices.DeleteFunc(d.scope.VisibleIDs, func(id string) bool { return id == env.ID })
	}
	slices.Sort(d.scope.VisibleIDs)
}

func (a *App) collectionLocalEnvironmentEditor(c ui.C, d *collectionEnvironmentDraft, env *model.Environment) {
	ui.Column(c).FillWidth().Gap(Space2).PadLeft(Space4).Children(func() {
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			ui.TextField(c, &env.Name, a.S.Env.Name, a.S.Env.NewEnvironment).Grow(1).MinWidth(0)
			if ui.Button(c, a.S.Env.Delete).Small().Icon(icons.Trash2).IconOnly().Ghost().Tooltip(a.S.Env.Delete).Clicked() {
				c.After(func() { d.deleteLocal(env.ID) })
			}
		})
		a.kvTable(c, &d.table, &env.Variables)
	})
}

func (d *collectionEnvironmentDraft) addLocal(name string) {
	id := storage.NewID()
	d.scope.Local = append(d.scope.Local, model.Environment{Version: model.SchemaVersion, ID: id, Name: name})
	if d.scope.VisibleIDs != nil {
		d.scope.VisibleIDs = append(d.scope.VisibleIDs, id)
		slices.Sort(d.scope.VisibleIDs)
	}
	d.toggleEditor(id)
}

func (d *collectionEnvironmentDraft) toggleEditor(id string) {
	if d.editedID == id {
		id = ""
	}
	d.editedID, d.table = id, kvTableState{}
}

func (d *collectionEnvironmentDraft) deleteLocal(id string) {
	d.scope.Local = slices.DeleteFunc(d.scope.Local, func(env model.Environment) bool { return env.ID == id })
	if len(d.scope.Local) == 0 {
		d.scope.Local = nil
	}
	d.scope.VisibleIDs = slices.DeleteFunc(d.scope.VisibleIDs, func(candidate string) bool { return candidate == id })
	delete(d.checks, id)
	d.toggleEditor(id)
}
