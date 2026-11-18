// Package gui is Reva's user interface, written with Viva.
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

	"github.com/runbrick/reva/internal/execution"
	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/gui/themes"
	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type tabKind uint8

const (
	requestTabKind tabKind = iota
	collectionSettingsTabKind
)

// Tab is a request with its latest result, or a collection/folder configuration.
type Tab struct {
	ID       int
	Kind     tabKind
	TargetID string // folder ID for configuration; empty means collection root
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
	send *ui.EffectState[sendResult, sendKey]
	sse  *sseState
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
	plan *execution.Plan
}

type sendKey struct {
	job    *sendJob
	stream *sseMailbox
}

type sendResult struct {
	job    *sendJob
	result execution.Result
}

// Section is what the left rail selects: the list shown in the sidebar.
type Section int

const (
	SectionAPIs         Section = iota // collections tree, the default
	SectionEnvironments                // environment list and editor
	SectionSettings                    // settings page
)

// Sections lists the rail items in display order.
var Sections = []Section{SectionAPIs, SectionEnvironments, SectionSettings}

// railParts names each rail item's hit identity.
var railParts = map[Section]string{
	SectionAPIs:         "apis",
	SectionEnvironments: "environments", SectionSettings: "settings",
}

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
	S           *i18n.Strings
	theme       ui.Theme
	palette     Palette
	themeStyle  themes.Style
	themeMode   string
	themeLayout themes.Layout
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
	websocket   websocketState
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
	a.themeStyle, a.themeMode = themes.Decode(s.Theme)
	a.syncTheme()
}

// Dark reports whether the App currently paints with the dark theme.
func (a *App) Dark() bool {
	switch a.themeMode {
	case "dark":
		return true
	case "system":
		return app.SystemAppearance() == app.AppearanceDark
	}
	return false
}

func (a *App) syncTheme() {
	dark := a.Dark()
	a.theme, a.palette, a.themeLayout = themes.Resolve(a.themeStyle, dark)
	if dark {
		app.SetNativeAppearance(app.AppearanceDark)
	} else {
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
	t.send = ui.NewEffectKeyed(a.run)
	a.tabs = append(a.tabs, t)
	a.active = len(a.tabs) - 1
	return t
}

// CloseTab closes tab i, keeping at least one tab open.
func (a *App) CloseTab(i int) {
	if i < 0 || i >= len(a.tabs) {
		return
	}
	delete(a.collections.settings, a.tabs[i].ID)
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
	if t == nil || t.Kind != requestTabKind {
		return
	}
	col, _ := a.Svc.Collections.Get(t.CollectionID)
	plan, err := execution.New(execution.Input{
		Request: t.Request, Collection: col, Variables: a.VarsFor(t), Settings: a.Svc.Settings.Get(),
	})
	if err != nil {
		a.authFailure(t, true)
		return
	}
	// Preserve immediate feedback and the previous response for script-free
	// invalid auth. Scripts can change auth variables, so those validate in Run.
	if t.Request.PreScript == "" && plan.ValidateAuth() != nil {
		a.authFailure(t, false)
		return
	}
	if t.sse != nil {
		t.sse.release()
		if t.sse.status != 0 {
			delete(a.response.views, t.ID)
		}
		t.sse = nil
	}
	t.job = &sendJob{req: plan.Request(), vars: plan.Variables(), plan: plan}
	t.Sending, t.Err, t.Response, t.Tests = true, "", nil, nil
	a.resetScripts(t)
}

// Cancel stops t's request in flight.
func (a *App) Cancel(t *Tab) {
	if t.Sending {
		if t.sse != nil {
			t.sse.release()
			t.sse.disconnected = true
		}
		t.Sending = false
		t.Err = context.Canceled.Error()
	}
}

// Toast shows a short message.
func (a *App) Toast(title, description string, kind ui.ToastKind) {
	a.toaster.Show(title, description, kind)
}

func (a *App) run(ctx context.Context, key sendKey) (sendResult, error) {
	if key.stream != nil {
		if sender, ok := a.Svc.Requests.(sseSender); ok {
			adapter := &ssePlanSender{sender: sender, stream: key.stream}
			result := key.job.plan.Run(ctx, adapter)
			if result.Response == nil {
				result.Response = adapter.partial
			}
			return sendResult{job: key.job, result: result}, nil
		}
	}
	return sendResult{job: key.job, result: key.job.plan.Run(ctx, a.Svc.Requests)}, nil
}

// declareSends declares each sending tab's effect and applies results.
func (a *App) declareSends(c ui.C) {
	a.pruneResponseViews()
	for _, t := range a.tabs {
		if !t.Sending {
			continue
		}
		tabContext := c.KeyInt(t.ID)
		key := sendKey{job: t.job, stream: a.declareSSE(tabContext, t)}
		ui.Effect(tabContext, t.send, key)
		res, _, ok := t.send.Take()
		if !ok || res.job != t.job {
			continue
		}
		t.Sending = false
		r := res.result
		if t.sse != nil {
			t.sse.finish(r.Err)
			if t.sse.status == 0 {
				t.sse = nil
			}
		}
		a.applyScripts(t, r)
		entry := model.HistoryEntry{At: time.Now(), Request: r.HistoryRequest}
		if r.Err != nil {
			t.Err = r.Err.Error()
			if r.Stage == execution.StageValidation {
				a.authFailure(t, false)
			}
			entry.Error = t.Err
		} else if r.Pre.Err != nil {
			t.Err = a.S.Scripts.PreScriptError
		}
		if r.Response != nil && (r.Err == nil || t.sse != nil) {
			t.Response = r.Response
			entry.StatusCode, entry.Duration = r.Response.StatusCode, r.Response.Duration
		}
		if !r.TransportStarted {
			continue
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
	if a.themeMode == "system" {
		a.syncTheme()
	}
	a.declareSends(c)
	a.declareWebSockets(c)
	a.declareCollectionRunner(c)
	if a.collections.runner.open {
		a.collectionRunnerPage(c)
		return
	}
	// Preview hides raw background data without moving asynchronous work
	// into a different declaration scope or changing independent UI state.
	previewing := a.requestPreviewOpen()
	theme := c.Theme()
	classic := a.desktopLayout()
	height := c.Bounds().Height - TitlebarHeight
	if classic {
		height -= desktopToolbarHeight
		if !previewing {
			height -= statusStripHeight(theme)
		}
	}
	ui.Column(c).Fill().Background(a.palette.Surface).Children(func() {
		a.titlebarStrip(c)
		ui.Row(c).FillWidth().Height(max(0, height)).Shrink(0).Children(func() {
			if !classic {
				a.rail(c)
			}
			switch a.Main {
			case PageHistory:
				ui.Column(c).Grow(1).FillHeight().Children(func() { a.historyPage(c) })
			case PageSettings:
				ui.Column(c).Grow(1).FillHeight().Children(func() { a.settingsPage(c) })
			default:
				ui.Column(c).Width(a.sideSplit.Pos).FillHeight().Background(a.palette.Sidebar).Children(func() {
					if !previewing {
						sidebar := ui.Column(c).Fill()
						if !classic {
							sidebar.PadBottom(statusStripHeight(theme))
						}
						sidebar.Children(func() { a.sidebar(c) })
						if !classic {
							a.statusStrip(c, false)
						}
					}
				})
				ui.Splitter(c, &a.sideSplit).Mins(220, 640).Label(a.S.Shell.SidebarLabel)
				a.workbench(c, theme)
			}
		})
		if classic && !previewing {
			a.statusStrip(c, true)
		}
	})
	if !previewing {
		a.dialogs(c)
		a.searchPalette(c)
		ui.Toasts(c, &a.toaster)
	}
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
				fg = theme.AccentForeground
			}
			// The hit's identity is named by its section's word; a Key set
			// before ui.ID would be replaced by a Part, giving every item one ID.
			id := ui.ID(c.Part(railParts[s]))
			item := ui.Column(c.KeyInt(int(s))).Width(RailWidth-2*Space1).Height(Space6*3).Center().Gap(Space2).
				Radius(theme.Metrics.RadiusControl).HitWith(id, label, gfx.HitOptions{Role: gfx.RoleButton, Selected: active}).Clip().
				Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring.WithAlpha(0.5)))
			if active {
				item.Background(a.palette.RailActive)
			}
			item.Children(func() {
				ui.Icon(c, icon).Size(Space6, Space6).Color(fg)
				ui.Text(c, label).FontSize(theme.Metrics.FontSizeS - 2).Color(fg)
				if active {
					ui.Slot(c).Absolute().Left(0).Top(0).Bottom(0).Width(Space1).Background(theme.Primary)
				}
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
	case SectionEnvironments:
		return a.S.Shell.Environments
	case SectionSettings:
		return a.S.Shell.Settings
	}
	return a.S.Shell.APIs
}

func (a *App) sectionIcon(s Section) gfx.Icon {
	switch s {
	case SectionEnvironments:
		return icons.Package
	case SectionSettings:
		return icons.Settings
	}
	return icons.Send
}

// sidebar shows the selected section's list.
func (a *App) sidebar(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		switch a.Section {
		case SectionEnvironments:
			a.envSidebar(c)
		default:
			a.collectionsPane(c)
		}
	})
}

func statusStripHeight(theme *ui.Theme) float32 { return theme.Metrics.ControlHeight + 1 }

func (a *App) statusStrip(c ui.C, fullWidth bool) {
	theme := c.Theme()
	label, color := a.S.Shell.Ready, theme.Success
	if a.Active().Sending {
		label, color = a.S.Response.Sending, theme.AccentForeground
	}
	strip := ui.Column(c).Height(statusStripHeight(theme)).Shrink(0)
	if fullWidth {
		strip.FillWidth().Background(a.palette.Rail)
	} else {
		strip.Absolute().Left(0).Right(0).Bottom(0).Background(a.palette.Sidebar)
	}
	strip.Children(func() {
		ui.Divider(c)
		ui.Row(c).FillWidth().Grow(1).PadX(Space4).Gap(Space2).Items(ui.Center).Children(func() {
			statusDot(c, color)
			ui.Text(c, label).FontSize(theme.Metrics.FontSizeS).Muted().Ellipsis()
		})
	})
}

// workbench is the tab bar over the request editor and the response. While
// the Environments section is selected, the env PR's editor may take the
// whole area instead.
func (a *App) workbench(c ui.C, theme *ui.Theme) {
	previewing := a.requestPreviewOpen()
	if a.Section == SectionEnvironments && a.envReady() {
		ui.Column(c).Grow(1).FillHeight().Background(a.palette.Surface).Children(func() { a.envMain(c) })
		return
	}
	ui.Column(c).Grow(1).FillHeight().Background(a.palette.Surface).Children(func() {
		a.tabBar(c)
		t := a.Active()
		if t.Kind == collectionSettingsTabKind {
			// Continue already authorized request downloads while editing settings.
			a.requestDownloads(c.KeyInt(t.ID))
			a.collectionSettingsPane(c.KeyInt(t.ID), t)
			return
		}
		if isWebSocket(t.Request) {
			a.websocketPane(c.KeyInt(t.ID), t)
			return
		}
		a.workbenchPanel(c, ui.Column(c).FillWidth().Height(a.mainSplit.Pos), !previewing, func() {
			a.requestPane(c.KeyInt(t.ID), t)
		})
		ui.Splitter(c, &a.mainSplit).Mins(180, 180).Label(a.S.Shell.SplitLabel)
		a.workbenchPanel(c, ui.Column(c).FillWidth().Grow(1), !previewing, func() {
			if previewing {
				return
			}
			if t.Response == nil && !t.Sending && (len(t.Tests) > 0 || len(a.scriptTab(t).logs) > 0) {
				a.scriptFailurePane(c.KeyInt(t.ID), t)
			} else {
				a.responsePane(c.KeyInt(t.ID), t)
			}
		})
	})
}

func (a *App) workbenchPanel(c ui.C, panel ui.Box, visible bool, content func()) {
	if visible && a.desktopLayout() {
		theme := c.Theme()
		panel.MarginX(Space1).MarginY(Space1).Background(theme.Card).
			Border(1, theme.Border).Radius(theme.Metrics.RadiusSurface).Clip()
	}
	panel.Children(content)
}

// dialogs declares every feature's modal dialogs; each returns at once when closed.
func (a *App) dialogs(c ui.C) {
	a.collectionsDialogs(c)
	a.envDialog(c)
	a.importDialog(c)
	a.settingsDialog(c)
}
