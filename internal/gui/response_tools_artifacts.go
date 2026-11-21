package gui

import (
	"errors"
	"strconv"
	"strings"

	"github.com/runbrick/reva/internal/execution"
	"github.com/runbrick/reva/internal/format"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/responseexample"
	"github.com/runbrick/reva/internal/vars"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type responseArtifactSource struct {
	response  *model.Response
	requestID string
	method    model.Method
	path      string
}

func (source responseArtifactSource) available(tab *Tab) bool {
	return source.response != nil && source.response == tab.Response && source.requestID == tab.Request.ID &&
		!tab.Sending && tab.Err == "" && tab.sse == nil
}

func (source responseArtifactSource) sample() responseexample.Sample {
	return responseexample.FromResponse(source.method, source.path, source.response)
}

func (a *App) captureResponseExampleSource(tab *Tab, result execution.Result) {
	method := result.Prepared.Method
	if !result.TransportStarted || tab.sse != nil || !responseexample.ValidMethod(method) || result.Response.StatusCode < 100 || result.Response.StatusCode > 599 {
		return
	}
	draftURL := result.HistoryRequest.URL
	const variableMarker = "reva-variable"
	var placeholders []model.KV
	for _, name := range vars.Unresolved(draftURL, nil) {
		placeholders = append(placeholders, model.KV{Key: name, Value: variableMarker})
	}
	path := responseexample.HTTPPath(vars.Expand(draftURL, placeholders))
	usesVariable := len(placeholders) != 0 && strings.Contains(path, variableMarker)
	sentPath := responseexample.HTTPPath(vars.Expand(result.Prepared.URL, result.Variables))
	if usesVariable || path != sentPath {
		path = ""
	}
	if a.response.views == nil {
		a.response.views = make(map[int]*responseView)
	}
	view := a.response.views[tab.ID]
	if view == nil {
		view = &responseView{}
		a.response.views[tab.ID] = view
	}
	view.prepare(result.Response)
	view.tools.artifact = responseArtifactSource{response: result.Response, requestID: result.HistoryRequest.ID, method: method, path: path}
}

type responseArtifactMode int

const (
	exampleView responseArtifactMode = iota
	exampleSave
	exampleDelete
	mappingReview
	mappingPreview
)

type responseArtifactsState struct {
	mode        responseArtifactMode
	open        bool
	fromExample bool
	exists      bool
	loadFailed  bool
	includeBody bool
	key         responseexample.Key
	saved       responseexample.Sample
	draft       responseexample.Sample
	mapping     []byte
	errorText   string
	code        responseCodeView
}

func (a *App) exampleKey(tab *Tab) (responseexample.Key, bool) {
	key := responseexample.Key{CollectionID: tab.CollectionID, RequestID: tab.Request.ID}
	if key.CollectionID == "" || key.RequestID == "" {
		return key, false
	}
	collection, found := a.Svc.Collections.Get(key.CollectionID)
	item := model.FindItem(collection.Items, key.RequestID)
	return key, found && item != nil && item.Type == model.ItemRequest && item.Request != nil
}

func (a *App) responseArtifactBar(ctx ui.C, tab *Tab) {
	var source responseArtifactSource
	if view := a.response.views[tab.ID]; view != nil {
		source = view.tools.artifact
	}
	canExport := source.available(tab)
	canOpen := tab.CollectionID != "" && tab.Request.ID != ""
	if !canOpen && !canExport {
		return
	}
	ui.Column(ctx.Key("response-artifacts")).FillWidth().Shrink(0).Children(func() {
		a.responseArtifactControls(ctx.KeyInt(tab.ID), tab, source, canOpen, canExport)
	})
}

func (a *App) responseArtifactControls(ctx ui.C, tab *Tab, source responseArtifactSource, canOpen, canExport bool) {
	labels := &a.S.Response.Artifacts
	ui.Row(ctx).FillWidth().Height(TabBarHeight).PadX(Space4).Gap(Space2).Items(ui.Center).Background(ctx.Theme().Card).Children(func() {
		state := ui.State[responseArtifactsState](ctx, "dialog")
		if canOpen && ui.Button(ctx, labels.Example).Small().Outline().Clicked() {
			ctx.After(func() {
				key, saved := a.exampleKey(tab)
				if !saved {
					a.Toast(labels.Example, labels.RequiresSaved, ui.ToastError)
					return
				}
				*state = responseArtifactsState{open: true, key: key}
				sample, exists, err := responseexample.Load(key)
				state.saved, state.exists, state.loadFailed = sample, exists, err != nil
			})
		}
		if canExport && ui.Button(ctx, labels.Export).Small().Outline().Clicked() {
			ctx.After(func() {
				*state = responseArtifactsState{open: true, mode: mappingReview, draft: source.sample()}
			})
		}
		if state.open {
			a.responseArtifactDialog(ctx, tab, state, source)
			if !state.open {
				ctx.After(func() { *state = responseArtifactsState{} })
			}
		}
	})
}

func (a *App) responseArtifactDialog(ctx ui.C, tab *Tab, state *responseArtifactsState, source responseArtifactSource) {
	labels := &a.S.Response.Artifacts
	title := labels.Example
	if state.mode == mappingReview || state.mode == mappingPreview {
		title = labels.Export
	}
	ui.ModalWith(ctx, &state.open, title, ui.ModalSpec{Width: Space6 * 29, Height: Space6 * 21}, func() {
		ui.Column(ctx).Fill().Gap(Space2).Children(func() {
			if state.errorText != "" {
				ui.Wrap(ctx, state.errorText).Color(ctx.Theme().Danger)
			}
			switch state.mode {
			case exampleView:
				a.exampleDetails(ctx, state, tab, source)
			case exampleSave, mappingReview:
				a.responseArtifactReview(ctx, state)
			case exampleDelete:
				ui.Wrap(ctx, labels.DeleteHelp)
			case mappingPreview:
				ui.Wrap(ctx, labels.PreviewHelp).Muted()
				state.code.declare(ctx, labels.JSONPreview, a.palette, true, nil)
			}
		})
	}, func() {
		a.responseArtifactActions(ctx, tab, state)
	})
}

func (a *App) exampleDetails(ctx ui.C, state *responseArtifactsState, tab *Tab, source responseArtifactSource) {
	labels := &a.S.Response.Artifacts
	switch {
	case state.loadFailed:
		ui.Wrap(ctx, labels.ReadError).Color(ctx.Theme().Danger)
	case !state.exists:
		ui.Wrap(ctx, labels.Empty).Muted()
	default:
		a.responseArtifactSummary(ctx, state.saved)
		if state.saved.Path != "" {
			ui.Text(ctx, state.saved.Path).Mono().Ellipsis()
		}
		if state.saved.Body == "" {
			ui.Wrap(ctx, labels.NoSavedBody).Muted()
		} else {
			state.code.set(state.saved.Body, nil)
			state.code.declare(ctx, labels.BodyReview, a.palette, true, nil)
		}
	}
	ui.Row(ctx).FillWidth().Gap(Space2).Children(func() {
		if source.available(tab) && !state.loadFailed {
			label := labels.ReviewSave
			if state.exists {
				label = labels.ReviewReplace
			}
			if ui.Button(ctx, label).Outline().Clicked() {
				ctx.After(func() { state.mode, state.draft = exampleSave, source.sample() })
			}
		}
		if state.exists && ui.Button(ctx, labels.ExportExample).Outline().Clicked() {
			state.mode, state.draft, state.fromExample = mappingReview, state.saved, true
		}
		if (state.exists || state.loadFailed) && ui.Button(ctx, labels.Delete).Ghost().Clicked() {
			state.mode = exampleDelete
		}
	})
}

func (a *App) responseArtifactSummary(ctx ui.C, sample responseexample.Sample) {
	labels := &a.S.Response
	ui.Row(ctx).FillWidth().Gap(Space3).Items(ui.Center).Children(func() {
		a.MethodBadge(ctx, sample.Method)
		ui.Text(ctx, labels.Tools.Status+"  "+strconv.Itoa(sample.Status)).Bold()
		if sample.ContentType != "" {
			ui.Text(ctx, sample.ContentType).Mono().Muted()
		}
	})
}

func (a *App) responseArtifactReview(ctx ui.C, state *responseArtifactsState) {
	labels := &a.S.Response.Artifacts
	a.responseArtifactSummary(ctx, state.draft)
	pathHelp, pathLabel := labels.ExportHelp, labels.Path
	if state.mode == exampleSave {
		pathHelp, pathLabel = labels.SaveHelp, labels.OptionalPath
	}
	ui.Wrap(ctx, pathHelp).Muted()
	ui.TextField(ctx, &state.draft.Path, pathLabel, "/v1/items").FillWidth()
	if state.draft.Path != "" && !responseexample.ValidPath(state.draft.Path) {
		ui.Wrap(ctx, labels.PathError).Color(ctx.Theme().Danger)
	}
	if state.draft.Body == "" {
		ui.Wrap(ctx, labels.BodyUnavailable).Muted()
		return
	}
	if state.mode == mappingReview && !state.draft.CanExportBody() {
		ui.Wrap(ctx, labels.TemplateBody).Muted()
		return
	}
	ui.Checkbox(ctx, &state.includeBody, labels.IncludeBody)
	if state.includeBody {
		ui.Wrap(ctx, labels.BodyHelp).Color(a.palette.Warning)
		state.code.set(state.draft.Body, nil)
		state.code.declare(ctx, labels.BodyReview, a.palette, true, nil)
	}
}

func (a *App) responseArtifactActions(ctx ui.C, tab *Tab, state *responseArtifactsState) {
	labels := &a.S.Response.Artifacts
	if state.mode == exampleView {
		if ui.Button(ctx, a.S.Common.Close).Outline().Clicked() {
			state.open = false
		}
		return
	}
	if ui.Button(ctx, labels.Back).Outline().Clicked() {
		switch state.mode {
		case mappingPreview:
			state.mode, state.mapping = mappingReview, nil
		case mappingReview:
			if !state.fromExample {
				state.open = false
				return
			}
			fallthrough
		default:
			state.mode, state.includeBody = exampleView, false
		}
		state.errorText = ""
		return
	}
	switch state.mode {
	case exampleSave:
		button := labels.Save
		if state.exists {
			button = labels.Replace
		}
		valid := state.draft.Path == "" || responseexample.ValidPath(state.draft.Path)
		if ui.Button(ctx, button).Disabled(!valid).Clicked() && valid {
			ctx.After(func() { a.persistExample(tab, state) })
		}
	case exampleDelete:
		if ui.Button(ctx, labels.ConfirmDelete).Destructive().Clicked() {
			ctx.After(func() { a.deleteExample(tab, state) })
		}
	case mappingReview:
		valid := responseexample.ValidPath(state.draft.Path)
		if ui.Button(ctx, labels.Preview).Disabled(!valid).Clicked() && valid {
			ctx.After(func() {
				data, err := state.draft.Mapping(state.includeBody)
				if err != nil {
					state.errorText = labels.MappingError
					return
				}
				preview := string(data)
				state.mode, state.mapping, state.errorText = mappingPreview, data, ""
				state.code.set(preview, format.TokenizeJSON(preview))
			})
		}
	case mappingPreview:
		if ui.Button(ctx, labels.ConfirmExport).Clicked() {
			ctx.After(func() { a.writeResponseMapping(state) })
		}
	}
}

func (a *App) persistExample(tab *Tab, state *responseArtifactsState) {
	labels := &a.S.Response.Artifacts
	key, found := a.exampleKey(tab)
	if !found || key != state.key {
		state.errorText = labels.RequiresSaved
		return
	}
	sample := state.draft
	if !state.includeBody {
		sample.Body = ""
	}
	if err := responseexample.Save(key, sample); err != nil {
		state.errorText = labels.WriteError
		return
	}
	state.saved, state.exists, state.mode = sample, true, exampleView
	state.draft, state.includeBody, state.errorText = responseexample.Sample{}, false, ""
	a.Toast(labels.Saved, "", ui.ToastSuccess)
}

func (a *App) deleteExample(tab *Tab, state *responseArtifactsState) {
	labels := &a.S.Response.Artifacts
	key, found := a.exampleKey(tab)
	if !found || key != state.key {
		state.errorText = labels.RequiresSaved
		return
	}
	if err := responseexample.Delete(key); err != nil {
		state.errorText = labels.DeleteError
		return
	}
	*state = responseArtifactsState{open: true, key: key}
	a.Toast(labels.Deleted, "", ui.ToastSuccess)
}

func (a *App) writeResponseMapping(state *responseArtifactsState) {
	labels := &a.S.Response.Artifacts
	path, err := app.SaveFile(app.SaveFileOptions{
		Title: labels.Export, SuggestedName: "wiremock-mapping.json", DefaultExtension: "json",
		Filters: []app.FileFilter{{Name: "JSON", Pattern: "*.json"}},
	})
	if errors.Is(err, app.ErrCanceled) || (err == nil && path == "") {
		return
	}
	if err != nil || responseexample.WriteMapping(path, state.mapping) != nil {
		state.errorText = labels.ExportError
		return
	}
	*state = responseArtifactsState{}
	a.Toast(labels.Exported, "", ui.ToastSuccess)
}
