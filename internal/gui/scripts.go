package gui

import (
	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/ui"
)

// scriptsState is the scripts section's UI state. Owned by the scripts PR.
type scriptsState struct{}

// prepareSend runs t's pre-request script, which may change the request
// and variables, just before sending. It runs on the UI thread.
func (a *App) prepareSend(t *Tab, req model.Request, vars []model.KV) (model.Request, []model.KV) {
	return req, vars
}

// afterResponse runs t's test script against resp and stores t.Tests.
func (a *App) afterResponse(t *Tab, resp *model.Response) {}

// scriptsSection declares the editors for t's pre-request and test
// scripts; the request editor shows it as one of its sections.
func (a *App) scriptsSection(c ui.C, t *Tab) {}

// testsSection declares t.Tests; the response viewer shows it as one of
// its sections.
func (a *App) testsSection(c ui.C, t *Tab) {}
