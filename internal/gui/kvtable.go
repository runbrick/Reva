package gui

import (
	"slices"

	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/ui"
)

// kvTableState keeps row identities through insertion and deletion. Keep one
// state per table; its zero value is ready to use.
type kvTableState struct {
	rows []*kvRowState
	next int
}

type kvRowState struct {
	id                     int
	enabled                bool
	key, value             ui.Field
	syncedKey, syncedValue string
}

// sync reads edits before the pane declares values derived from this table
// (notably the URL), and puts programmatic row changes into the editors.
func (s *kvTableState) sync(rows []model.KV) bool {
	s.resize(len(rows))
	changed := false
	for i := range rows {
		r, state := &rows[i], s.rows[i]
		if text := state.key.Text(); text != state.syncedKey {
			r.Key, state.syncedKey, changed = text, text, true
		} else if r.Key != state.syncedKey {
			state.key.SetText(r.Key)
			state.syncedKey = r.Key
		}
		if text := state.value.Text(); text != state.syncedValue {
			r.Value, state.syncedValue, changed = text, text, true
		} else if r.Value != state.syncedValue {
			state.value.SetText(r.Value)
			state.syncedValue = r.Value
		}
	}
	return changed
}

func (s *kvTableState) resize(n int) {
	if len(s.rows) > n {
		s.rows = s.rows[:n]
	}
	for len(s.rows) < n {
		s.next++
		s.rows = append(s.rows, &kvRowState{id: s.next})
	}
}

// kvTable edits rows and reports an edit. Structural edits run after the paint;
// callers that derive other values from rows must also use c.After on change.
// Labels are the shared Common strings so other feature panes can reuse it.
func (a *App) kvTable(c ui.C, s *kvTableState, rows *[]model.KV) bool {
	changed := s.sync(*rows)
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		for i := range *rows {
			r, state := &(*rows)[i], s.rows[i]
			state.enabled = !r.Disabled
			ui.Row(c.KeyInt(state.id)).FillWidth().Gap(8).Items(ui.Center).Children(func() {
				if ui.Checkbox(c, &state.enabled, "").Label(a.S.Common.Key).Changed() {
					r.Disabled = !state.enabled
					changed = true
				}
				if ui.TextField(c, &state.key, a.S.Common.Key, a.S.Common.Key).Grow(1).Changed() {
					r.Key, state.syncedKey = state.key.Text(), state.key.Text()
					changed = true
				}
				if ui.TextField(c, &state.value, a.S.Common.Value, a.S.Common.Value).Grow(1).Changed() {
					r.Value, state.syncedValue = state.value.Text(), state.value.Text()
					changed = true
				}
				if ui.Button(c, a.S.Common.Delete).Clicked() {
					changed = true
					c.After(func() {
						*rows = slices.Delete(*rows, i, i+1)
						s.rows = slices.Delete(s.rows, i, i+1)
					})
				}
			})
		}
		if ui.Button(c, a.S.Common.Add).Clicked() {
			changed = true
			c.After(func() { *rows = append(*rows, model.KV{}) })
		}
	})
	return changed
}
