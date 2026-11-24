package gui

import (
	"fmt"
	"slices"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/preview"
	"github.com/vivaui/viva/ui"
)

// Only the active HTTP tab can have a visible preview. Called by tab title
// masking as well as the editor, so switching directly to WebSocket clears it.
func (a *App) requestPreviewOpen() bool {
	active := a.Active()
	for id, state := range a.request.tabs {
		if id != active.ID || active.Kind != requestTabKind || isWebSocket(active.Request) || active.Request.Method == model.MethodGRPC || a.Main != PageWorkbench || a.Section == SectionEnvironments {
			state.previewOpen = false
		}
	}
	state := a.request.tabs[active.ID]
	return state != nil && state.previewOpen
}

func (a *App) requestPreviewSnapshot(t *Tab) preview.Snapshot {
	col, _ := a.Svc.Collections.Get(t.CollectionID)
	env, _ := a.activeEnvironment(col)
	return preview.Build(t.Request, col, env)
}

// Picker labels use the same redactor as the preview. Include the candidate's
// values as well as the active environment's values without changing active
// expansion precedence or either stored environment.
func (a *App) requestPreviewEnvironmentName(candidate model.Environment) string {
	t := a.Active()
	col, _ := a.Svc.Collections.Get(t.CollectionID)
	active, _ := a.activeEnvironment(col)
	candidate.Variables = append(slices.Clone(candidate.Variables), active.Variables...)
	return preview.Build(t.Request, col, candidate).EnvironmentName
}

// The preview is rebuilt on every declaration; no resolved credential values
// are retained in editor state. Its only state is requestTabState.previewOpen.
func (a *App) requestPreview(c ui.C, t *Tab, state *requestTabState) {
	if t.Kind != requestTabKind || isWebSocket(t.Request) || t.Request.Method == model.MethodGRPC {
		state.previewOpen = false
		return
	}
	snapshot := a.requestPreviewSnapshot(t)
	s, theme := &a.S.Request, c.Theme()
	ui.Column(c).Fill().Background(a.palette.Surface)
	ui.ModalWith(c, &state.previewOpen, s.PreviewTitle, ui.ModalSpec{Width: Space6 * 28, Height: Space6 * 20}, func() {
		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Scrollbars().Children(func() {
			ui.Column(c).FillWidth().Gap(Space2).Children(func() {
				ui.Wrap(c, s.PreviewPhase).Color(theme.Primary)
				if snapshot.EnvironmentName == "" {
					ui.Wrap(c, s.PreviewNoEnvironment).Muted()
				} else {
					ui.Wrap(c, fmt.Sprintf(s.PreviewActiveEnvironment, snapshot.EnvironmentName)).Muted()
				}
				ui.Text(c, s.URL).Bold()
				if snapshot.InvalidURL {
					ui.Wrap(c, s.PreviewURLInvalid).Color(theme.Danger)
				} else {
					ui.Wrap(c, snapshot.URL).Mono()
				}
				ui.Wrap(c, s.PreviewMaskHelp).Muted().FontSize(theme.Metrics.FontSizeS)
				ui.Divider(c)
				origin := s.PreviewOwn
				switch snapshot.AuthSource {
				case auth.SourceFolder:
					origin = s.PreviewFolder
				case auth.SourceCollection:
					origin = s.PreviewCollection
				}
				ui.Wrap(c, fmt.Sprintf(s.PreviewAuth, a.requestAuthTypeLabel(snapshot.AuthType), origin))
				if snapshot.AuthMissing {
					ui.Wrap(c, s.AuthMissingContext).Color(theme.Danger)
				} else if snapshot.AuthInvalid {
					ui.Wrap(c, s.AuthInvalid).Color(theme.Danger)
				}
				if snapshot.AuthOverridden {
					ui.Wrap(c, s.PreviewAuthOverride).Muted()
				}
				if snapshot.AuthQuery {
					ui.Wrap(c, s.PreviewAuthQuery).Muted()
				}
				ui.Text(c, s.Headers).Bold()
				if len(snapshot.HeaderNames) == 0 {
					ui.Wrap(c, s.PreviewNoHeaders).Muted()
				}
				for i, name := range snapshot.HeaderNames {
					ui.Wrap(c.KeyInt(i), name+": "+s.PreviewMasked).Mono()
				}
				ui.Wrap(c, fmt.Sprintf(s.PreviewBody, a.requestBodyModeLabel(snapshot.BodyMode))).Muted()
				ui.Divider(c)
				ui.Text(c, s.PreviewVariables).Bold()
				if len(snapshot.Variables) == 0 {
					ui.Wrap(c, s.PreviewNoVariables).Muted()
				}
				for i, variable := range snapshot.Variables {
					scope, status := s.PreviewCollection, s.PreviewWinner
					if variable.Scope == preview.Environment {
						scope = s.PreviewEnvironment
					}
					switch variable.State {
					case preview.Disabled:
						status = s.PreviewDisabled
					case preview.Shadowed:
						status = s.PreviewShadowed
					}
					ui.Wrap(c.Key("variable").KeyInt(i), fmt.Sprintf(s.PreviewVariableRow, variable.Name, scope, variable.Row, status))
				}
				ui.Text(c, s.PreviewUnresolved).Bold()
				if len(snapshot.Unresolved) == 0 {
					ui.Wrap(c, s.PreviewNoUnresolved).Muted()
				}
				for i, name := range snapshot.Unresolved {
					ui.Wrap(c.Key("unknown").KeyInt(i), "{{"+name+"}}").Color(a.palette.Warning)
				}
				ui.Divider(c)
				ui.Wrap(c, s.PreviewLimits).Muted().FontSize(theme.Metrics.FontSizeS)
			})
		})
	}, func() {
		if ui.Button(c, s.PreviewClose).Outline().Clicked() {
			state.previewOpen = false
		}
	})
}
