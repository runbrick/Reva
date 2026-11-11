package gui

import (
	"slices"
	"strings"

	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/ui"
)

// kvTableState preserves identities through structural edits and keeps the bulk
// editor in sync before callers derive the URL from its rows.
type kvTableState struct {
	rows       []*kvRowState
	next       int
	bulk       bool
	bulkText   ui.Field
	syncedBulk string
	headers    bool
}

type kvRowState struct {
	id                                        int
	enabled                                   bool
	key, value, description                   ui.Field
	syncedKey, syncedValue, syncedDescription string
}

func (s *kvTableState) sync(rows []model.KV) bool {
	s.resize(len(rows))
	changed := false
	for i := range rows {
		r, state := &rows[i], s.rows[i]
		for _, cell := range []struct {
			field          *ui.Field
			stored, synced *string
		}{
			{&state.key, &r.Key, &state.syncedKey},
			{&state.value, &r.Value, &state.syncedValue},
			{&state.description, &r.Description, &state.syncedDescription},
		} {
			if text := cell.field.Text(); text != *cell.synced {
				*cell.stored, *cell.synced, changed = text, text, true
			} else if *cell.stored != *cell.synced {
				cell.field.SetText(*cell.stored)
				*cell.synced = *cell.stored
			}
		}
	}
	return changed
}

func (s *kvTableState) syncBulk(rows *[]model.KV) bool {
	if !s.bulk || s.bulkText.Text() == s.syncedBulk {
		return false
	}
	s.syncedBulk = s.bulkText.Text()
	*rows = requestParseBulk(s.syncedBulk, *rows)
	s.rows = nil
	return true
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

func requestBulkText(rows []model.KV) string {
	var out strings.Builder
	for i, row := range rows {
		if i > 0 {
			out.WriteByte('\n')
		}
		if row.Disabled {
			out.WriteString("// ")
		}
		out.WriteString(row.Key)
		out.WriteString(": ")
		out.WriteString(row.Value)
	}
	return out.String()
}

func requestParseBulk(text string, previous []model.KV) []model.KV {
	var rows []model.KV
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		disabled := strings.HasPrefix(line, "//")
		if disabled {
			line = strings.TrimSpace(strings.TrimPrefix(line, "//"))
		}
		key, value, _ := strings.Cut(line, ":")
		rows = append(rows, model.KV{Key: strings.TrimSpace(key), Value: strings.TrimSpace(value), Disabled: disabled})
	}
	// Match all surviving keys first, so inserting a new row cannot take a
	// later row's note. Duplicate keys retain their notes in occurrence order.
	used, matched := make([]bool, len(previous)), make([]bool, len(rows))
	for index := range rows {
		for i, old := range previous {
			if !used[i] && old.Key == rows[index].Key {
				rows[index].Description, used[i], matched[index] = old.Description, true, true
				break
			}
		}
	}
	// A renamed key at an otherwise unchanged position keeps that row's note.
	for index := range rows {
		if !matched[index] && index < len(previous) && !used[index] {
			rows[index].Description = previous[index].Description
		}
	}
	return rows
}

// kvTable reports edits, including deferred insertions and deletions. Callers
// deriving the URL also schedule their update with c.After.
func (a *App) kvTable(c ui.C, s *kvTableState, rows *[]model.KV) bool {
	changed := s.syncBulk(rows)
	changed = s.sync(*rows) || changed
	theme, labels := c.Theme(), &a.S.Request
	key, value, add := labels.ParamName, labels.ParamValue, labels.AddParam
	if s.headers {
		key, value, add = labels.HeaderName, labels.HeaderValue, labels.AddHeader
	}
	ui.Column(c).FillWidth().Gap(Space2).Children(func() {
		toggle := labels.BulkEdit
		if s.bulk {
			toggle = labels.TableEdit
		}
		if ui.Button(c, toggle).Ghost().Small().Self(ui.End).Clicked() {
			if !s.bulk {
				s.syncedBulk = requestBulkText(*rows)
				s.bulkText.SetText(s.syncedBulk)
			}
			s.bulk = !s.bulk
		}
		if s.bulk {
			ui.Text(c, labels.BulkHelp).Muted().FontSize(theme.Metrics.FontSizeS)
			if ui.TextArea(c, &s.bulkText, labels.BulkContent, "").Mono().FillWidth().Height(Space6 * 6).Changed() {
				changed = s.syncBulk(rows) || changed
			}
			return
		}
		ui.Column(c).FillWidth().Border(1, theme.Border).Radius(theme.Metrics.RadiusControl).Clip().Children(func() {
			ui.Row(c).FillWidth().Height(theme.Metrics.ControlHeight).Items(ui.Center).Background(a.palette.TableHeader).Children(func() {
				ui.Row(c).Width(theme.Metrics.ControlHeight + Space4).Shrink(0)
				ui.Divider(c).Vertical()
				ui.Text(c, key).Basis(0).Grow(1).PadX(Space3).FontSize(theme.Metrics.FontSizeS).Bold()
				ui.Divider(c).Vertical()
				ui.Text(c, value).Basis(0).Grow(1).PadX(Space3).FontSize(theme.Metrics.FontSizeS).Bold()
				ui.Divider(c).Vertical()
				ui.Text(c, labels.Description).Basis(0).Grow(1).PadX(Space3).FontSize(theme.Metrics.FontSizeS).Bold()
				ui.Row(c).Width(theme.Metrics.ControlHeight).Shrink(0)
			})
			for i := range *rows {
				r, state := &(*rows)[i], s.rows[i]
				state.enabled = !r.Disabled
				ui.Divider(c)
				rowContext := c.KeyInt(state.id)
				row := ui.Row(rowContext).FillWidth().Height(theme.Metrics.ControlHeight).Items(ui.Center).
					Hit(ui.ID(rowContext.Part("row")), key).Hover(ui.Look{}.Background(theme.Hover))
				row.Children(func() {
					ui.Row(c).Width(theme.Metrics.ControlHeight + Space4).Shrink(0).Center().Children(func() {
						if ui.Checkbox(c, &state.enabled, "").Label(a.S.Common.Key).Changed() {
							r.Disabled = !state.enabled
							changed = true
						}
					})
					ui.Divider(c).Vertical()
					if ui.TextField(c, &state.key, a.S.Common.Key, key).Basis(0).Grow(1).Height(theme.Metrics.ControlHeight).Border(0, theme.Border).Radius(0).Background(theme.Background).PadX(Space3).Changed() {
						r.Key, state.syncedKey = state.key.Text(), state.key.Text()
						changed = true
					}
					ui.Divider(c).Vertical()
					if ui.TextField(c, &state.value, a.S.Common.Value, value).Basis(0).Grow(1).Height(theme.Metrics.ControlHeight).Border(0, theme.Border).Radius(0).Background(theme.Background).PadX(Space3).Changed() {
						r.Value, state.syncedValue = state.value.Text(), state.value.Text()
						changed = true
					}
					ui.Divider(c).Vertical()
					if ui.TextField(c, &state.description, labels.Description, labels.Description).Basis(0).Grow(1).Height(theme.Metrics.ControlHeight).Border(0, theme.Border).Radius(0).Background(theme.Background).PadX(Space3).Changed() {
						r.Description, state.syncedDescription = state.description.Text(), state.description.Text()
						changed = true
					}
					if ui.Button(c, a.S.Common.Delete).Icon(icons.Trash2).IconOnly().Ghost().
						Opacity(0).StateFrom(row).Hover(ui.Look{}.Opacity(1)).Focused(ui.Look{}.Opacity(1)).Clicked() {
						changed = true
						c.After(func() { *rows = slices.Delete(*rows, i, i+1); s.rows = slices.Delete(s.rows, i, i+1) })
					}
				})
			}
			ui.Divider(c)
			if ui.ButtonBase(c, a.S.Common.Add).FillWidth().Height(theme.Metrics.ControlHeight).Justify(ui.Start).PadX(Space3).Gap(Space2).
				Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring)).Children(func() {
				ui.Icon(c, icons.Plus).Size(Space4, Space4).Color(theme.MutedForeground)
				ui.Text(c, add).Color(theme.MutedForeground)
			}).Clicked() {
				changed = true
				c.After(func() { *rows = append(*rows, model.KV{}) })
			}
		})
	})
	return changed
}
