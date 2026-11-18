package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
	"github.com/runbrick/reva/internal/vars"
	"github.com/vivaui/viva/ui"
)

// envState owns the titlebar picker, sidebar menus and independent editor drafts.
type envState struct {
	picker     bool
	activeID   string
	drafts     []*envDraft
	selected   string
	rename     bool
	deleteOpen bool
	deleteID   string
	err        string
}

type envDraft struct {
	id, name  string
	variables []model.KV
	saved     model.Environment
	table     kvTableState
	menu      bool
}

// Vars returns send variables in order of increasing precedence. Disabled
// environment variables never shadow collection variables.
func (a *App) Vars() []model.KV {
	return a.VarsFor(a.Active())
}

// VarsFor uses the passed tab's collection, including when another tab is active.
// Pre-request script variables are appended by prepareSend at send start.
func (a *App) VarsFor(t *Tab) []model.KV {
	col, _ := a.Svc.Collections.Get(t.CollectionID)
	out := slices.Clone(col.Variables)
	if env, ok := a.activeEnvironment(col); ok {
		for _, kv := range env.Variables {
			if !kv.Disabled {
				out = append(out, kv)
			}
		}
	}
	return out
}

func (a *App) environmentCollection() model.Collection {
	if a.Main == PageWorkbench && a.Section != SectionEnvironments {
		col, _ := a.Svc.Collections.Get(a.Active().CollectionID)
		return col
	}
	return model.Collection{}
}

func (a *App) environmentFor(col model.Collection, id string) (model.Environment, bool) {
	if !col.EnvironmentScope.Shows(id) {
		return model.Environment{}, false
	}
	if env, ok := col.EnvironmentScope.FindLocal(id); ok {
		return env, true
	}
	return a.Svc.Environments.Get(id)
}

func (a *App) activeEnvironment(col model.Collection) (model.Environment, bool) {
	if col.EnvironmentScope != nil {
		return a.environmentFor(col, col.EnvironmentScope.ActiveID)
	}
	return a.environmentFor(col, a.Svc.Settings.Get().ActiveEnvID)
}

func (a *App) availableEnvironments(col model.Collection) []model.Environment {
	envs := a.Svc.Environments.List()
	if scope := col.EnvironmentScope; scope != nil {
		envs = slices.DeleteFunc(slices.Clone(envs), func(env model.Environment) bool {
			_, local := scope.FindLocal(env.ID)
			return local
		})
		envs = append(envs, scope.Local...)
	}
	return envs
}

func (a *App) visibleEnvironments(col model.Collection) []model.Environment {
	return slices.DeleteFunc(a.availableEnvironments(col), func(env model.Environment) bool {
		return !col.EnvironmentScope.Shows(env.ID)
	})
}

// UnresolvedVars lists distinct missing names in URL then enabled-header order.
func (a *App) UnresolvedVars() []string {
	req, vs := &a.Active().Request, a.Vars()
	out := vars.Unresolved(req.URL, vs)
	for _, h := range req.Headers {
		if h.Disabled {
			continue
		}
		for _, name := range vars.Unresolved(h.Value, vs) {
			if !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	}
	if cfg, _, err := a.effectiveAuth(a.Active()); err == nil && !isWebSocket(*req) {
		var fields [2]string
		switch cfg.Type {
		case model.AuthBearer:
			fields[0] = cfg.Token
		case model.AuthBasic:
			fields = [2]string{cfg.Username, cfg.Password}
		case model.AuthAPIKey:
			fields = [2]string{cfg.Key, cfg.Value}
		}
		for _, field := range fields {
			for _, name := range vars.Unresolved(field, vs) {
				if !slices.Contains(out, name) {
					out = append(out, name)
				}
			}
		}
	}
	return out
}

// envSelector is a compact titlebar menu with an action after the choices.
func (a *App) envSelector(c ui.C) {
	s, theme := &a.env, c.Theme()
	col := a.environmentCollection()
	active, hasActive := a.activeEnvironment(col)
	s.activeID = active.ID
	previewing := a.requestPreviewOpen()
	name := a.S.Env.NoEnvironment
	if hasActive {
		name = active.Name
		if previewing {
			name = a.requestPreviewEnvironmentName(active)
		}
	}
	trigger := ui.ButtonBase(c, a.S.Env.Title).Height(Space4 * 2).MaxWidth(Space6 * 10).
		PadX(Space3).Gap(Space2).Radius(theme.Metrics.RadiusControl).
		Background(a.palette.Rail).Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(Space1/2, theme.Ring)).Children(func() {
		ui.Icon(c, icons.Globe).Size(Space4, Space4).Color(theme.MutedForeground)
		ui.Text(c, name).Ellipsis()
		ui.Icon(c, icons.ChevronDown).Size(Space4, Space4).Color(theme.MutedForeground)
	})
	if trigger.Clicked() {
		s.picker = !s.picker
	}
	ui.Menu(c, trigger, &s.picker, func(menu *ui.MenuItems) {
		if menu.Item(a.S.Env.NoEnvironment).Chosen(s.activeID == "").Clicked() {
			c.After(func() { a.selectEnvironment(col.ID, "") })
		}
		for _, env := range a.visibleEnvironments(col) {
			name := env.Name
			if previewing {
				name = a.requestPreviewEnvironmentName(env)
			}
			if menu.Item(name).Chosen(env.ID == s.activeID).Clicked() {
				c.After(func() { a.selectEnvironment(col.ID, env.ID) })
			}
		}
		menu.Separator()
		if col.ID != "" && menu.Item(a.S.Collections.Environments).Icon(icons.Settings).Clicked() {
			c.After(func() { a.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsEnvironments) })
		}
		if menu.Item(a.S.Env.Manage).Icon(icons.Globe).Clicked() {
			c.After(a.openEnvironments)
		}
	})
	missing := a.UnresolvedVars()
	if previewing {
		missing = a.requestPreviewSnapshot(a.Active()).Unresolved
	}
	if len(missing) > 0 {
		label := fmt.Sprintf(a.S.Env.Unresolved, len(missing))
		ui.Row(c).Size(Space2, Space2).Radius(Space1).Background(a.palette.Warning).
			Hit(ui.ID(c), label).Tooltip(a.S.Env.UnresolvedTip + strings.Join(missing, ", "))
	}
}

func (a *App) setActiveEnvironment(id string) {
	a.selectEnvironment(a.environmentCollection().ID, id)
}

func (a *App) selectEnvironment(collectionID, id string) {
	if collectionID != "" {
		col, ok := a.Svc.Collections.Get(collectionID)
		if !ok {
			return
		}
		if id != "" {
			if _, ok := a.environmentFor(col, id); !ok {
				return
			}
		}
		col.EnvironmentScope = col.EnvironmentScope.Clone()
		if col.EnvironmentScope == nil {
			col.EnvironmentScope = &model.CollectionEnvironmentScope{}
		}
		col.EnvironmentScope.ActiveID = id
		if err := a.Svc.Collections.Save(col); err != nil {
			a.Toast(a.S.Env.Error, "", ui.ToastError)
		}
		return
	}
	settings := a.Svc.Settings.Get()
	settings.ActiveEnvID = id
	if err := a.Svc.Settings.Save(settings); err != nil {
		a.Toast(a.S.Env.Error, err.Error(), ui.ToastError)
	}
}

func newEnvDraft(env model.Environment) *envDraft {
	env.Variables = slices.Clone(env.Variables)
	return &envDraft{id: env.ID, name: env.Name, variables: slices.Clone(env.Variables), saved: env}
}

func (d *envDraft) dirty() bool {
	return d.name != d.saved.Name || !slices.Equal(d.variables, d.saved.Variables)
}

func (d *envDraft) environment() model.Environment {
	env := d.saved
	env.Version, env.ID, env.Name = model.SchemaVersion, d.id, strings.TrimSpace(d.name)
	env.Variables = slices.Clone(d.variables)
	return env
}

func (s *envState) current() *envDraft {
	for _, d := range s.drafts {
		if d.id == s.selected {
			return d
		}
	}
	return nil
}

// syncEnvironments keeps unsaved drafts when the user changes section or row.
func (a *App) syncEnvironments() {
	s := &a.env
	envs := a.Svc.Environments.List()
	s.drafts = slices.DeleteFunc(s.drafts, func(d *envDraft) bool {
		return !slices.ContainsFunc(envs, func(e model.Environment) bool { return e.ID == d.id })
	})
	for _, env := range envs {
		if !slices.ContainsFunc(s.drafts, func(d *envDraft) bool { return d.id == env.ID }) {
			s.drafts = append(s.drafts, newEnvDraft(env))
		}
	}
	if s.current() == nil {
		s.selected = a.Svc.Settings.Get().ActiveEnvID
		if s.current() == nil && len(envs) > 0 {
			s.selected = envs[0].ID
		}
	}
}

func (a *App) openEnvironments() {
	a.SelectSection(SectionEnvironments)
	a.syncEnvironments()
}

func (a *App) createEnvironment() {
	env, err := a.Svc.Environments.Create(a.S.Env.NewEnvironment)
	if err != nil {
		a.envError(err)
		return
	}
	a.env.drafts = append(a.env.drafts, newEnvDraft(env))
	a.env.selected, a.env.rename, a.env.err = env.ID, true, ""
}

func (a *App) envError(err error) {
	a.env.err = a.S.Env.Error + ": " + err.Error()
	a.Toast(a.S.Env.Error, err.Error(), ui.ToastError)
}

func (a *App) envSidebar(c ui.C) {
	a.syncEnvironments()
	s, theme := &a.env, c.Theme()
	ui.Column(c).Fill().Pad(Space3).Gap(Space3).Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Children(func() {
			ui.Text(c, a.S.Env.Title).Bold().Grow(1)
			if ui.Button(c, a.S.Env.New).Icon(icons.Plus).IconOnly().Ghost().Tooltip(a.S.Env.New).Clicked() {
				c.After(a.createEnvironment)
			}
		})
		ui.Scroll(c).FillWidth().Grow(1).Gap(Space1).Children(func() {
			for _, env := range a.Svc.Environments.List() {
				var draft *envDraft
				for _, d := range s.drafts {
					if d.id == env.ID {
						draft = d
						break
					}
				}
				if draft == nil {
					continue
				}
				ui.Row(c.Key(env.ID)).FillWidth().Gap(Space1).Items(ui.Center).Children(func() {
					row := ui.ButtonBase(c, env.Name).Grow(1).Height(theme.Metrics.ControlHeight).
						PadX(Space2).Gap(Space2).Justify(ui.Start).Radius(theme.Metrics.RadiusControl).
						Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(Space1/2, theme.Ring))
					if s.selected == env.ID {
						row.Background(a.palette.RailActive)
					}
					row.Children(func() {
						ui.Icon(c, icons.Globe).Size(Space4, Space4).Color(theme.MutedForeground)
						ui.Text(c, env.Name).Grow(1).Ellipsis()
						if draft.dirty() {
							ui.Text(c, a.S.Env.UnsavedMark).Color(a.palette.Warning)
						}
						if a.Svc.Settings.Get().ActiveEnvID == env.ID {
							ui.Icon(c, icons.Check).Size(Space4, Space4).Color(theme.Primary)
						}
					})
					if row.Clicked() {
						s.selected, s.rename, s.err = env.ID, false, ""
					}
					more := ui.Button(c, a.S.Env.Actions).Icon(icons.Ellipsis).IconOnly().Ghost().Tooltip(a.S.Env.Actions)
					if more.Clicked() {
						draft.menu = !draft.menu
					}
					actions := func(menu *ui.MenuItems) { a.envActions(c, menu, draft) }
					ui.Menu(c, more, &draft.menu, actions)
					ui.ContextMenu(c, row, actions)
				})
			}
		})
	})
}

func (a *App) envActions(c ui.C, menu *ui.MenuItems, d *envDraft) {
	s := &a.env
	if menu.Item(a.S.Env.SetActive).Icon(icons.Check).Disabled(a.Svc.Settings.Get().ActiveEnvID == d.id).Clicked() {
		c.After(func() { a.selectEnvironment("", d.id) })
	}
	menu.Separator()
	if menu.Item(a.S.Env.Rename).Clicked() {
		s.selected, s.rename, s.err = d.id, true, ""
	}
	if menu.Item(a.S.Env.Duplicate).Clicked() {
		c.After(func() {
			env := d.environment()
			env.ID, env.Name = storage.NewID(), fmt.Sprintf(a.S.Env.CopyName, env.Name)
			if err := a.Svc.Environments.Save(env); err != nil {
				a.envError(err)
				return
			}
			s.drafts = append(s.drafts, newEnvDraft(env))
			s.selected, s.rename, s.err = env.ID, true, ""
		})
	}
	menu.Separator()
	if menu.Item(a.S.Env.Delete).Destructive().Clicked() {
		s.deleteID, s.deleteOpen, s.err = d.id, true, ""
	}
}

// envDialog now contains only the destructive-action confirmation; editing is a page.
func (a *App) envDialog(c ui.C) {
	s := &a.env
	if !s.deleteOpen {
		return
	}
	env, ok := a.Svc.Environments.Get(s.deleteID)
	if !ok {
		s.deleteOpen = false
		return
	}
	ui.Modal(c, &s.deleteOpen, a.S.Env.Delete, func() {
		ui.Wrap(c, fmt.Sprintf(a.S.Env.DeleteConfirm, env.Name))
		if s.err != "" {
			ui.Wrap(c, s.err).Color(c.Theme().Danger)
		}
	}, func() {
		if ui.Button(c, a.S.Env.Cancel).Outline().Clicked() {
			s.deleteOpen = false
		}
		if ui.Button(c, a.S.Env.Delete).Destructive().Clicked() {
			c.After(func() {
				settings := a.Svc.Settings.Get()
				if settings.ActiveEnvID == env.ID {
					next := settings
					next.ActiveEnvID = ""
					if err := a.Svc.Settings.Save(next); err != nil {
						a.envError(err)
						return
					}
				}
				if err := a.Svc.Environments.Delete(env.ID); err != nil {
					if a.Svc.Settings.Get() != settings {
						if restoreErr := a.Svc.Settings.Save(settings); restoreErr != nil {
							a.envError(restoreErr)
						}
					}
					a.envError(err)
					return
				}
				s.deleteOpen, s.err = false, ""
				a.syncEnvironments()
			})
		}
	})
}

func (a *App) envReady() bool { return true }

func (a *App) envMain(c ui.C) {
	a.syncEnvironments()
	s, theme := &a.env, c.Theme()
	ui.Column(c).Fill().Pad(Space6).Gap(Space4).Children(func() {
		d := s.current()
		if d == nil {
			ui.Column(c).Fill().Center().Gap(Space4).Children(func() {
				ui.Icon(c, icons.Globe).Size(Space6*2, Space6*2).Color(theme.MutedForeground)
				ui.Text(c, a.S.Env.Title).Bold().FontSize(theme.Metrics.FontSizeL)
				ui.Wrap(c, a.S.Env.Empty).Muted()
				if ui.Button(c, a.S.Env.New).Icon(icons.Plus).Clicked() {
					c.After(a.createEnvironment)
				}
			})
			if s.err != "" {
				ui.Wrap(c, s.err).Color(theme.Danger)
			}
			return
		}
		ui.Column(c.Key(d.id)).Fill().Gap(Space4).Children(func() {
			// Synchronize fields before dirty indicators and shortcut save see the draft.
			d.table.sync(d.variables)
			ui.Row(c).FillWidth().Gap(Space3).Items(ui.Center).Children(func() {
				ui.Text(c, a.S.Env.Title).Bold().FontSize(theme.Metrics.FontSizeL)
				if d.dirty() {
					ui.Text(c, a.S.Env.Unsaved).FontSize(theme.Metrics.FontSizeS).Color(a.palette.Warning)
				}
				ui.Spacer(c)
				if ui.Button(c, a.S.Env.Discard).Outline().Disabled(!d.dirty()).Clicked() {
					c.After(func() {
						if env, ok := a.Svc.Environments.Get(d.id); ok {
							*d = *newEnvDraft(env)
						}
						s.err, s.rename = "", false
					})
				}
				if ui.Button(c, a.S.Env.Save).Icon(icons.Check).Disabled(!d.dirty()).Clicked() {
					c.After(a.saveEnvironments)
				}
			})
			name := ui.TextField(c, &d.name, a.S.Env.Name, a.S.Env.NewEnvironment).FillWidth()
			if s.rename {
				name.AutoFocus()
			}
			ui.Wrap(c, a.S.Env.SaveHint).Muted().FontSize(theme.Metrics.FontSizeS)
			ui.Scroll(c).FillWidth().Grow(1).Gap(Space3).Children(func() {
				a.kvTable(c, &d.table, &d.variables)
			})
			if s.err != "" {
				ui.Wrap(c, s.err).Color(theme.Danger)
			}
			if ui.Shortcut(c, ui.Primary, "s") {
				c.After(a.saveEnvironments)
			}
		})
	})
}

func (a *App) saveEnvironments() {
	s := &a.env
	d := s.current()
	if d == nil {
		return
	}
	d.table.sync(d.variables)
	if strings.TrimSpace(d.name) == "" {
		s.err, s.rename = a.S.Env.NameRequired, true
		return
	}
	env := d.environment()
	if err := a.Svc.Environments.Save(env); err != nil {
		a.envError(err)
		return
	}
	d.name, d.saved, s.err, s.rename = env.Name, env, "", false
}
