package gui

import "github.com/runbrick/reva/internal/model"

func (a *App) openOnboardingPractice(name string) {
	tab := a.NewTab()
	tab.Request.Name = name
	tab.Request.URL = "https://example.invalid/hello"
	tab.Request.Headers = []model.KV{{Key: "Accept", Value: "application/json"}}
	a.Main, a.Section = PageWorkbench, SectionAPIs
	a.requestTab(tab).previewOpen = true
}
