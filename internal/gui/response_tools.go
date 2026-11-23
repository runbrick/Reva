package gui

import (
	"fmt"
	"strconv"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/responsejson"
	"github.com/vivaui/viva/ui"
)

type responseBaseline struct {
	status int
	doc    *responsejson.Document
}

type responseToolsState struct {
	open, inspect, hasSelection, moreFields, moreChanges, invalidIgnore bool
	source                                                              *model.Response
	artifact                                                            responseArtifactSource
	doc                                                                 *responsejson.Document
	compared                                                            *responseBaseline
	changes                                                             []responsejson.Change
	fields                                                              []responsejson.Field
	filter, ignored, pointer, variable                                  string
}

func (state *responseToolsState) show(response *model.Response) {
	if response == nil {
		return
	}
	if state.source != response {
		*state = responseToolsState{source: response, ignored: state.ignored, artifact: state.artifact}
		if !response.Truncated {
			state.doc, _ = responsejson.Parse(response.Body)
		}
		if state.doc != nil {
			state.fields, state.moreFields = state.doc.Fields("", 64)
		}
	}
	state.open = true
}

func (state *responseToolsState) compare(baseline *responseBaseline) {
	state.compared, state.changes, state.moreChanges = nil, nil, false
	ignored, err := responsejson.IgnoredPaths(state.ignored)
	state.invalidIgnore = err != nil
	if err != nil {
		return
	}
	state.compared = baseline
	if baseline.doc != nil && state.doc != nil {
		state.changes, state.moreChanges = responsejson.Diff(baseline.doc, state.doc, ignored, 100)
	}
}

var responseToolModes = []bool{false, true}

func (a *App) responseToolsModal(ctx ui.C, tab *Tab, state *responseToolsState) {
	if !state.open {
		return
	}
	if state.source != tab.Response || tab.Sending || tab.sse != nil {
		state.open = false
		return
	}
	labels := &a.S.Response.Tools
	ui.ModalWith(ctx, &state.open, labels.Title, ui.ModalSpec{Width: Space6 * 30, Height: Space6 * 20}, func() {
		ui.Column(ctx).Fill().Gap(Space3).Children(func() {
			ui.Choice(ctx, &state.inspect, responseToolModes, func(inspect bool) string {
				if inspect {
					return labels.Fields
				}
				return labels.Compare
			})
			if state.inspect {
				a.responseFieldTools(ctx.Key("fields"), tab, state)
			} else {
				a.responseComparison(ctx.Key("compare"), state)
			}
		})
	}, func() {
		if ui.Button(ctx, a.S.Common.Close).Outline().Clicked() {
			state.open = false
		}
	})
}

func (a *App) responseComparison(ctx ui.C, state *responseToolsState) {
	labels := &a.S.Response.Tools
	baseline := a.response.baseline
	ui.Row(ctx).FillWidth().Gap(Space2).Children(func() {
		if ui.Button(ctx, labels.SetBaseline).Outline().Clicked() {
			ctx.After(func() {
				a.clearResponseBaseline()
				a.response.baseline = &responseBaseline{status: state.source.StatusCode, doc: state.doc}
			})
		}
		if ui.Button(ctx, labels.RunComparison).Disabled(baseline == nil).Clicked() && baseline != nil {
			ctx.After(func() { state.compare(baseline) })
		}
		if ui.Button(ctx, labels.ClearBaseline).Ghost().Disabled(baseline == nil).Clicked() {
			ctx.After(a.clearResponseBaseline)
		}
	})
	if baseline == nil {
		ui.Wrap(ctx, labels.NoBaseline).Muted()
		return
	}
	if ui.TextArea(ctx, &state.ignored, labels.Ignored, labels.IgnoreHint).FillWidth().Height(Space6 * 2).Changed() {
		state.compared, state.changes, state.invalidIgnore = nil, nil, false
	}
	if state.invalidIgnore {
		ui.Wrap(ctx, labels.IgnoreError).Color(ctx.Theme().Danger)
	}
	if state.compared != baseline {
		ui.Wrap(ctx, labels.BaselineReady).Muted()
		return
	}
	ui.Row(ctx).FillWidth().Gap(Space3).Children(func() {
		ui.Text(ctx, labels.Status).Bold()
		a.responseComparisonValue(ctx, labels.Baseline, strconv.Itoa(baseline.status))
		a.responseComparisonValue(ctx, labels.Current, strconv.Itoa(state.source.StatusCode))
	})
	if baseline.doc == nil || state.doc == nil {
		ui.Wrap(ctx, labels.JSONUnavailable).Muted()
		return
	}
	if len(state.changes) == 0 {
		ui.Wrap(ctx, labels.NoChanges).Muted()
		return
	}
	ui.Scroll(ctx).FillWidth().Grow(1).MinHeight(0).Scrollbars().Children(func() {
		ui.Column(ctx).FillWidth().Gap(Space2).Children(func() {
			for index, change := range state.changes {
				a.responseChange(ctx.KeyInt(index), state, change)
			}
			if state.moreChanges {
				ui.Wrap(ctx, labels.MoreChanges).Muted()
			}
		})
	})
}

func (a *App) clearResponseBaseline() {
	a.response.baseline = nil
	for _, view := range a.response.views {
		view.tools.compared, view.tools.changes = nil, nil
	}
}

func (a *App) responseChange(ctx ui.C, state *responseToolsState, change responsejson.Change) {
	labels := &a.S.Response.Tools
	label, color := labels.ValueChanged, ctx.Theme().Primary
	switch change.Kind {
	case responsejson.Added:
		label, color = labels.Added, ctx.Theme().Success
	case responsejson.Removed:
		label, color = labels.Removed, ctx.Theme().Danger
	case responsejson.TypeChanged:
		label, color = labels.TypeChanged, a.palette.Warning
	}
	ui.Column(ctx).FillWidth().Pad(Space2).Gap(Space1).Background(ctx.Theme().Muted).Children(func() {
		ui.Row(ctx).FillWidth().Items(ui.Center).Children(func() {
			ui.Text(ctx, label).Color(color).Bold()
			ui.Spacer(ctx)
			if change.After != nil && ui.Button(ctx, labels.InspectCurrent).Small().Ghost().Clicked() {
				state.pointer, state.hasSelection, state.inspect = change.Pointer, true, true
			}
		})
		ui.Text(ctx, responsePointerLabel(labels, change.Pointer)).FillWidth().Ellipsis().Mono()
		ui.Row(ctx).FillWidth().Gap(Space3).Children(func() {
			a.responseComparisonValue(ctx, labels.Baseline, responseFieldValue(labels, change.Before))
			a.responseComparisonValue(ctx, labels.Current, responseFieldValue(labels, change.After))
		})
	})
}

func (a *App) responseComparisonValue(ctx ui.C, label, value string) {
	ui.Column(ctx).Basis(0).Grow(1).MinWidth(0).Gap(Space1).Children(func() {
		ui.Text(ctx, label).Muted()
		ui.Wrap(ctx, value).Mono()
	})
}

func (a *App) responseFieldTools(ctx ui.C, tab *Tab, state *responseToolsState) {
	labels := &a.S.Response.Tools
	if state.doc == nil {
		ui.Wrap(ctx, labels.JSONUnavailable).Muted()
		return
	}
	if state.hasSelection {
		a.responseFieldActions(ctx.Key("actions"), tab, state, state.doc.Lookup(state.pointer))
	} else {
		ui.Wrap(ctx, labels.SelectField).Muted()
	}
	if ui.TextField(ctx, &state.filter, labels.Filter, labels.FilterHint).FillWidth().Changed() {
		state.fields, state.moreFields = state.doc.Fields(state.filter, 64)
	}
	if len(state.fields) == 0 {
		ui.Wrap(ctx, labels.NoFields).Muted()
		return
	}
	ui.Scroll(ctx).FillWidth().Grow(1).MinHeight(0).Scrollbars().Children(func() {
		ui.Column(ctx).FillWidth().Gap(Space1).Children(func() {
			for _, field := range state.fields {
				label := responsePointerLabel(labels, field.Pointer)
				if ui.Button(ctx.Key(field.Pointer), label).Outline().Small().FillWidth().Clicked() {
					state.pointer, state.hasSelection = field.Pointer, true
				}
			}
			if state.moreFields {
				ui.Wrap(ctx, labels.MoreFields).Muted()
			}
		})
	})
}

func (a *App) responseFieldActions(ctx ui.C, tab *Tab, state *responseToolsState, field *responsejson.Value) {
	labels := &a.S.Response.Tools
	path := responsePointerLabel(labels, state.pointer)
	ui.Text(ctx, labels.Selected+"  "+path+"  "+responseFieldValue(labels, field)).FillWidth().Ellipsis().Mono()
	encoded, scalar := field.JSONScalar()
	pointer := responsejson.Quote(state.pointer)
	ui.Row(ctx).FillWidth().Gap(Space2).Children(func() {
		if ui.Button(ctx, labels.Exists).Outline().Small().Clicked() {
			a.addResponseTest(ctx, tab, state, fmt.Sprintf(labels.ExistsTest, path), "hasJSONField("+pointer+")")
		}
		if ui.Button(ctx, labels.Equals).Outline().Small().Disabled(!scalar).Clicked() && scalar {
			a.addResponseTest(ctx, tab, state, fmt.Sprintf(labels.EqualsTest, path), "jsonFieldEquals("+pointer+", "+responsejson.Quote(encoded)+")")
		}
	})
	if scalar {
		ui.Wrap(ctx, labels.EqualityHelp).Muted().FontSize(ctx.Theme().Metrics.FontSizeS)
		ui.Row(ctx).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			ui.TextField(ctx, &state.variable, labels.Variable, labels.VariableHint).Grow(1).MinWidth(0)
			name := state.variable
			if ui.Button(ctx, labels.Capture).Outline().Small().Disabled(!responsejson.ValidVariable(name)).Clicked() && responsejson.ValidVariable(name) {
				a.addResponseTest(ctx, tab, state, fmt.Sprintf(labels.CaptureTest, name), "captureJSON("+pointer+", "+responsejson.Quote(name)+")")
			}
		})
		ui.Wrap(ctx, labels.CaptureHelp).Muted().FontSize(ctx.Theme().Metrics.FontSizeS)
	}
}

func (a *App) addResponseTest(ctx ui.C, tab *Tab, state *responseToolsState, name, expression string) {
	source := "pm.test(" + responsejson.Quote(name) + ", function () {\n  pm.expect(pm.response." + expression + ").to.equal(true);\n});"
	ctx.After(func() {
		if tab.Response != state.source {
			return
		}
		a.insertScript(tab, a.scriptTab(tab), source)
		a.requestTab(tab).section = requestScripts
		state.open = false
		a.Toast(a.S.Response.Tools.ScriptAdded, "", ui.ToastSuccess)
	})
}

func responsePointerLabel(labels *i18n.ResponseTools, path string) string {
	if path == "" {
		return labels.Root
	}
	quoted := strconv.Quote(path)
	return quoted[1 : len(quoted)-1]
}

func responseFieldValue(labels *i18n.ResponseTools, field *responsejson.Value) string {
	if field == nil {
		return labels.Missing
	}
	switch field.Kind() {
	case responsejson.Object:
		return labels.Object
	case responsejson.Array:
		return labels.Array
	case responsejson.String:
		return labels.String + "  " + field.Summary()
	case responsejson.Number:
		return labels.Number + "  " + field.Summary()
	case responsejson.Boolean:
		return labels.Boolean + "  " + field.Summary()
	default:
		return labels.Null
	}
}
