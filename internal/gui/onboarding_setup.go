package gui

import (
	"slices"

	"github.com/runbrick/reva/internal/model"
)

func (a *App) showOnboardingEntry(t *Tab) bool {
	if a.Main != PageWorkbench || a.Section != SectionAPIs || a.settings.guideOpenedThisSession || a.settings.guideOpen {
		return false
	}
	if t == nil || t.Kind != requestTabKind || t.CollectionID != "" || t.Dirty ||
		t.Sending || t.Response != nil || t.Err != "" || len(t.Tests) != 0 {
		return false
	}
	if scripts := a.scripts.tabs[t.ID]; scripts != nil && (scripts.preError != "" || len(scripts.logs) != 0) {
		return false
	}
	req := &t.Request
	blank := model.NewRequest(req.ID)
	return req.Method == blank.Method && req.Name == blank.Name && req.URL == blank.URL &&
		slices.Equal(req.Params, blank.Params) && slices.Equal(req.Headers, blank.Headers) &&
		req.Body.Mode == blank.Body.Mode && req.Body.Text == blank.Body.Text &&
		req.Body.ContentType == blank.Body.ContentType && req.Body.FilePath == blank.Body.FilePath &&
		slices.Equal(req.Body.Form, blank.Body.Form) && slices.Equal(req.Body.URLEncoded, blank.Body.URLEncoded) &&
		req.Auth == blank.Auth && req.Options == blank.Options &&
		req.PreScript == blank.PreScript && req.TestScript == blank.TestScript &&
		!a.Svc.Settings.Get().OnboardingCompleted
}

func (a *App) openOnboardingGuide() {
	a.settings.guideOpenedThisSession = true
	a.settings.guideOpen = true
	a.settings.guideError = ""
}

func (a *App) openOnboardingPractice(name string) {
	tab := a.NewTab()
	tab.Request.Name = name
	tab.Request.URL = "https://example.invalid/hello"
	tab.Request.Headers = []model.KV{{Key: "Accept", Value: "application/json"}}
	a.Main, a.Section = PageWorkbench, SectionAPIs
	a.requestTab(tab).previewOpen = true
}
