package gui

import (
	"context"
	"reflect"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
)

func TestOnboardingEntryRequiresUntouchedDefaultRequest(t *testing.T) {
	h := newHarness(t)
	a := h.App
	for range 2 {
		if !a.showOnboardingEntry(a.Active()) {
			t.Fatal("querying eligibility must leave the first blank tab eligible")
		}
	}
	blank := a.NewTab()
	blank.Request.Params = []model.KV{}
	blank.Request.Headers = []model.KV{}
	blank.Request.Body.Form = []model.FormField{}
	blank.Request.Body.URLEncoded = []model.KV{}
	if !a.showOnboardingEntry(blank) || blank.Request.Params == nil || blank.Request.Headers == nil ||
		blank.Request.Body.Form == nil || blank.Request.Body.URLEncoded == nil {
		t.Fatal("a later blank tab with non-nil empty slices must be eligible without rewriting the request")
	}
	for _, test := range []struct {
		name   string
		change func(*Tab)
	}{
		{"collection settings", func(tab *Tab) { tab.Kind = collectionSettingsTabKind }},
		{"saved request", func(tab *Tab) { tab.CollectionID = "collection" }},
		{"edited and cleared", func(tab *Tab) { tab.Dirty = true }},
		{"sending", func(tab *Tab) { tab.Sending = true }},
		{"response", func(tab *Tab) { tab.Response = &model.Response{} }},
		{"error", func(tab *Tab) { tab.Err = "send failed" }},
		{"test results", func(tab *Tab) { tab.Tests = []TestResult{{Passed: true}} }},
		{"name", func(tab *Tab) { tab.Request.Name = "Draft" }},
		{"URL", func(tab *Tab) { tab.Request.URL = "https://example.invalid/" }},
		{"other method", func(tab *Tab) { tab.Request.Method = model.MethodPost }},
		{"WebSocket", func(tab *Tab) { tab.Request.Method = webSocketMethod }},
		{"parameter", func(tab *Tab) { tab.Request.Params = []model.KV{{Key: "page"}} }},
		{"disabled header", func(tab *Tab) { tab.Request.Headers = []model.KV{{Key: "Accept", Disabled: true}} }},
		{"body mode", func(tab *Tab) { tab.Request.Body.Mode = model.BodyJSON }},
		{"retained body text", func(tab *Tab) { tab.Request.Body.Text = "draft" }},
		{"retained form", func(tab *Tab) { tab.Request.Body.Form = []model.FormField{{Key: "file"}} }},
		{"auth", func(tab *Tab) { tab.Request.Auth.Type = model.AuthInherit }},
		{"retained credential", func(tab *Tab) { tab.Request.Auth.Token = "draft" }},
		{"request options", func(tab *Tab) { tab.Request.Options.TimeoutMs = 1000 }},
		{"pre-request script", func(tab *Tab) { tab.Request.PreScript = "// draft" }},
		{"test script", func(tab *Tab) { tab.Request.TestScript = "// draft" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			tab := *blank
			test.change(&tab)
			if a.showOnboardingEntry(&tab) {
				t.Fatal("onboarding was offered for a non-blank or used request")
			}
		})
	}
	if a.showOnboardingEntry(nil) {
		t.Fatal("onboarding was offered without a tab")
	}
	scripts := a.scriptTab(blank)
	scripts.logs = []string{"script output"}
	if a.showOnboardingEntry(blank) {
		t.Fatal("onboarding was offered alongside script output")
	}
	scripts.logs = nil
	a.Section = SectionEnvironments
	if a.showOnboardingEntry(blank) {
		t.Fatal("onboarding was offered in Environments")
	}
	a.Main, a.Section = PageSettings, SectionAPIs
	if a.showOnboardingEntry(blank) {
		t.Fatal("the workbench entry was offered on the Settings page")
	}
}

func TestOnboardingEntryOpenSuppressesOnlyCurrentSession(t *testing.T) {
	svc := fake.New()
	settings := &onboardingSettingsProbe{SettingsService: svc.Settings}
	svc.Settings = settings
	svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		t.Error("opening the guide must not send a request")
		return model.Response{}, nil
	}
	h := newHarness(t, svc)
	a := h.App
	tab := a.Active()
	wantRequest := tab.Request
	wantSettings := svc.Settings.Get()
	a.settings.guideError = "stale error"
	a.openOnboardingGuide()
	if !a.settings.guideOpen || a.settings.guideError != "" || a.showOnboardingEntry(tab) {
		t.Fatal("opening must show the guide, clear the stale error and suppress the entry")
	}
	if a.Main != PageWorkbench || a.Section != SectionAPIs || a.Active() != tab || len(a.Tabs()) != 1 ||
		!reflect.DeepEqual(tab.Request, wantRequest) || tab.Sending {
		t.Fatal("opening the guide changed navigation, tabs or the draft")
	}
	if err := h.D.Resize(960, 600); err != nil {
		t.Fatal(err)
	}
	h.click(a.S.Settings.GuideSkip)
	if a.settings.guideOpen || a.showOnboardingEntry(tab) || a.showOnboardingEntry(a.NewTab()) {
		t.Fatal("skipping must close the guide and keep the entry suppressed for all tabs this session")
	}
	if settings.saves != 0 || svc.Settings.Get() != wantSettings || len(svc.History.List()) != 0 {
		t.Fatal("opening and skipping the guide saved settings or added history")
	}
	a = newHarness(t, svc).App
	if !a.showOnboardingEntry(a.Active()) {
		t.Fatal("skipping persisted suppression across sessions")
	}
	wantSettings.OnboardingCompleted = true
	if err := svc.Settings.Save(wantSettings); err != nil {
		t.Fatal(err)
	}
	if a.showOnboardingEntry(a.Active()) {
		t.Fatal("persisted completion must suppress the entry without visiting Settings")
	}
}
