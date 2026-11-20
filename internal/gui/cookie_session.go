package gui

import (
	"github.com/runbrick/reva/internal/service/request"
	"github.com/vivaui/viva/ui"
)

type manualCookieScope struct {
	collectionID, environmentID string
}

type manualCookieState struct {
	enabled  bool
	sessions map[manualCookieScope]*request.CookieSession
}

func (a *App) manualCookieSessionFor(t *Tab) *request.CookieSession {
	state := &a.request.cookies
	if !state.enabled {
		return nil
	}
	col, _ := a.Svc.Collections.Get(t.CollectionID)
	env, _ := a.activeEnvironment(col)
	scope := manualCookieScope{collectionID: t.CollectionID, environmentID: env.ID}
	if state.sessions == nil {
		state.sessions = make(map[manualCookieScope]*request.CookieSession)
	}
	if state.sessions[scope] == nil {
		state.sessions[scope] = request.NewCookieSession()
	}
	return state.sessions[scope]
}

func (a *App) requestCookieControls(c ui.C) {
	state := &a.request.cookies
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		if ui.Checkbox(c, &state.enabled, a.S.Request.CookieSessionEnabled).Grow(1).Changed() && !state.enabled {
			state.sessions = nil
		}
		if ui.Button(c, a.S.Request.CookieSessionClear).Small().Outline().Disabled(!state.enabled).Clicked() {
			state.sessions = nil
		}
	})
}
