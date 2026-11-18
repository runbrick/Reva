package gui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestStudioWorkbenchKeepsControlsAtMinimumWidth(t *testing.T) {
	for _, variant := range []struct{ lang, mode string }{{"zh", "light"}, {"en", "dark"}} {
		t.Run(variant.lang+"/"+variant.mode, func(t *testing.T) {
			h := studioFixture(t, variant.lang, variant.mode)
			studioResize(t, h, 1586, 992)
			labels := h.App.S
			if !h.shows(labels.Request.Details) {
				t.Fatal("wide workbench is missing request details")
			}
			h.click(labels.Request.Preview)
			if !h.shows(labels.Request.PreviewTitle) || h.shows(labels.Request.Details) {
				t.Fatal("read-only preview did not replace the request details")
			}
			h.click(labels.Request.PreviewClose)
			studioResize(t, h, 960, 600)
			if h.shows(labels.Request.Details) || !h.shows("12345") {
				t.Fatal("compact layout did not drop the inspector and keep the parameters visible")
			}
			for _, label := range []string{labels.Request.URL, labels.Request.SendAccessible, labels.Request.Save} {
				hit := collectionLayoutHit(t, h, label)
				if hit.Bounds.Width < Space6 || hit.Bounds.X+hit.Bounds.Width > 960 {
					t.Fatalf("control %q is clipped: %+v", label, hit.Bounds)
				}
			}
			h.click(labels.Request.SendAccessible)
			h.waitFor("response", func() bool { return h.shows("200 OK") && !h.App.Active().Sending })
			h.click(labels.Response.Raw)
			if h.App.response.views[h.App.Active().ID].mode != responseRaw || !h.shows("success") {
				t.Fatal("compact workbench did not switch to the raw response")
			}
			h.click(labels.Shell.History)
			if h.App.Main != PageHistory {
				t.Fatal("rail did not open history")
			}
		})
	}
}

func TestStudioEnvironmentFilterAndSave(t *testing.T) {
	h := studioFixture(t, "en", "light")
	studioResize(t, h, 1586, 992)
	labels := h.App.S.Env
	h.click(h.App.S.Shell.Environments)
	h.typeInto(labels.Filter, "prod")
	if slices.Contains(h.hitLabels(), "Development") || !slices.Contains(h.hitLabels(), "Production") || !h.shows(labels.Details) {
		t.Fatal("the environment filter or inspector did not update")
	}
	h.click("Production")
	h.click(labels.SetActive)
	if h.Svc.Settings.Get().ActiveEnvID != "prod" {
		t.Fatal("inspector did not activate the selected environment")
	}
	collectionsReplace(h, h.App.S.Common.Value, "https://api.example.test/v2")
	h.click(labels.Save)
	env, _ := h.Svc.Environments.Get("prod")
	if env.Variables[0].Value != "https://api.example.test/v2" || h.App.env.current().dirty() {
		t.Fatal("variable edit was not saved")
	}
}

func TestStudioSettingsPreviewPreservesDraft(t *testing.T) {
	h := studioFixture(t, "en", "classic:light")
	studioResize(t, h, 1586, 992)
	draft := h.App.Active()
	draft.Dirty = true
	labels := h.App.S.Settings
	h.click(labels.Title)
	if !h.shows(labels.ThemePreview) || h.shows("Interface style") || h.shows("Classic desktop") {
		t.Fatal("appearance page should offer only the current layout's theme controls")
	}
	h.click(fmt.Sprintf(labels.PreviewTheme, labels.Dark))
	if !h.App.Dark() || h.Svc.Settings.Get().Theme != "dark" {
		t.Fatal("preview tile did not persist the selected appearance")
	}
	h.click(labels.Requests)
	if !h.shows(labels.Timeout) || h.shows(labels.ThemePreview) {
		t.Fatal("settings navigation did not switch sections")
	}
	h.click(h.App.S.Shell.APIs)
	if h.App.Active() != draft || !draft.Dirty {
		t.Fatal("changing appearance lost the open request draft")
	}
}

func TestLegacyAppearanceLoadsInStudio(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	local, err := store.Open(dir)
	authMust(t, err)
	svc := fake.New()
	svc.Settings = local.Settings()
	settings := svc.Settings.Get()
	settings.Theme = "classic:system"
	authMust(t, svc.Settings.Save(settings))
	h := newHarness(t, svc)
	for _, appearance := range []app.Appearance{app.AppearanceDark, app.AppearanceLight} {
		h.D.SetSystemAppearance(appearance)
		settingsRepaint(t, h)
		if h.D.NativeAppearance() != appearance || svc.Settings.Get().Theme != "classic:system" {
			t.Fatal("loading legacy settings did not preserve system appearance")
		}
	}
	h.click(h.App.S.Titlebar.DarkTheme)
	reloaded, err := store.Open(dir)
	authMust(t, err)
	svc.Settings = reloaded.Settings()
	restarted := newHarness(t, svc)
	if !restarted.App.Dark() || svc.Settings.Get().Theme != "dark" {
		t.Fatal("appearance change did not persist as a plain mode")
	}
	studioResize(t, restarted, 1586, 992)
	if !restarted.shows(restarted.App.S.Request.Details) {
		t.Fatal("legacy settings did not open in the current workbench")
	}
}

func studioFixture(t *testing.T, lang, mode string) *harness {
	t.Helper()
	svc := fake.New()
	settings := model.DefaultSettings()
	settings.Language, settings.Theme = lang, mode
	authMust(t, svc.Settings.Save(settings))
	request := model.NewRequest("user-info")
	request.Name, request.URL = "Get user info", "https://api.example.test/user/info"
	request.Params = []model.KV{{Key: "userId", Value: "12345"}}
	collection := model.Collection{Version: model.SchemaVersion, ID: "demo", Name: "API examples", Items: []model.Item{
		{ID: request.ID, Name: request.Name, Type: model.ItemRequest, Request: &request},
	}}
	authMust(t, svc.Collections.Save(collection))
	svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		return studioResponseFixture(), nil
	}
	h := newHarness(t, svc)
	h.App.OpenRequest(request, collection.ID)
	for _, env := range []model.Environment{
		{ID: "prod", Name: "Production", Variables: []model.KV{{Key: "baseURL", Value: "https://api.example.test", Description: "API base URL"}}},
		{ID: "dev", Name: "Development", Variables: []model.KV{{Key: "baseURL", Value: "https://dev.example.test"}}},
	} {
		authMust(t, h.Svc.Environments.Save(env))
	}
	return h
}

func studioResponseFixture() model.Response {
	body := []byte(`{"code":0,"message":"success","data":{"id":12345,"name":"Reva"}}`)
	return model.Response{StatusCode: 200, Status: "200 OK", Body: body, Size: int64(len(body)), Duration: 56 * time.Millisecond}
}

func studioResize(t *testing.T, h *harness, width, height int) {
	t.Helper()
	authMust(t, h.D.Resize(width, height))
	h.pump()
}

func TestStudioHeadlessCapture(t *testing.T) {
	dir := os.Getenv("REVA_THEME_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set REVA_THEME_CAPTURE_DIR to render theme screenshots")
	}
	for _, variant := range []struct {
		name, mode    string
		section       Section
		page          MainPage
		width, height int
	}{
		{"workbench", "light", SectionAPIs, PageWorkbench, 1586, 992},
		{"settings", "dark", SectionSettings, PageSettings, 1586, 992},
		{"environments", "light", SectionEnvironments, PageWorkbench, 1586, 992},
		{"compact", "dark", SectionAPIs, PageWorkbench, 960, 600},
	} {
		h := studioFixture(t, "zh", variant.mode)
		env := h.Svc.Settings.Get()
		env.ActiveEnvID = "dev"
		authMust(t, h.Svc.Settings.Save(env))
		h.App.Section, h.App.Main = variant.section, variant.page
		response := studioResponseFixture()
		h.App.Active().Response = &response
		page := ui.NewPage(ui.PageConfig{View: h.App.Page, Theme: &h.App.theme})
		session, err := headless.Open(page.Config(h.App.windowConfig(app.Config{Width: variant.width, Height: variant.height})), nil, headless.Options{Scale: 1})
		authMust(t, err)
		path := filepath.Join(dir, "studio-"+variant.name+".png")
		err = session.Screenshot(path)
		session.Close()
		authMust(t, err)
		t.Log(path)
	}
}
