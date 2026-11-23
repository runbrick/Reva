package gui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service/fake"
	requestservice "github.com/runbrick/reva/internal/service/request"
)

func TestCollectionRunnerPresetReopensWithoutSensitiveData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	const presetName = "Smoke preset"
	const secretEnv = "private-environment-value"
	const secretCookie = "private-session-cookie"
	const secretToken = "private-access-token"
	wire := make(chan string, 6)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if cookie, err := req.Cookie("session"); err == nil {
			path += ":" + cookie.Value
		}
		wire <- path
		http.SetCookie(w, &http.Cookie{Name: "session", Value: secretCookie, Path: "/"})
	}))
	defer server.Close()
	svc := fake.New()
	svc.Requests = requestservice.New()
	first := collectionRunItem("first", server.URL+"/{{tenant}}/first")
	first.Request.Auth = model.Auth{Type: model.AuthBearer, Token: secretToken}
	collection := model.Collection{ID: "project", Name: "Runner project", Items: []model.Item{
		collectionRunItem("outside", server.URL+"/outside"),
		{ID: "outer", Name: "Parent", Type: model.ItemFolder, Items: []model.Item{
			{ID: "folder", Name: "Smoke cases", Type: model.ItemFolder, Items: []model.Item{
				first, collectionRunItem("second", server.URL+"/{{tenant}}/second"),
			}},
		}},
	}}
	environment := model.Environment{ID: "staging", Name: "Staging", Variables: []model.KV{{Key: "tenant", Value: secretEnv}}}
	authMust(t, svc.Collections.Save(collection))
	authMust(t, svc.Environments.Save(environment))
	dataPath := filepath.Join(t.TempDir(), "private-run-input.csv")
	authMust(t, os.WriteFile(dataPath, []byte("tenant\nprivate-csv-row\n"), 0600))
	t.Run("save before closing", func(t *testing.T) {
		h := newHarness(t, svc)
		labels, text := i18n.RunnerFor(h.App.S.Lang), h.App.S.Collections
		collectionRunOverview(h, collection.ID, "")
		h.click(text.RunCollection)
		h.waitFor("preset picker", func() bool { return h.shows(labels.CustomPreset) })
		h.click(labels.Scope)
		collectionsOption(h, fmt.Sprintf(labels.FolderPath, "Parent", "Smoke cases"))
		h.click(text.RunEnvironment)
		collectionsOption(h, environment.Name)
		h.click(text.RunStopOnFailure)
		h.click(text.RunUseCookieSession)
		authMust(t, h.D.QueueOpenFiles([]string{dataPath}, nil))
		h.click(labels.BrowseData)
		h.typeInto(labels.PresetName, presetName)
		h.click(labels.SavePreset)
		h.waitFor("saved preset", func() bool { return h.App.collections.runner.presets.value.ID != "" })
		if !h.shows(labels.DataSelected) || len(wire) != 0 {
			t.Fatal("saving a preset cleared the active draft or sent a request")
		}
	})
	presetPath := filepath.Join(dir, "runner-presets.json")
	before, err := os.ReadFile(presetPath)
	authMust(t, err)
	var disk struct {
		Collections map[string][]map[string]json.RawMessage
	}
	authMust(t, json.Unmarshal(before, &disk))
	if len(disk.Collections) != 1 || len(disk.Collections[collection.ID]) != 1 {
		t.Fatal("preset file did not partition by collection ID")
	}
	fields := disk.Collections[collection.ID][0]
	if len(fields) != 6 || string(fields["folderId"]) != `"folder"` || string(fields["environmentId"]) != `"staging"` ||
		len(fields["id"]) < 3 || string(fields["name"]) != `"Smoke preset"` ||
		string(fields["stopOnFailure"]) != "true" || string(fields["useCookieSession"]) != "true" {
		t.Fatalf("unexpected preset fields: %s", before)
	}
	for _, secret := range []string{"private-run-input.csv", "private-csv-row", secretEnv, secretCookie, secretToken, server.URL} {
		if bytes.Contains(before, []byte(secret)) {
			t.Fatalf("preset file contains %q", secret)
		}
	}
	h := newHarness(t, svc)
	labels, text := i18n.RunnerFor(h.App.S.Lang), h.App.S.Collections
	collectionRunOverview(h, collection.ID, "")
	h.click(text.RunCollection)
	authMust(t, h.D.Resize(960, 600))
	h.waitFor("reopened preset picker", func() bool { return h.shows(labels.CustomPreset) })
	authMust(t, h.D.QueueOpenFiles([]string{dataPath}, nil))
	h.click(labels.BrowseData)
	h.click(labels.Preset)
	collectionsOption(h, presetName)
	state := &h.App.collections.runner
	if h.shows(labels.DataSelected) || state.dataFile != "" || len(wire) != 0 || state.job != nil ||
		!state.stopOnFailure || !state.useCookieSession || !h.shows(fmt.Sprintf(labels.CollectionTarget, collection.Name)) ||
		!h.shows(fmt.Sprintf(labels.FolderPath, "Parent", "Smoke cases")) || !h.shows(environment.Name) {
		t.Fatal("applying the preset did not confirm the saved configuration without the old file or a send")
	}
	h.click(text.RunStart)
	if report := collectionRunReport(h); report.Iterations != 0 || report.Counts != (runner.Counts{Total: 2, Passed: 2}) {
		t.Fatalf("unexpected scoped report: %+v", report)
	}
	assertCollectionRunPaths(t, wire, "/"+secretEnv+"/first", "/"+secretEnv+"/second:"+secretCookie)
	after, err := os.ReadFile(presetPath)
	authMust(t, err)
	if !bytes.Equal(before, after) {
		t.Fatal("executing a run wrote results to the preset file")
	}
}

func TestCollectionRunnerPresetManagementAndStorageErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	path := filepath.Join(dir, "runner-presets.json")
	authMust(t, os.WriteFile(path, []byte("broken"), 0600))
	svc := fake.New()
	collection := model.Collection{ID: "project", Name: "Project"}
	authMust(t, svc.Collections.Save(collection))
	h := newHarness(t, svc)
	labels, text := i18n.RunnerFor(h.App.S.Lang), h.App.S.Collections
	collectionRunOverview(h, collection.ID, "")
	h.click(text.RunCollection)
	h.waitFor("preset read error", func() bool { return h.shows(labels.PresetLoadError) })
	authMust(t, os.Remove(path))
	h.click(labels.RetryPresets)
	h.waitFor("reloaded preset picker", func() bool { return h.shows(labels.CustomPreset) })
	h.click(labels.SavePreset)
	h.waitFor("empty preset name", func() bool { return h.shows(labels.EmptyPresetName) })
	h.typeInto(labels.PresetName, "Smoke")
	h.click(labels.SavePreset)
	panel := &h.App.collections.runner.presets
	h.waitFor("created preset", func() bool { return panel.value.ID != "" })
	id := panel.value.ID
	h.click(labels.Preset)
	collectionsOption(h, labels.CustomPreset)
	h.typeInto(labels.PresetName, "smoke")
	h.click(labels.SavePreset)
	h.waitFor("duplicate preset name", func() bool { return h.shows(labels.DuplicatePresetName) })
	h.click(labels.Preset)
	collectionsOption(h, "Smoke")
	envReplaceField(h, labels.PresetName, "Renamed")
	authMust(t, os.WriteFile(path, []byte("broken"), 0600))
	h.click(labels.UpdatePreset)
	h.waitFor("preset write error", func() bool { return h.shows(labels.PresetWriteError) })
	if panel.name != "Renamed" || panel.value.ID != id {
		t.Fatal("failed update discarded the selected preset or the name draft")
	}
	if _, _, err := runner.SavePreset(collection.ID, runner.Preset{Name: "Smoke"}); err == nil {
		t.Fatal("failed update silently replaced the preset store")
	}
	authMust(t, os.Remove(path))
	_, _, err := runner.SavePreset(collection.ID, runner.Preset{Name: "Smoke"})
	authMust(t, err)
	h.click(labels.UpdatePreset)
	h.waitFor("preset became unavailable", func() bool { return h.shows(labels.PresetUnavailable) })
	h.click(labels.RetryPresets)
	h.waitFor("reloaded preset picker", func() bool { return h.shows(labels.CustomPreset) })
	h.click(labels.Preset)
	collectionsOption(h, "Smoke")
	id = panel.value.ID
	envReplaceField(h, labels.PresetName, "Renamed")
	h.click(labels.UpdatePreset)
	h.waitFor("updated preset name", func() bool { return panel.value.Name == "Renamed" })
	if panel.value.ID != id {
		t.Fatal("updating the preset changed its ID")
	}
	h.click(labels.DeletePreset)
	h.waitFor("deleted preset", func() bool { return len(panel.choices) == 1 })
	if presets, err := runner.ListPresets(collection.ID); err != nil || len(presets) != 0 {
		t.Fatalf("preset remains after deletion: %v, %v", presets, err)
	}
}

func TestCollectionRunnerPresetUnavailableTargetsRequireReselection(t *testing.T) {
	for _, tc := range []struct {
		name        string
		staleFolder bool
		beforeApply bool
		hideEnv     bool
	}{
		{name: "deleted folder at apply", staleFolder: true, beforeApply: true},
		{name: "deleted environment at apply", beforeApply: true},
		{name: "hidden environment at start", hideEnv: true},
		{name: "deleted folder at start", staleFolder: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("REVA_HOME", t.TempDir())
			wire := make(chan string, 4)
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
				wire <- req.URL.Path
			}))
			defer server.Close()
			svc := fake.New()
			svc.Requests = requestservice.New()
			collection := model.Collection{ID: "project", Name: "Project", Items: []model.Item{
				collectionRunItem("root", server.URL+"/root"),
				{ID: "folder", Name: "Smoke cases", Type: model.ItemFolder, Items: []model.Item{
					collectionRunItem("request", server.URL+"/folder"),
				}},
			}}
			env := model.Environment{ID: "staging", Name: "Staging"}
			authMust(t, svc.Collections.Save(collection))
			authMust(t, svc.Environments.Save(env))
			preset := runner.Preset{Name: "Smoke preset", FolderID: "folder", EnvironmentID: env.ID}
			_, _, err := runner.SavePreset(collection.ID, preset)
			authMust(t, err)
			invalidate := func() {
				switch {
				case tc.staleFolder:
					collection.Items = collection.Items[:1]
					authMust(t, svc.Collections.Save(collection))
				case tc.hideEnv:
					collection.EnvironmentScope = &model.CollectionEnvironmentScope{VisibleIDs: []string{}}
					authMust(t, svc.Collections.Save(collection))
				default:
					authMust(t, svc.Environments.Delete(env.ID))
				}
			}
			if tc.beforeApply {
				invalidate()
			}
			h := newHarness(t, svc)
			labels, text := i18n.RunnerFor(h.App.S.Lang), h.App.S.Collections
			collectionRunOverview(h, collection.ID, "")
			h.click(text.RunCollection)
			h.waitFor("preset picker", func() bool { return h.shows(labels.CustomPreset) })
			h.click(labels.Preset)
			collectionsOption(h, preset.Name)
			warning, selector, replacement := text.RunMissingEnvironment, text.RunEnvironment, text.RunNoEnvironment
			wantPath := "/folder"
			if tc.staleFolder {
				warning, selector, replacement = labels.MissingFolder, labels.Scope, labels.WholeCollection
				wantPath = "/root"
			}
			if tc.beforeApply && !h.shows(warning) {
				t.Fatal("applying the preset did not disclose the unavailable target")
			}
			if !tc.beforeApply {
				invalidate()
			}
			state := &h.App.collections.runner
			for range 2 {
				h.click(text.RunStart)
				if len(wire) != 0 || state.job != nil || !h.shows(warning) || state.folderID != preset.FolderID || state.environmentID != preset.EnvironmentID {
					t.Fatal("an unavailable preset target allowed an implicit fallback")
				}
			}
			h.click(selector)
			collectionsOption(h, replacement)
			h.click(text.RunStart)
			if report := collectionRunReport(h); report.Counts.Passed != 1 {
				t.Fatalf("explicit reselection did not recover: %+v", report)
			}
			assertCollectionRunPaths(t, wire, wantPath)
		})
	}
}

func TestCollectionRunnerPresetReviewsRenamedRunTargets(t *testing.T) {
	t.Setenv("REVA_HOME", t.TempDir())
	wire := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		wire <- req.URL.Path
	}))
	defer server.Close()
	svc := fake.New()
	svc.Requests = requestservice.New()
	collection := model.Collection{ID: "project", Name: "Project", Items: []model.Item{
		{ID: "folder", Name: "Smoke cases", Type: model.ItemFolder, Items: []model.Item{
			collectionRunItem("request", server.URL+"/scoped"),
		}},
	}}
	env := model.Environment{ID: "staging", Name: "Staging"}
	authMust(t, svc.Collections.Save(collection))
	authMust(t, svc.Environments.Save(env))
	preset := runner.Preset{Name: "Smoke preset", FolderID: "folder", EnvironmentID: env.ID}
	_, _, err := runner.SavePreset(collection.ID, preset)
	authMust(t, err)
	h := newHarness(t, svc)
	labels, text := i18n.RunnerFor(h.App.S.Lang), h.App.S.Collections
	collectionRunOverview(h, collection.ID, "")
	h.click(text.RunCollection)
	h.waitFor("preset picker", func() bool { return h.shows(labels.CustomPreset) })
	h.click(labels.Preset)
	collectionsOption(h, preset.Name)
	collection.Name, collection.Items[0].Name, env.Name = "New project", "New scope", "New environment"
	authMust(t, svc.Collections.Save(collection))
	authMust(t, svc.Environments.Save(env))
	h.click(text.RunStart)
	if len(wire) != 0 || h.App.collections.runner.job != nil || !h.shows(labels.TargetChanged) ||
		!h.shows(fmt.Sprintf(labels.CollectionTarget, collection.Name)) || !h.shows(collection.Items[0].Name) || !h.shows(env.Name) {
		t.Fatal("starting a renamed target did not first show the current scope and environment")
	}
	h.click(text.RunStart)
	if report := collectionRunReport(h); report.Counts.Passed != 1 {
		t.Fatalf("confirmed run failed: %+v", report)
	}
	assertCollectionRunPaths(t, wire, "/scoped")
}
