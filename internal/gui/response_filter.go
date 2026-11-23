package gui

import (
	"github.com/runbrick/reva/internal/format"
	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/responsejson"
	"github.com/vivaui/viva/ui"
)

type responseFilterState struct {
	open   bool
	query  string
	result responsejson.FilterResult
	err    error
	code   responseCodeView
}

func (state *responseFilterState) clearResult() {
	state.result = responsejson.FilterResult{}
	state.err = nil
	state.code = responseCodeView{}
}

func (state *responseFilterState) run(body []byte) {
	state.clearResult()
	state.result, state.err = responsejson.Filter(body, state.query)
	if state.result.Matches > 0 {
		state.code.set(state.result.JSON, format.TokenizeJSON(state.result.JSON))
	}
}

func (a *App) responseFilterModal(c ui.C, v *responseView) {
	state := &v.filter
	if !state.open {
		return
	}
	labels := i18n.ResponseFilterFor(a.S.Lang)
	truncated := v.source.Truncated
	ui.ModalWith(c, &state.open, labels.Title, ui.ModalSpec{
		Description: labels.Syntax, Width: Space6 * 28, Height: Space6 * 19,
	}, func() {
		ui.Column(c).Fill().Gap(Space2).Children(func() {
			ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
				previous := state.query
				field := ui.TextField(c, &state.query, labels.Query, labels.Placeholder).Grow(1).MinWidth(0).Disabled(truncated)
				if state.query != previous {
					state.clearResult()
				}
				run := ui.Button(c, labels.Run).Shrink(0).Disabled(truncated)
				if !truncated && (run.Clicked() || field.Submitted()) {
					c.After(func() { state.run(v.source.Body) })
				}
			})
			ui.Paragraph(c, labels.Limits, 0).FillWidth().Muted()
			if message := state.message(labels, truncated); message != "" {
				ui.Paragraph(c, message, 0).FillWidth().Color(c.Theme().MutedForeground)
			}
			if state.result.Matches > 0 {
				state.code.declare(c, labels.Result, a.palette, true, nil)
			}
		})
	}, func() {
		if ui.Button(c, labels.Copy).Outline().Disabled(state.result.Matches == 0).Clicked() {
			a.responseCopy(c, state.result.JSON)
		}
		if ui.Button(c, a.S.Common.Close).Outline().Clicked() {
			state.open = false
		}
	})
}

func (state *responseFilterState) message(labels *i18n.ResponseFilter, truncated bool) string {
	if truncated {
		return labels.Truncated
	}
	switch state.err {
	case responsejson.ErrFilterSyntax:
		return labels.InvalidQuery
	case responsejson.ErrFilterQueryLimit:
		return labels.QueryLimit
	case responsejson.ErrFilterInputLimit:
		return labels.InputLimit
	case responsejson.ErrFilterResultLimit:
		return labels.ResultLimit
	case responsejson.ErrInvalid:
		return labels.InvalidJSON
	case nil:
		if state.result.Matches == 0 && state.result.JSON != "" {
			return labels.NoMatches
		}
	}
	return ""
}
