package gui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/runbrick/reqo/internal/vars"
	"github.com/vivaui/viva/ui"
)

// envState is the environment selector's and editor's UI state. Owned by
// the environments PR.
type envState struct {
	open     bool
	activeID string
	choices  []string
	drafts   []*envDraft
	selected string
	deleted  []string
	rename   bool
	err      string
}

// Drafts own their rows, so typing never changes a service's slice before Save.
// Stable row IDs preserve focus when an earlier variable is removed.
type envDraft struct {
	id, name  string
	persisted bool
	rows      []*envRow
}

type envRow struct {
	id      string
	enabled bool
	kv      model.KV
}

// Vars returns the variables used for {{name}} substitution in the next
// send, lowest precedence first (collection variables, then the active
// environment).
func (a *App) Vars() []model.KV {
	var out []model.KV
	if col, ok := a.Svc.Collections.Get(a.Active().CollectionID); ok {
		out = append(out, col.Variables...)
	}
	if env, ok := a.Svc.Environments.Get(a.Svc.Settings.Get().ActiveEnvID); ok {
		for _, kv := range env.Variables {
			if !kv.Disabled {
				out = append(out, kv)
			}
		}
	}
	return out
}

// UnresolvedVars returns distinct missing names, in URL then header order.
// Disabled headers are not sent and therefore do not contribute warnings.
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
	return out
}

// envSelector declares the active-environment picker in the top bar.
func (a *App) envSelector(c ui.C) {
	s := &a.env
	envs := a.Svc.Environments.List()
	s.choices = append(s.choices[:0], "")
	for _, env := range envs {
		s.choices = append(s.choices, env.ID)
	}
	s.activeID = a.Svc.Settings.Get().ActiveEnvID
	if !slices.Contains(s.choices, s.activeID) {
		s.activeID = ""
	}
	label := func(id string) string {
		for _, env := range envs {
			if env.ID == id {
				return env.Name
			}
		}
		return a.S.Env.NoEnvironment
	}
	if ui.Select(c, &s.activeID, s.choices, label).Label(a.S.Env.Title).Width(200).Changed() {
		settings := a.Svc.Settings.Get()
		settings.ActiveEnvID = s.activeID
		if err := a.Svc.Settings.Save(settings); err != nil {
			a.Toast(a.S.Env.Error, err.Error(), ui.ToastError)
		}
	}
	if ui.Button(c, a.S.Env.Edit).Outline().Clicked() {
		c.After(a.openEnvironments)
	}
	if missing := a.UnresolvedVars(); len(missing) > 0 {
		theme := c.Theme()
		label := fmt.Sprintf(a.S.Env.Unresolved, len(missing))
		ui.Text(c, label).FontSize(theme.Metrics.FontSizeS).
			Color(theme.Danger).Background(theme.Danger.WithAlpha(0.12)).
			PadX(8).PadY(4).Radius(theme.Metrics.RadiusControl).
			Hit(ui.ID(c), label).Tooltip(a.S.Env.UnresolvedTip + strings.Join(missing, ", "))
	}
}

func (a *App) openEnvironments() {
	s := &a.env
	s.drafts, s.deleted, s.err, s.rename = nil, nil, "", false
	s.selected = a.Svc.Settings.Get().ActiveEnvID
	for _, env := range a.Svc.Environments.List() {
		s.drafts = append(s.drafts, newEnvDraft(env, true))
	}
	if s.current() == nil && len(s.drafts) > 0 {
		s.selected = s.drafts[0].id
	}
	s.open = true
}

func newEnvDraft(env model.Environment, persisted bool) *envDraft {
	d := &envDraft{id: env.ID, name: env.Name, persisted: persisted}
	for _, kv := range env.Variables {
		d.rows = append(d.rows, &envRow{id: storage.NewID(), enabled: !kv.Disabled, kv: kv})
	}
	return d
}

func (s *envState) current() *envDraft {
	for _, d := range s.drafts {
		if d.id == s.selected {
			return d
		}
	}
	return nil
}

func (d *envDraft) environment() model.Environment {
	env := model.Environment{Version: model.SchemaVersion, ID: d.id, Name: strings.TrimSpace(d.name)}
	for _, row := range d.rows {
		kv := row.kv
		kv.Disabled = !row.enabled
		env.Variables = append(env.Variables, kv)
	}
	return env
}

// envDialog stages all edits (including New, Duplicate and Delete) locally.
// Only Save persists them; every other close discards the entire draft session.
func (a *App) envDialog(c ui.C) {
	s := &a.env
	if !s.open {
		return
	}
	ui.ModalWith(c, &s.open, a.S.Env.Title, ui.ModalSpec{Width: 960, Height: 560}, func() {
		ui.Column(c).Fill().Gap(8).Children(func() {
			ui.Wrap(c, a.S.Env.SaveHint).Muted()
			ui.Row(c).FillWidth().Grow(1).Gap(16).Children(func() {
				ui.Column(c).Width(200).FillHeight().Gap(8).Children(func() {
					if ui.Button(c, a.S.Env.New).FillWidth().Clicked() {
						c.After(func() {
							d := newEnvDraft(model.Environment{ID: storage.NewID(), Name: a.S.Env.NewEnvironment}, false)
							s.drafts = append(s.drafts, d)
							s.selected, s.rename = d.id, true
						})
					}
					ui.Scroll(c).FillWidth().Grow(1).Gap(4).Children(func() {
						for _, d := range s.drafts {
							button := ui.Button(c.Key(d.id), d.name).FillWidth()
							if d.id == s.selected {
								button.Secondary()
							} else {
								button.Ghost()
							}
							if button.Clicked() {
								s.selected, s.rename = d.id, false
							}
						}
					})
					if d := s.current(); d != nil {
						if ui.Button(c, a.S.Env.Rename).Outline().FillWidth().Clicked() {
							s.rename = true
						}
						if ui.Button(c, a.S.Env.Duplicate).Outline().FillWidth().Clicked() {
							c.After(func() {
								env := d.environment()
								env.ID, env.Name = storage.NewID(), fmt.Sprintf(a.S.Env.CopyName, env.Name)
								copy := newEnvDraft(env, false)
								s.drafts = append(s.drafts, copy)
								s.selected, s.rename = copy.id, true
							})
						}
						if ui.Button(c, a.S.Env.Delete).DestructiveOutline().FillWidth().Clicked() {
							c.After(func() {
								if d.persisted {
									s.deleted = append(s.deleted, d.id)
								}
								s.drafts = slices.DeleteFunc(s.drafts, func(item *envDraft) bool { return item.id == d.id })
								s.selected, s.rename = "", false
								if len(s.drafts) > 0 {
									s.selected = s.drafts[0].id
								}
							})
						}
					}
				})
				ui.Column(c).Grow(1).FillHeight().Gap(8).Children(func() {
					d := s.current()
					if d == nil {
						ui.Wrap(c, a.S.Env.Empty).Muted()
						return
					}
					ui.Column(c.Key(d.id)).Fill().Gap(8).Children(func() {
						if s.rename {
							ui.TextField(c, &d.name, a.S.Env.Name, a.S.Env.NewEnvironment).FillWidth().AutoFocus()
						} else {
							ui.Text(c, d.name).Bold()
						}
						ui.Row(c).FillWidth().Gap(8).Children(func() {
							ui.Text(c, a.S.Env.Enabled).Width(88)
							ui.Text(c, a.S.Env.Key).Grow(1)
							ui.Text(c, a.S.Env.Value).Grow(1)
							ui.Spacer(c).Width(100)
						})
						ui.Scroll(c).FillWidth().Grow(1).Gap(8).Children(func() {
							for _, row := range d.rows {
								ui.Row(c.Key(row.id)).FillWidth().Gap(8).Items(ui.Center).Children(func() {
									ui.Checkbox(c, &row.enabled, a.S.Env.Enabled).Width(88)
									ui.TextField(c, &row.kv.Key, a.S.Env.Key, "").Grow(1)
									ui.TextField(c, &row.kv.Value, a.S.Env.Value, "").Grow(1)
									if ui.Button(c, a.S.Env.DeleteRow).Outline().Width(100).Clicked() {
										c.After(func() {
											d.rows = slices.DeleteFunc(d.rows, func(item *envRow) bool { return item.id == row.id })
										})
									}
								})
							}
						})
						if ui.Button(c, a.S.Env.AddRow).Outline().Clicked() {
							c.After(func() { d.rows = append(d.rows, &envRow{id: storage.NewID(), enabled: true}) })
						}
					})
				})
			})
			if s.err != "" {
				ui.Wrap(c, s.err).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, a.S.Env.Close).Outline().Clicked() {
			s.open = false
		}
		if ui.Button(c, a.S.Env.Save).Clicked() {
			c.After(a.saveEnvironments)
		}
	})
	if !s.open {
		c.After(func() { s.drafts, s.deleted, s.err = nil, nil, "" })
	}
}

func (a *App) saveEnvironments() {
	s := &a.env
	for _, d := range s.drafts {
		if strings.TrimSpace(d.name) == "" {
			s.selected, s.rename, s.err = d.id, true, a.S.Env.NameRequired
			return
		}
	}
	for _, d := range s.drafts {
		if err := a.Svc.Environments.Save(d.environment()); err != nil {
			s.err = a.S.Env.Error + ": " + err.Error()
			return
		}
		d.persisted = true
	}
	settings := a.Svc.Settings.Get()
	if slices.Contains(s.deleted, settings.ActiveEnvID) {
		settings.ActiveEnvID = ""
		if err := a.Svc.Settings.Save(settings); err != nil {
			s.err = a.S.Env.Error + ": " + err.Error()
			return
		}
	}
	for len(s.deleted) > 0 {
		if err := a.Svc.Environments.Delete(s.deleted[0]); err != nil {
			s.err = a.S.Env.Error + ": " + err.Error()
			return
		}
		s.deleted = s.deleted[1:]
	}
	s.open, s.drafts, s.deleted, s.err = false, nil, nil, ""
}

// envSidebar is the sidebar list of the Environments rail section. Until
// the env PR builds it, it offers the editor dialog.
func (a *App) envSidebar(c ui.C) {
	ui.Column(c).Fill().Pad(Space4).Gap(Space3).Children(func() {
		ui.Text(c, a.S.Env.Title).Bold()
		if ui.Button(c, a.S.Env.Edit).Outline().Clicked() {
			c.After(a.openEnvironments)
		}
	})
}

// envReady reports whether envMain has an editor to show; while it is false
// the shell keeps the workbench in the Environments section. envMain
// declares the environment editor in the main area. Owned by the env PR.
func (a *App) envReady() bool { return false }

// envMain declares the environment editor in the main area while the
func (a *App) envMain(c ui.C) {}
