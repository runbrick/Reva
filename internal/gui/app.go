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
	"runtime"
	"time"

	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
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

// Section is what the left rail selects: the list shown in the sidebar.
type Section int

const (
	SectionAPIs         Section = iota // collections tree, the default
	SectionCollections                 // collection management overview
	SectionEnvironments                // environment list and editor
	SectionSettings                    // settings page
)

// Sections lists the rail items in display order.
var Sections = []Section{SectionAPIs, SectionCollections, SectionEnvironments, SectionSettings}

// MainPage is what the titlebar navigation selects: the main area's content.
type MainPage int

const (
	PageWorkbench MainPage = iota // sidebar + request tabs + editor + response
	PageHistory                   // full-width history
	PageSettings                  // full-width settings
)

// Pages lists the titlebar navigation items in display order.
var Pages = []MainPage{PageWorkbench, PageHistory, PageSettings}

// App is the whole application's UI state.
type App struct {
	Svc service.Services
	// S is the active string table; replace it to switch language.
	S       *i18n.Strings
	theme   ui.Theme
	palette Palette
	// themeName is the stored theme choice ("light", "dark" or "system").
	themeName string
	// Platform is the OS the window runs on ("darwin", "windows", "linux");
	// tests set it to check each platform's titlebar.
	Platform string

	tabs   []*Tab
	active int
	nextID int

	// Section is the rail selection, Main the titlebar selection. Feature
	// code may set them (e.g. a settings shortcut sets Main = PageSettings).
	Section Section
	Main    MainPage

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
	titlebar    titlebarState
	search      searchState
}

// New builds the App around svc, applying the stored settings.
func New(svc service.Services) *App {
	a := &App{Svc: svc, Platform: runtime.GOOS}
	a.sideSplit = ui.SplitterState{Pos: SidebarWidth}
	a.mainSplit = ui.SplitterState{Ratio: 0.5}
	a.ApplySettings(svc.Settings.Get())
	a.NewTab()
	return a
}

// ApplySettings switches language and theme to those in s. Theme "system"
// follows the OS appearance (re-read every paint by syncTheme).
func (a *App) ApplySettings(s model.Settings) {
	a.S = i18n.For(i18n.Lang(s.Language))
	a.themeName = s.Theme
	a.syncTheme()
}

// Dark reports whether the App currently paints with the dark theme.
func (a *App) Dark() bool {
	switch a.themeName {
	case "dark":
		return true
	case "system":
		return app.SystemAppearance() == app.AppearanceDark
	}
	return false
}

func (a *App) syncTheme() {
	if a.Dark() {
		a.theme, a.palette = darkTheme()
		app.SetNativeAppearance(app.AppearanceDark)
	} else {
		a.theme, a.palette = lightTheme()
		app.SetNativeAppearance(app.AppearanceLight)
	}
}

// Config returns the window configuration for the App's page: a custom
// titlebar the app paints itself (titlebar.go), on every platform.
func (a *App) Config() app.Config {
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	return page.Config(a.windowConfig(app.Config{
		Title: a.S.Common.AppName, Width: 1440, Height: 900, MinWidth: 960, MinHeight: 600,
	}))
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

// Page declares the whole window:
//
//	+----------------------------------------------------------------+
//	| titlebar: window controls, logo, page nav, search, env, user   |  titlebar.go
//	+------+-----------------+---------------------------------------+
//	| rail | sidebar (list)  | main: tabs / request / response       |
//	|      |                 |   or the History / Settings page      |
//	+------+-----------------+---------------------------------------+
func (a *App) Page(c ui.C) {
	if a.themeName == "system" {
		a.syncTheme()
	}
	a.declareSends(c)
	theme := c.Theme()
	ui.Column(c).Fill().Background(a.palette.Surface).Children(func() {
		a.titlebarStrip(c)
		ui.Row(c).FillWidth().Grow(1).Children(func() {
			a.rail(c)
			switch a.Main {
			case PageHistory:
				ui.Column(c).Grow(1).FillHeight().Children(func() { a.historyPage(c) })
			case PageSettings:
				ui.Column(c).Grow(1).FillHeight().Children(func() { a.settingsPage(c) })
			default:
				ui.Column(c).Width(a.sideSplit.Pos).FillHeight().Background(a.palette.Sidebar).Children(func() {
					a.sidebar(c)
				})
				ui.Splitter(c, &a.sideSplit).Mins(220, 640).Label(a.S.Shell.SidebarLabel)
				a.workbench(c, theme)
			}
		})
	})
	a.dialogs(c)
	a.searchPalette(c)
	ui.Toasts(c, &a.toaster)
}

// rail is the left icon navigation. It selects Section, and on the
// Settings item the Settings page.
func (a *App) rail(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Width(RailWidth).FillHeight().Background(a.palette.Rail).PadY(Space3).Gap(Space2).
		Items(ui.Center).FocusGroup(ui.FocusVertical).Label(a.S.Shell.RailLabel).Children(func() {
		for _, s := range Sections {
			icon, label := a.sectionIcon(s), a.sectionLabel(s)
			active := a.Main == PageWorkbench && a.Section == s || s == SectionSettings && a.Main == PageSettings
			fg := theme.MutedForeground
			if active {
				fg = theme.Primary
			}
			kc := c.KeyInt(int(s))
			item := ui.Column(kc).Width(56).PadY(Space2).Gap(Space1).Items(ui.Center).
				Radius(theme.Metrics.RadiusSurface).Hit(ui.ID(kc.Part("rail")), label).
				Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring.WithAlpha(0.5)))
			if active {
				item.Background(a.palette.RailActive)
			}
			item.Children(func() {
				ui.Icon(c, icon).Size(22, 22).Color(fg)
				ui.Text(c, label).FontSize(theme.Metrics.FontSizeS).Color(fg)
			})
			if item.Clicked() {
				a.SelectSection(s)
			}
		}
	})
	ui.Divider(c).Vertical()
}

// pageLabel names a titlebar navigation item.
func (a *App) pageLabel(p MainPage) string {
	switch p {
	case PageHistory:
		return a.S.Shell.History
	case PageSettings:
		return a.S.Shell.Settings
	}
	return a.S.Shell.Workbench
}

// SelectSection shows section s: the Settings item opens the Settings page,
// the others the workbench with that section's list in the sidebar.
func (a *App) SelectSection(s Section) {
	if s == SectionSettings {
		a.Main = PageSettings
		return
	}
	a.Section, a.Main = s, PageWorkbench
}

func (a *App) sectionLabel(s Section) string {
	switch s {
	case SectionCollections:
		return a.S.Shell.Collections
	case SectionEnvironments:
		return a.S.Shell.Environments
	case SectionSettings:
		return a.S.Shell.Settings
	}
	return a.S.Shell.APIs
}

func (a *App) sectionIcon(s Section) gfx.Icon {
	switch s {
	case SectionCollections:
		return icons.Package
	case SectionEnvironments:
		return icons.Globe
	case SectionSettings:
		return icons.Settings
	}
	return icons.Layers
}

// sidebar shows the selected section's list.
func (a *App) sidebar(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		switch a.Section {
		case SectionEnvironments:
			a.envSidebar(c)
		case SectionCollections:
			a.collectionsOverview(c)
		default:
			a.collectionsPane(c)
		}
	})
	ui.Divider(c).Vertical()
}

// workbench is the tab bar over the request editor and the response.
func (a *App) workbench(c ui.C, theme *ui.Theme) {
	ui.Column(c).Grow(1).FillHeight().Background(a.palette.Surface).Children(func() {
		a.tabBar(c)
		t := a.Active()
		ui.Column(c).FillWidth().Height(a.mainSplit.Pos).Children(func() {
			a.requestPane(c.KeyInt(t.ID), t)
		})
		ui.Splitter(c, &a.mainSplit).Mins(180, 180).Label(a.S.Shell.SplitLabel)
		ui.Column(c).FillWidth().Grow(1).Children(func() {
			a.responsePane(c.KeyInt(t.ID), t)
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
