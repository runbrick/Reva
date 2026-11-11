package gui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func envReplaceField(h *harness, label, value string) {
	h.t.Helper()
	h.click(label)
	if handled, err := h.D.Shortcut('a'); err != nil || !handled {
		h.t.Fatalf("select field text: handled=%v err=%v", handled, err)
	}
	if err := h.D.Type(value); err != nil {
		h.t.Fatal(err)
	}
}

func envSelect(h *harness, name string) {
	h.t.Helper()
	h.click(h.App.S.Env.Title)
	h.click(name)
}

func envCaptureRequests(h *harness) <-chan model.Request {
	h.t.Helper()
	received := make(chan model.Request, 8)
	h.Svc.Requests.(*fake.Requests).Fn = func(_ context.Context, req model.Request) (model.Response, error) {
		received <- req
		return model.Response{StatusCode: 200, URL: req.URL}, nil
	}
	return received
}

func envSend(h *harness, received <-chan model.Request) model.Request {
	h.t.Helper()
	h.click("Send")
	h.waitFor("environment request", func() bool { return !h.App.Active().Sending && h.App.Active().Response != nil })
	select {
	case req := <-received:
		return req
	default:
		h.t.Fatal("request fake did not receive the send")
		return model.Request{}
	}
}

func TestEnvironmentCreateSelectAndSend(t *testing.T) {
	for _, language := range []string{"zh", "en"} {
		t.Run(language, func(t *testing.T) {
			svc := fake.New()
			settings := svc.Settings.Get()
			settings.Language, settings.Theme, settings.HistoryLimit = language, "dark", 37
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			received := envCaptureRequests(h)
			h.click(h.App.S.Env.Edit)
			if !h.shows(h.App.S.Env.SaveHint) {
				t.Fatal("editor does not explain Save and discard")
			}
			h.click(h.App.S.Env.New)
			envReplaceField(h, h.App.S.Env.Name, "Development")
			h.click(h.App.S.Env.AddRow)
			h.typeInto(h.App.S.Env.Key, "host")
			h.typeInto(h.App.S.Env.Value, "https://dev.example.test")
			if len(svc.Environments.List()) != 0 {
				t.Fatal("New persisted before Save")
			}
			h.click(h.App.S.Env.Save)
			envs := svc.Environments.List()
			if len(envs) != 1 || envs[0].Name != "Development" || !reflect.DeepEqual(envs[0].Variables, []model.KV{{Key: "host", Value: "https://dev.example.test"}}) {
				t.Fatalf("saved environments: %+v", envs)
			}
			envSelect(h, "Development")
			settings.ActiveEnvID = envs[0].ID
			if got := svc.Settings.Get(); got != settings {
				t.Fatalf("selection must preserve other settings: got %+v want %+v", got, settings)
			}
			h.App.Active().Request.URL = "{{host}}/users"
			h.pump()
			if req := envSend(h, received); req.URL != "https://dev.example.test/users" {
				t.Fatalf("sent URL %q", req.URL)
			}
			// Recreating the App uses the stored selection, not feature-local state.
			reopened := newHarness(t, svc)
			if reopened.App.env.activeID != envs[0].ID || !reopened.shows("Development") {
				t.Fatal("environment selection was not restored")
			}
			envSelect(h, h.App.S.Env.NoEnvironment)
			if got := svc.Settings.Get(); got.ActiveEnvID != "" || got.Language != language || got.HistoryLimit != 37 || got.Theme != "dark" {
				t.Fatalf("No environment settings: %+v", got)
			}
			if req := envSend(h, received); req.URL != "{{host}}/users" {
				t.Fatalf("No environment sent URL %q", req.URL)
			}
		})
	}
}

func TestEnvironmentOverridesCollectionVariables(t *testing.T) {
	h := newHarness(t)
	received := envCaptureRequests(h)
	col, err := h.Svc.Collections.Create("Collection")
	if err != nil {
		t.Fatal(err)
	}
	col.Variables = []model.KV{{Key: "host", Value: "https://collection.example.test"}, {Key: "path", Value: "users"}}
	if err := h.Svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	env := model.Environment{ID: "dev", Name: "Development", Variables: []model.KV{
		{Key: "host", Value: "https://env.example.test"},
		{Key: "host", Value: "https://disabled.example.test", Disabled: true},
	}}
	if err := h.Svc.Environments.Save(env); err != nil {
		t.Fatal(err)
	}
	h.App.Active().CollectionID = col.ID
	h.App.Active().Request.URL = "{{host}}/{{path}}"
	h.pump()
	envSelect(h, env.Name)
	want := append(append([]model.KV(nil), col.Variables...), env.Variables[0])
	if got := h.App.Vars(); !reflect.DeepEqual(got, want) {
		t.Fatalf("variable precedence: got %+v want %+v", got, want)
	}
	if req := envSend(h, received); req.URL != "https://env.example.test/users" {
		t.Fatalf("environment override: %q", req.URL)
	}
	envSelect(h, h.App.S.Env.NoEnvironment)
	if req := envSend(h, received); req.URL != "https://collection.example.test/users" {
		t.Fatalf("collection fallback: %q", req.URL)
	}
	// Returned variables belong to the send, not the service's backing slice.
	vs := h.App.Vars()
	vs[0].Value = "changed"
	stored, _ := h.Svc.Collections.Get(col.ID)
	if stored.Variables[0].Value != col.Variables[0].Value {
		t.Fatal("Vars aliases stored collection variables")
	}
}

func TestEnvironmentUnresolvedVarsAndTooltip(t *testing.T) {
	h := newHarness(t)
	h.App.Active().Request.URL = "https://{{missing}}/{{ missing }}/{{known}}"
	h.App.Active().Request.Headers = []model.KV{
		{Key: "X-Token", Value: "{{token}}/{{missing}}"},
		{Key: "{{ignored-key}}", Value: "literal"},
		{Key: "X-Disabled", Value: "{{ignored-value}}", Disabled: true},
	}
	env := model.Environment{ID: "dev", Name: "Development", Variables: []model.KV{
		{Key: "known", Value: "value"}, {Key: "token", Value: "disabled", Disabled: true},
	}}
	if err := h.Svc.Environments.Save(env); err != nil {
		t.Fatal(err)
	}
	settings := h.Svc.Settings.Get()
	settings.ActiveEnvID = env.ID
	if err := h.Svc.Settings.Save(settings); err != nil {
		t.Fatal(err)
	}
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	if got := h.App.UnresolvedVars(); !reflect.DeepEqual(got, []string{"missing", "token"}) {
		t.Fatalf("unresolved variables: %q", got)
	}
	badge := fmt.Sprintf(h.App.S.Env.Unresolved, 2)
	if !h.shows(badge) {
		t.Fatalf("missing unresolved-variable badge %q", badge)
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == badge {
			if err := h.D.Move(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			if err := h.D.Advance(time.Second); err != nil {
				t.Fatal(err)
			}
			if !h.shows(h.App.S.Env.UnresolvedTip + "missing, token") {
				t.Fatal("tooltip does not list unresolved names")
			}
			return
		}
	}
	t.Fatal("warning badge cannot be hovered")
}

func TestEnvironmentEditorDiscardRenameDuplicateAndDelete(t *testing.T) {
	h := newHarness(t)
	original := model.Environment{Version: model.SchemaVersion, ID: "original", Name: "Original", Variables: []model.KV{{Key: "host", Value: "original"}}}
	if err := h.Svc.Environments.Save(original); err != nil {
		t.Fatal(err)
	}
	settings := h.Svc.Settings.Get()
	settings.ActiveEnvID = original.ID
	if err := h.Svc.Settings.Save(settings); err != nil {
		t.Fatal(err)
	}
	h.pump()
	h.click(h.App.S.Env.Edit)
	envReplaceField(h, h.App.S.Env.Value, "discarded")
	h.click(h.App.S.Env.Duplicate)
	envReplaceField(h, h.App.S.Env.Name, "Discarded copy")
	h.click(h.App.S.Env.Delete)
	h.click(h.App.S.Env.Delete)
	h.click(h.App.S.Env.New)
	h.click(h.App.S.Env.Close)
	if got := h.Svc.Environments.List(); !reflect.DeepEqual(got, []model.Environment{original}) {
		t.Fatalf("closing changed stored environments: %+v", got)
	}
	if h.Svc.Settings.Get() != settings {
		t.Fatal("closing changed active environment settings")
	}
	h.click(h.App.S.Env.Edit)
	h.click(h.App.S.Env.Rename)
	envReplaceField(h, h.App.S.Env.Name, "Renamed")
	h.click(h.App.S.Env.Duplicate)
	envReplaceField(h, h.App.S.Env.Name, "Copy")
	envReplaceField(h, h.App.S.Env.Value, "copy value")
	h.click(h.App.S.Env.Enabled)
	h.click(h.App.S.Env.Save)
	stored, _ := h.Svc.Environments.Get(original.ID)
	if stored.Name != "Renamed" || stored.Variables[0].Value != "original" || stored.Variables[0].Disabled {
		t.Fatalf("renamed original was changed by its copy: %+v", stored)
	}
	envs := h.Svc.Environments.List()
	if len(envs) != 2 || envs[0].Name != "Copy" || envs[0].ID == original.ID || envs[0].Variables[0].Value != "copy value" || !envs[0].Variables[0].Disabled {
		t.Fatalf("duplicate not saved independently: %+v", envs)
	}
	h.click(h.App.S.Env.Edit)
	h.click(h.App.S.Env.Delete)
	h.click(h.App.S.Env.Save)
	if _, ok := h.Svc.Environments.Get(original.ID); ok {
		t.Fatal("Delete did not remove active environment")
	}
	settings.ActiveEnvID = ""
	if got := h.Svc.Settings.Get(); got != settings {
		t.Fatalf("deleting active environment did not clear only selection: %+v", got)
	}
}

func TestEnvironmentEditorEscapeDiscardsAndRowsKeepIdentity(t *testing.T) {
	h := newHarness(t)
	env := model.Environment{ID: "dev", Name: "Development", Variables: []model.KV{{Key: "first", Value: "one"}, {Key: "second", Value: "two"}}}
	if err := h.Svc.Environments.Save(env); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Env.Edit)
	var secondValueID gfx.ID
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Env.Value {
			secondValueID = hit.ID
		}
	}
	h.click(h.App.S.Env.DeleteRow)
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Env.Value && hit.ID != secondValueID {
			t.Fatal("deleting an earlier row changed surviving field identity")
		}
	}
	envReplaceField(h, h.App.S.Env.Value, "updated")
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	stored, _ := h.Svc.Environments.Get(env.ID)
	if !reflect.DeepEqual(stored, env) {
		t.Fatalf("Escape persisted variable edits: %+v", stored)
	}
	h.click(h.App.S.Env.Edit)
	h.click(h.App.S.Env.DeleteRow)
	h.click(h.App.S.Env.AddRow)
	h.click(h.App.S.Env.Save)
	stored, _ = h.Svc.Environments.Get(env.ID)
	if len(stored.Variables) != 2 || stored.Variables[0].Key != "second" || stored.Variables[1].Disabled {
		t.Fatalf("row delete/add not saved: %+v", stored)
	}
}

type envFailSave struct {
	service.EnvironmentService
	fail bool
}

func (s *envFailSave) Save(env model.Environment) error {
	if s.fail {
		return errors.New("write failed")
	}
	return s.EnvironmentService.Save(env)
}

func TestEnvironmentSaveFailureKeepsDraftForRetry(t *testing.T) {
	svc := fake.New()
	failing := &envFailSave{EnvironmentService: svc.Environments, fail: true}
	svc.Environments = failing
	h := newHarness(t, svc)
	h.click(h.App.S.Env.Edit)
	h.click(h.App.S.Env.New)
	envReplaceField(h, h.App.S.Env.Name, "Retry")
	h.click(h.App.S.Env.Save)
	if !h.shows("write failed") || !h.App.env.open || len(svc.Environments.List()) != 0 {
		t.Fatal("failed Save lost draft or hid error")
	}
	failing.fail = false
	h.click(h.App.S.Env.Save)
	if envs := svc.Environments.List(); len(envs) != 1 || envs[0].Name != "Retry" {
		t.Fatalf("retry lost edits: %+v", envs)
	}
}

func TestEnvironmentNameRequiredBeforeSaving(t *testing.T) {
	h := newHarness(t)
	h.click(h.App.S.Env.Edit)
	h.click(h.App.S.Env.New)
	envReplaceField(h, h.App.S.Env.Name, "  ")
	h.click(h.App.S.Env.Save)
	if !h.shows(h.App.S.Env.NameRequired) || len(h.Svc.Environments.List()) != 0 {
		t.Fatal("blank name was saved or validation was hidden")
	}
	envReplaceField(h, h.App.S.Env.Name, "  Development  ")
	h.click(h.App.S.Env.Save)
	if envs := h.Svc.Environments.List(); len(envs) != 1 || envs[0].Name != "Development" {
		t.Fatalf("name correction was not saved: %+v", envs)
	}
}

type envFailSettings struct{ service.SettingsService }

func (s *envFailSettings) Save(model.Settings) error { return errors.New("settings write failed") }

func TestEnvironmentSelectionFailurePreservesSettings(t *testing.T) {
	svc := fake.New()
	if err := svc.Environments.Save(model.Environment{ID: "dev", Name: "Development"}); err != nil {
		t.Fatal(err)
	}
	settings := svc.Settings.Get()
	svc.Settings = &envFailSettings{SettingsService: svc.Settings}
	h := newHarness(t, svc)
	envSelect(h, "Development")
	if got := svc.Settings.Get(); got != settings || !h.shows("settings write failed") {
		t.Fatalf("failed settings Save: settings=%+v", got)
	}
	if len(h.App.Vars()) != 0 || !h.shows(h.App.S.Env.NoEnvironment) {
		t.Fatal("failed selection was used instead of persisted settings")
	}
}

type envFailDelete struct {
	service.EnvironmentService
	failID string
}

func (s *envFailDelete) Delete(id string) error {
	if id == s.failID {
		return errors.New("delete failed")
	}
	if _, ok := s.EnvironmentService.Get(id); !ok {
		return fake.ErrNotFound
	}
	return s.EnvironmentService.Delete(id)
}

func TestEnvironmentDeleteFailureCanRetryRemainingDeletions(t *testing.T) {
	svc := fake.New()
	for _, env := range []model.Environment{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}} {
		if err := svc.Environments.Save(env); err != nil {
			t.Fatal(err)
		}
	}
	failing := &envFailDelete{EnvironmentService: svc.Environments, failID: "b"}
	svc.Environments = failing
	h := newHarness(t, svc)
	h.click(h.App.S.Env.Edit)
	h.click(h.App.S.Env.Delete)
	h.click(h.App.S.Env.Delete)
	h.click(h.App.S.Env.Save)
	if !h.shows("delete failed") || len(svc.Environments.List()) != 1 {
		t.Fatal("delete failure did not preserve remaining deletion")
	}
	failing.failID = ""
	h.click(h.App.S.Env.Save)
	if h.App.env.open || len(svc.Environments.List()) != 0 {
		t.Fatal("retry repeated completed deletions or lost remaining ones")
	}
}

func BenchmarkEnvironmentPaint(b *testing.B) {
	for _, editor := range []bool{false, true} {
		b.Run(fmt.Sprintf("editor=%v", editor), func(b *testing.B) {
			svc := fake.New()
			if err := svc.Environments.Save(model.Environment{ID: "dev", Name: "Development", Variables: []model.KV{{Key: "host", Value: "https://example.test"}}}); err != nil {
				b.Fatal(err)
			}
			a := New(svc)
			if editor {
				a.openEnvironments()
			}
			page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
			config := page.Config(app.Config{Width: 1280, Height: 800})
			d, err := app.NewDriver(config, nil)
			if err != nil {
				b.Fatal(err)
			}
			defer d.Close()
			for range 5 {
				config.Invalidator.Invalidate()
				if err := d.Pump(); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				config.Invalidator.Invalidate()
				if err := d.Pump(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
