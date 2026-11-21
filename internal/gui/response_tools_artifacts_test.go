package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/responseexample"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestResponseExampleOfflineReopenAndDelete(t *testing.T) {
	home := t.TempDir()
	t.Setenv(storage.EnvHome, home)
	local, err := store.Open(home)
	authMust(t, err)
	var sends atomic.Int32
	sender := &fake.Requests{Fn: func(_ context.Context, req model.Request) (model.Response, error) {
		sends.Add(1)
		return artifactResponseFixture(), nil
	}}
	svc := artifactServices(local, sender)
	collection, err := svc.Collections.Create("Example collection")
	authMust(t, err)
	req := model.NewRequest("")
	req.Name, req.Method, req.URL = "Example request", model.MethodPost, "https://private-host.test/v1/items?token=private-query"
	req, err = svc.Collections.SaveRequest(collection.ID, "", req)
	authMust(t, err)
	key := responseexample.Key{CollectionID: collection.ID, RequestID: req.ID}

	first := newHarness(t, svc)
	requestEnglish(first)
	first.click(req.Name)
	labels := first.App.S.Response.Artifacts
	first.click(labels.Example)
	if !first.shows(labels.Empty) {
		t.Fatal("the unsent saved request did not show the empty example state")
	}
	first.click(first.App.S.Common.Close)
	first.click("Send")
	first.waitFor("a response", func() bool { return first.App.Active().Response != nil })
	first.click(labels.Example)
	first.click(labels.ReviewSave)
	assertExampleMissing(t, key)
	first.click(labels.Save)
	sample, found, err := responseexample.Load(key)
	if err != nil || !found || sample.Body != "" || sample.Path != "/v1/items" || sample.Method != model.MethodPost || sample.Status != 201 || sample.ContentType != "application/json" {
		t.Fatalf("default saved example: %+v, %v, %v", sample, found, err)
	}
	first.click(labels.ReviewReplace)
	first.click(labels.IncludeBody)
	if !first.shows(labels.BodyHelp) {
		t.Fatal("checking the body option did not expose the review step")
	}
	first.click(labels.Replace)
	first.click(first.App.S.Common.Close)

	reopened, err := store.Open(home)
	authMust(t, err)
	second := newHarness(t, artifactServices(reopened, sender))
	requestEnglish(second)
	second.click(req.Name)
	second.click(labels.Example)
	if second.App.Active().Response != nil || !second.shows("/v1/items") || !second.shows("201") || second.shows(labels.NoSavedBody) {
		t.Fatal("saved response could not be viewed offline")
	}
	exportPath := filepath.Join(t.TempDir(), "from-example.json")
	defaultPath := filepath.Join(filepath.Dir(exportPath), "metadata-only.json")
	second.click(labels.ExportExample)
	second.click(labels.Preview)
	authMust(t, second.D.QueueSaveFile(defaultPath, nil))
	second.click(labels.ConfirmExport)
	defaultMapping, err := os.ReadFile(defaultPath)
	authMust(t, err)
	if strings.Contains(string(defaultMapping), `"body"`) {
		t.Fatal("export of a saved example included the body before review")
	}
	second.click(labels.Example)
	second.click(labels.ExportExample)
	second.click(labels.IncludeBody)
	second.click(labels.Preview)
	assertArtifactFileMissing(t, exportPath)
	authMust(t, second.D.QueueSaveFile(exportPath, nil))
	second.click(labels.ConfirmExport)
	exported, err := os.ReadFile(exportPath)
	authMust(t, err)
	if !strings.Contains(string(exported), `"body": "{\"items\":[42]}"`) || strings.Contains(string(exported), "private-") {
		t.Fatalf("mapping did not contain only reviewed data:\n%s", exported)
	}
	second.click(labels.Example)
	second.click(labels.Delete)
	if _, found, err := responseexample.Load(key); err != nil || !found {
		t.Fatalf("opening delete confirmation removed the example: %v, %v", found, err)
	}
	second.click(labels.ConfirmDelete)
	assertExampleMissing(t, key)
	if sends.Load() != 1 || len(second.Svc.History.List()) != 1 {
		t.Fatalf("expected one explicit send and history entry, got %d sends and %d history entries", sends.Load(), len(second.Svc.History.List()))
	}
}

func TestWireMockExportUsesCompletedSendAndExplicitPreview(t *testing.T) {
	view := responseHarness(t, artifactResponseFixture(), nil)
	tab := view.App.Active()
	tab.Request.URL, tab.Request.Method = "https://other-host.test/wrong?secret=private-draft", model.MethodDelete
	view.pump()
	labels := view.App.S.Response.Artifacts
	if view.shows(labels.Example) {
		t.Fatal("unsaved request can open the local example flow")
	}
	path := filepath.Join(t.TempDir(), "mapping.json")
	view.click(labels.Export)
	view.click(labels.Preview)
	if !view.shows(labels.PreviewHelp) {
		t.Fatal("mapping preview is missing")
	}
	authMust(t, view.D.QueueSaveFile("", app.ErrCanceled))
	view.click(labels.ConfirmExport)
	assertArtifactFileMissing(t, path)
	if !view.shows(labels.PreviewHelp) {
		t.Fatal("cancelling the save picker lost the reviewed mapping")
	}
	authMust(t, view.D.QueueSaveFile(path, nil))
	view.click(labels.ConfirmExport)
	data, err := os.ReadFile(path)
	authMust(t, err)
	for _, expected := range []string{`"method": "GET"`, `"urlPath": "/response"`, `"status": 201`, `"Content-Type": "application/json"`} {
		if !strings.Contains(string(data), expected) {
			t.Errorf("missing %s in %s", expected, data)
		}
	}
	for _, unwanted := range []string{"private-", "wrong", "DELETE", "items", "body", "Set-Cookie", "Location"} {
		if strings.Contains(string(data), unwanted) {
			t.Errorf("unexpected %q in %s", unwanted, data)
		}
	}
	view.click(labels.Export)
	view.click(labels.Preview)
	authMust(t, view.D.Key(gfx.KeyEscape, 0))
	if view.shows(labels.PreviewHelp) {
		t.Fatal("Escape did not dismiss the mapping preview")
	}
}

func TestWireMockExportDoesNotPrefillResolvedPathVariables(t *testing.T) {
	svc := fake.New()
	svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		return artifactResponseFixture(), nil
	}
	view := newHarness(t, svc)
	requestEnglish(view)
	tab := view.App.Active()
	tab.Request.URL = "{{base}}/v1/items/{{token}}?auth=private-query"
	tab.Request.PreScript = `pm.variables.set("base", "https://private-host.test"); pm.variables.set("token", "reva-variable");`
	view.pump()
	view.click("Send")
	view.waitFor("a scripted response", func() bool { return !tab.Sending })
	labels := view.App.S.Response.Artifacts
	view.click(labels.Export)
	view.click(labels.Preview)
	if view.shows(labels.PreviewHelp) {
		t.Fatal("a variable-expanded path was prefilled")
	}
	view.typeInto(labels.Path, "/v1/items/fixture")
	view.click(labels.Preview)
	if !view.shows(labels.PreviewHelp) {
		t.Fatal("a reviewed literal path could not be exported")
	}
}

func artifactResponseFixture() model.Response {
	return model.Response{
		StatusCode: 201, Status: "201 Created", Body: []byte(`{"items":[42]}`),
		URL: "https://private-host.test/path?api_key=private-response-url",
		Headers: []model.KV{
			{Key: "Content-Type", Value: "application/json; profile=private-profile"},
			{Key: "Set-Cookie", Value: "key=private-cookie"},
			{Key: "Location", Value: "https://private-host.test/path"},
		},
		Cookies: []model.Cookie{{Name: "key", Value: "private-cookie"}},
	}
}

func artifactServices(local *store.Store, sender service.RequestService) service.Services {
	return service.Services{
		Requests: sender, History: local.History(), Collections: local.Collections(),
		Environments: local.Environments(), Settings: local.Settings(),
	}
}

func assertExampleMissing(t *testing.T, key responseexample.Key) {
	t.Helper()
	if _, found, err := responseexample.Load(key); err != nil || found {
		t.Fatalf("unexpected persisted example: %v, %v", found, err)
	}
}

func assertArtifactFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no mapping file, got %v", err)
	}
}
