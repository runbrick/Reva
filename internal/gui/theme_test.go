package gui

import (
	"context"
	"os"
	"path/filepath"
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

func TestInterfaceStylePersistsAndPreservesDraft(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	local, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := fake.New()
	svc.Settings = local.Settings()
	h := newHarness(t, svc)
	active := h.App.Active()
	active.Request.Name, active.Request.URL, active.Dirty = "Draft", "https://example.test/draft", true
	settings := h.App.S.Settings
	h.click(settings.Title)
	h.click(settings.ClassicStyle)
	settingsAssertSelected(t, h, settings.ClassicStyle)
	if !h.App.desktopLayout() || h.App.Dark() || svc.Settings.Get().Theme != "classic:light" {
		t.Fatal("classic style did not apply and persist")
	}
	h.click(settings.System)
	for _, appearance := range []app.Appearance{app.AppearanceDark, app.AppearanceLight} {
		h.D.SetSystemAppearance(appearance)
		settingsRepaint(t, h)
		if h.D.NativeAppearance() != appearance || !h.App.desktopLayout() || svc.Settings.Get().Theme != "classic:system" {
			t.Fatal("system appearance changed the saved style or failed to take effect")
		}
	}
	reloaded, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	restart := svc
	restart.Settings = reloaded.Settings()
	restarted := newHarness(t, restart)
	if !restarted.App.desktopLayout() || restart.Settings.Get().Theme != "classic:system" {
		t.Fatal("classic style and appearance were not restored from disk")
	}
	h.click(h.App.S.Titlebar.DarkTheme)
	if !h.App.Dark() || !h.App.desktopLayout() || svc.Settings.Get().Theme != "classic:dark" {
		t.Fatal("titlebar appearance toggle lost the classic style")
	}
	h.click(settings.ModernStyle)
	if h.App.desktopLayout() || !h.App.Dark() || svc.Settings.Get().Theme != "dark" {
		t.Fatal("switching back to modern lost dark appearance")
	}
	h.click(h.App.S.Shell.Workbench)
	if h.App.Active() != active || !active.Dirty || !h.shows(active.Request.URL) {
		t.Fatal("switching styles lost the open request draft")
	}
}

func TestInterfaceStyleSaveFailureKeepsCurrentStyle(t *testing.T) {
	svc := fake.New()
	svc.Settings = &settingsSaveFailure{Settings: svc.Settings.(*fake.Settings)}
	h := newHarness(t, svc)
	settings := h.App.S.Settings
	h.click(settings.Title)
	h.click(settings.ClassicStyle)
	if h.App.desktopLayout() || svc.Settings.Get() != model.DefaultSettings() || !h.shows("settings cannot be saved") {
		t.Fatal("failed settings save changed the style or did not report the failure")
	}
	settingsAssertSelected(t, h, settings.ModernStyle)
}

func TestClassicWorkbenchNavigationAndResponse(t *testing.T) {
	for _, variant := range []struct{ lang, mode string }{{"zh", "light"}, {"en", "dark"}} {
		t.Run(variant.lang+"/"+variant.mode, func(t *testing.T) {
			h := classicWorkbenchFixture(t, variant.lang, variant.mode)
			if err := h.D.Resize(960, 600); err != nil {
				t.Fatal(err)
			}
			h.click(h.App.S.Shell.History)
			if h.App.Main != PageHistory {
				t.Fatal("history was not reachable from the titlebar")
			}
			h.click(h.App.S.Shell.Environments)
			if h.App.Main != PageWorkbench || h.App.Section != SectionEnvironments {
				t.Fatal("environments were not reachable from the toolbar")
			}
			h.click(h.App.S.Shell.APIs)
			h.click(h.App.S.Request.Send)
			h.waitFor("the response", func() bool { return h.shows("200 OK") && !h.App.Active().Sending })
			h.click(h.App.S.Response.Raw)
			if view := h.App.response.views[h.App.Active().ID]; view.mode != responseRaw || !h.shows("success") {
				t.Fatal("classic response mode did not show the raw body")
			}
			status := false
			for _, label := range h.D.Snapshot().Labels {
				if label.Value == h.App.S.Shell.Ready && label.Bounds.Y >= 600-statusStripHeight(&h.App.theme) && label.Bounds.Y+label.Bounds.Height <= 600 {
					status = true
				}
			}
			if !status {
				t.Fatal("status strip is not visible at the minimum window size")
			}
		})
	}
}

func classicWorkbenchFixture(t *testing.T, lang, mode string) *harness {
	t.Helper()
	svc := fake.New()
	settings := model.DefaultSettings()
	settings.Language, settings.Theme = lang, "classic:"+mode
	if err := svc.Settings.Save(settings); err != nil {
		t.Fatal(err)
	}
	request := model.NewRequest("user-info")
	request.Name, request.URL = "Get user info", "https://api.example.test/user/info"
	request.Params = []model.KV{{Key: "userId", Value: "12345"}}
	collection := model.Collection{Version: model.SchemaVersion, ID: "demo", Name: "API examples", Items: []model.Item{
		{ID: request.ID, Name: request.Name, Request: &request},
	}}
	if err := svc.Collections.Save(collection); err != nil {
		t.Fatal(err)
	}
	svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		body := []byte(`{"code":0,"message":"success","data":{"id":12345,"name":"Reva"}}`)
		return model.Response{StatusCode: 200, Status: "200 OK", Body: body, Size: int64(len(body)), Duration: 56 * time.Millisecond}, nil
	}
	h := newHarness(t, svc)
	h.App.OpenRequest(request, collection.ID)
	settingsRepaint(t, h)
	return h
}

func TestClassicHeadlessCapture(t *testing.T) {
	dir := os.Getenv("REVA_THEME_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set REVA_THEME_CAPTURE_DIR to render theme screenshots")
	}
	for _, variant := range []struct {
		lang, mode    string
		width, height int
	}{{"zh", "light", 1440, 900}, {"en", "dark", 960, 600}} {
		h := classicWorkbenchFixture(t, variant.lang, variant.mode)
		h.click(h.App.S.Request.Send)
		h.waitFor("the response", func() bool { return h.shows("200 OK") && !h.App.Active().Sending })
		page := ui.NewPage(ui.PageConfig{View: h.App.Page, Theme: &h.App.theme})
		session, err := headless.Open(page.Config(app.Config{Width: variant.width, Height: variant.height}), nil, headless.Options{Scale: 1})
		if err != nil {
			t.Fatal(err)
		}
		err = session.Screenshot(filepath.Join(dir, "classic-"+variant.mode+".png"))
		session.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
