// Package gui is Reqo's user interface, written with Viva.
//
// The App owns the window layout and the send pipeline. Every feature lives
// in its own file and owns its own state struct and draw functions:
//
//	request.go      request editor (method, URL, params, headers, body, auth)
//	response.go     response viewer (status, time, size, headers, body)
//	history.go      history sidebar pane
//	collections.go  collections sidebar pane and "save request" dialog
//	env.go          environment selector, editor and Vars()
//	importer.go     import (Postman / curl) button and dialog
//	settings.go     settings button and dialog (theme, language, history limit)
//	tabs.go         request tab bar and OpenRequest
//	scripts.go      pre-request / test script hooks
//
// A feature PR edits its own file(s) and does not change app.go; if it needs
// a new hook from the shell, it says so in its PR description.
package gui

import (
	"context"
	"time"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// Tab is one open request and its latest result.
type Tab struct {
	ID       int
	Request  model.Request
	Response *model.Response
	// Err is the transport error of the last send, if any.
	Err     string
	Sending bool
	// CollectionID is the collection the request was opened from or saved
	// to; empty for an unsaved request.
	CollectionID string
	// Dirty is set by editors when the request changes after load/save.
	Dirty bool
	// Tests are the results of the last test script run (scripts.go).
	Tests []TestResult

	job  *sendJob
	send *ui.EffectState[sendResult, *sendJob]
}

// TestResult is one assertion reported by a test script.
type TestResult struct {
	Name   string
	Passed bool
	Error  string
}

type sendJob struct {
	req  model.Request
	vars []model.KV
}

type sendResult struct {
	resp model.Response
	err  error
}

type sideTab int

const (
	sideCollections sideTab = iota
	sideHistory
)

// App is the whole application's UI state.
type App struct {
	Svc service.Services
	// S is the active string table; replace it to switch language.
	S     *i18n.Strings
	theme ui.Theme

	tabs   []*Tab
	active int
	nextID int

	sideTab   sideTab
	sideSplit ui.SplitterState
	mainSplit ui.SplitterState
	toaster   ui.Toaster

	// Feature state, one struct per feature file.
	request     requestState
	response    responseState
	history     historyState
	collections collectionsState
	env         envState
	importer    importerState
	settings    settingsState
	tabBarState tabsState
	scripts     scriptsState
}

// New builds the App around svc, applying the stored settings.
func New(svc service.Services) *App {
	a := &App{Svc: svc}
	a.sideSplit = ui.SplitterState{Pos: 260}
	a.mainSplit = ui.SplitterState{Ratio: 0.5}
	a.ApplySettings(svc.Settings.Get())
	a.NewTab()
	return a
}

// ApplySettings switches language and theme to those in s.
func (a *App) ApplySettings(s model.Settings) {
	a.S = i18n.For(i18n.Lang(s.Language))
	if s.Theme == "dark" {
		a.theme = ui.DarkTheme()
	} else {
		a.theme = ui.LightTheme()
	}
}

// Config returns the window configuration for the App's page.
func (a *App) Config() app.Config {
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	return page.Config(app.Config{Title: a.S.Common.AppName, Width: 1280, Height: 800})
}

// Active returns the tab being edited.
func (a *App) Active() *Tab { return a.tabs[a.active] }

// Tabs returns the open tabs.
func (a *App) Tabs() []*Tab { return a.tabs }

// NewTab opens an empty request in a new tab and makes it active.
func (a *App) NewTab() *Tab {
	a.nextID++
	t := &Tab{ID: a.nextID, Request: model.NewRequest(storage.NewID())}
	t.send = ui.NewEffectKeyed[sendResult, *sendJob](a.run)
	a.tabs = append(a.tabs, t)
	a.active = len(a.tabs) - 1
	return t
}

// CloseTab closes tab i, keeping at least one tab open.
func (a *App) CloseTab(i int) {
	if i < 0 || i >= len(a.tabs) {
		return
	}
	a.tabs = append(a.tabs[:i:i], a.tabs[i+1:]...)
	if len(a.tabs) == 0 {
		a.NewTab()
	}
	if a.active >= len(a.tabs) || a.active > i {
		a.active = max(0, a.active-1)
	}
}

// Send starts sending t's request with the current variables. The result
// arrives on a later paint; Cancel stops it.
func (a *App) Send(t *Tab) {
	req, vars := a.prepareSend(t, t.Request.Clone(), a.Vars())
	t.job = &sendJob{req: req, vars: vars}
	t.Sending, t.Err, t.Response, t.Tests = true, "", nil, nil
}

// Cancel stops t's request in flight.
func (a *App) Cancel(t *Tab) {
	if t.Sending {
		t.Sending = false
		t.Err = context.Canceled.Error()
	}
}

// Toast shows a short message.
func (a *App) Toast(title, description string, kind ui.ToastKind) {
	a.toaster.Show(title, description, kind)
}

func (a *App) run(ctx context.Context, job *sendJob) (sendResult, error) {
	resp, err := a.Svc.Requests.Send(ctx, job.req, job.vars)
	return sendResult{resp: resp, err: err}, nil
}

// declareSends declares each sending tab's effect and applies results.
func (a *App) declareSends(c ui.C) {
	for _, t := range a.tabs {
		if !t.Sending {
			continue
		}
		ui.Effect(c, t.send, t.job)
		res, _, ok := t.send.Take()
		if !ok {
			continue
		}
		t.Sending = false
		entry := model.HistoryEntry{At: time.Now(), Request: t.Request.Clone()}
		if res.err != nil {
			t.Err = res.err.Error()
			entry.Error = t.Err
		} else {
			resp := res.resp
			t.Response = &resp
			entry.StatusCode, entry.Duration = resp.StatusCode, resp.Duration
			a.afterResponse(t, &resp)
		}
		if err := a.Svc.History.Add(entry); err != nil {
			a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
		}
	}
}

// Page declares the whole window.
func (a *App) Page(c ui.C) {
	a.declareSends(c)
	theme := c.Theme()
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		a.topBar(c)
		ui.Divider(c)
		ui.Row(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).Width(a.sideSplit.Pos).FillHeight().Background(theme.Card).Children(func() {
				a.sidebar(c)
			})
			ui.Splitter(c, &a.sideSplit).Mins(200, 480).Label("sidebar")
			ui.Column(c).Grow(1).FillHeight().Children(func() {
				a.tabBar(c)
				t := a.Active()
				ui.Column(c).FillWidth().Height(a.mainSplit.Pos).Children(func() {
					a.requestPane(c.KeyInt(t.ID), t)
				})
				ui.Splitter(c, &a.mainSplit).Mins(160, 160).Label("request/response")
				ui.Column(c).FillWidth().Grow(1).Children(func() {
					a.responsePane(c.KeyInt(t.ID), t)
				})
			})
		})
	})
	a.dialogs(c)
	ui.Toasts(c, &a.toaster)
}

func (a *App) topBar(c ui.C) {
	ui.Row(c).FillWidth().PadX(16).PadY(8).Gap(8).Items(ui.Center).Children(func() {
		ui.Text(c, a.S.Common.AppName).FontSize(16).Bold()
		ui.Spacer(c)
		a.envSelector(c)
		a.importButton(c)
		a.settingsButton(c)
	})
}

func (a *App) sidebar(c ui.C) {
	labels := func(s sideTab) string {
		if s == sideHistory {
			return a.S.History.Title
		}
		return a.S.Collections.Title
	}
	ui.Column(c).Fill().Pad(8).Gap(8).Children(func() {
		ui.Tabs(c, &a.sideTab, []sideTab{sideCollections, sideHistory}, labels).FillWidth()
		ui.Column(c).FillWidth().Grow(1).Children(func() {
			if a.sideTab == sideHistory {
				a.historyPane(c)
			} else {
				a.collectionsPane(c)
			}
		})
	})
}

// dialogs declares every feature's modal dialogs; each returns at once when closed.
func (a *App) dialogs(c ui.C) {
	a.collectionsDialogs(c)
	a.envDialog(c)
	a.importDialog(c)
	a.settingsDialog(c)
}
