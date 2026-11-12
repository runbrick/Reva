package gui

import (
	"fmt"
	"strings"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/script"
	"github.com/runbrick/reqo/internal/vars"
	"github.com/vivaui/viva/ui"
)

// scriptsState is the scripts section's UI state. Owned by the scripts PR.
type scriptsState struct {
	tabs map[int]*scriptTabState
}

type scriptTabState struct {
	testsEditor   bool
	menuOpen      bool
	preEditor     scriptCodeEditor
	testEditor    scriptCodeEditor
	preError      string
	logs          []string
	summary       string
	summaryFormat string
	passed, total int
}

var scriptEditorChoices = []bool{false, true}

func (a *App) scriptTab(t *Tab) *scriptTabState {
	if a.scripts.tabs == nil {
		a.scripts.tabs = make(map[int]*scriptTabState)
	}
	// The shell has no feature close hook. Retire state for closed tabs here.
	for id := range a.scripts.tabs {
		found := false
		for _, open := range a.tabs {
			if open.ID == id {
				found = true
				break
			}
		}
		if !found {
			delete(a.scripts.tabs, id)
		}
	}
	s := a.scripts.tabs[t.ID]
	if s == nil {
		s = &scriptTabState{}
		a.scripts.tabs[t.ID] = s
	}
	return s
}

// prepareSend runs t's pre-request script, which may change the request
// and variables, just before sending. It runs on the UI thread.
func (a *App) prepareSend(t *Tab, req model.Request, vars []model.KV) (model.Request, []model.KV) {
	s := a.scriptTab(t)
	s.preError, s.logs = "", nil
	if strings.TrimSpace(req.PreScript) != "" {
		var err error
		vars, s.logs, err = script.RunPre(req.PreScript, &req, vars, 0)
		if err != nil {
			s.preError = err.Error()
		}
	}
	return req, vars
}

// afterResponse runs t's test script against resp and stores t.Tests.
func (a *App) afterResponse(t *Tab, resp *model.Response) {
	s := a.scriptTab(t)
	t.Tests = nil
	if s.preError != "" {
		t.Tests = append(t.Tests, TestResult{Name: a.S.Scripts.PreScriptError, Error: s.preError})
	}
	if resp == nil {
		return
	}
	req, values := t.Request.Clone(), a.Vars()
	if t.job != nil {
		req, values = t.job.req, t.job.vars
	}
	if strings.TrimSpace(req.TestScript) == "" {
		return
	}
	results, logs, err := script.RunTests(req.TestScript, vars.Apply(req, values), *resp, values, 0)
	s.logs = append(s.logs, logs...)
	for _, result := range results {
		t.Tests = append(t.Tests, TestResult{Name: result.Name, Passed: result.Passed, Error: result.Error})
	}
	if err != nil {
		t.Tests = append(t.Tests, TestResult{Name: a.S.Scripts.TestScriptError, Error: err.Error()})
	}
}

// scriptsSection declares the editors for t's pre-request and test
// scripts; the request editor shows it as one of its sections.
func (a *App) scriptsSection(c ui.C, t *Tab) {
	s := a.scriptTab(t)
	ui.Column(c).Fill().Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Choice(c, &s.testsEditor, scriptEditorChoices, func(tests bool) string {
				if tests {
					return a.S.Scripts.Tests
				}
				return a.S.Scripts.PreRequest
			})
			ui.Spacer(c)
			button := ui.Button(c, a.S.Scripts.Snippets).Outline()
			if button.Clicked() {
				s.menuOpen = !s.menuOpen
			}
			ui.Menu(c, button, &s.menuOpen, func(menu *ui.MenuItems) {
				if menu.Item(a.S.Scripts.StatusSnippetLabel).Clicked() {
					a.insertScript(t, s, a.S.Scripts.StatusSnippet)
				}
				if menu.Item(a.S.Scripts.TimeSnippetLabel).Clicked() {
					a.insertScript(t, s, a.S.Scripts.TimeSnippet)
				}
				if menu.Item(a.S.Scripts.VariableSnippetLabel).Clicked() {
					a.insertScript(t, s, a.S.Scripts.VariableSnippet)
				}
			})
		})
		ui.Wrap(c, a.S.Scripts.Help).Muted()
		text, label := &t.Request.PreScript, a.S.Scripts.PreRequest
		editor := &s.preEditor
		if s.testsEditor {
			text, label = &t.Request.TestScript, a.S.Scripts.Tests
			editor = &s.testEditor
		}
		if editor.declare(c, text, label, a.palette) {
			t.Dirty = true
		}
	})
}

// All offered snippets use the response API, so insert them in the test editor.
func (a *App) insertScript(t *Tab, s *scriptTabState, source string) {
	s.testsEditor = true
	if t.Request.TestScript != "" && !strings.HasSuffix(t.Request.TestScript, "\n") {
		t.Request.TestScript += "\n"
	}
	t.Request.TestScript += source + "\n"
	t.Dirty = true
}

// testsSection declares t.Tests; the response viewer shows it as one of
// its sections.
func (a *App) testsSection(c ui.C, t *Tab) {
	s := a.scriptTab(t)
	passed := 0
	for _, result := range t.Tests {
		if result.Passed {
			passed++
		}
	}
	if s.summary == "" || s.passed != passed || s.total != len(t.Tests) || s.summaryFormat != a.S.Scripts.PassedSummary {
		s.passed, s.total, s.summaryFormat = passed, len(t.Tests), a.S.Scripts.PassedSummary
		s.summary = fmt.Sprintf(s.summaryFormat, passed, len(t.Tests))
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, s.summary).Bold()
			if len(t.Tests) == 0 {
				ui.Wrap(c, a.S.Scripts.Empty).Muted()
			}
			for i, result := range t.Tests {
				row := c.KeyInt(i)
				ui.Column(row).FillWidth().Gap(4).Children(func() {
					ui.Row(row).FillWidth().Gap(8).Children(func() {
						marker, color := a.S.Scripts.FailMarker, c.Theme().Danger
						if result.Passed {
							marker, color = a.S.Scripts.PassMarker, c.Theme().Success
						}
						ui.Text(row, marker).Color(color)
						ui.Wrap(row, result.Name).Grow(1)
					})
					if result.Error != "" {
						ui.Wrap(row, result.Error).Color(c.Theme().Danger)
					}
				})
			}
			if len(s.logs) > 0 {
				ui.Divider(c)
				ui.Text(c, a.S.Scripts.Console).Bold()
				for i, line := range s.logs {
					ui.Wrap(c.KeyInt(i).Key("console"), line).Mono()
				}
			}
		})
	})
}
