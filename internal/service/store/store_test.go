package store_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service/store"
	"github.com/runbrick/reqo/internal/storage"
)

func openStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if errs := s.LoadErrors(); len(errs) != 0 {
		t.Fatalf("unexpected load errors: %v", errs)
	}
	return s
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func request(name string) model.Request {
	r := model.NewRequest("")
	r.Name, r.URL = name, "https://example.test/{{path}}"
	r.Params = []model.KV{{Key: "q", Value: "param"}}
	r.Headers = []model.KV{{Key: "X-Test", Value: "header"}}
	r.Body = model.Body{
		Mode: model.BodyFormData, Form: []model.FormField{{Key: "f", Value: "form"}},
		URLEncoded: []model.KV{{Key: "u", Value: "urlencoded"}},
	}
	return r
}

func changeRequest(r *model.Request) {
	r.Name = "changed"
	r.Params[0].Value = "changed"
	r.Headers[0].Value = "changed"
	r.Body.Form[0].Value = "changed"
	r.Body.URLEncoded[0].Value = "changed"
}

func assertRequest(t *testing.T, got, want model.Request) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request changed through an alias:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	if got := s.Settings().Get(); got != model.DefaultSettings() {
		t.Fatalf("defaults = %+v", got)
	}
	must(t, storage.WriteJSON(filepath.Join(dir, "settings.json"), model.Settings{Theme: "dark"}))
	s = openStore(t, dir)
	want := model.DefaultSettings()
	want.Theme = "dark"
	if got := s.Settings().Get(); got != want {
		t.Fatalf("partial settings = %+v, want %+v", got, want)
	}
	want.Language, want.ActiveEnvID, want.HistoryLimit = "en", "environment", 7
	must(t, s.Settings().Save(want))
	if got := openStore(t, dir).Settings().Get(); got != want {
		t.Fatalf("reopened settings = %+v, want %+v", got, want)
	}
	must(t, s.Settings().Save(model.Settings{}))
	if got := openStore(t, dir).Settings().Get(); got != model.DefaultSettings() {
		t.Fatalf("saving zero fields = %+v", got)
	}
}

func TestHistoryRoundTripAndMutations(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	entry := model.HistoryEntry{Request: request("first"), StatusCode: 201, Duration: 3 * time.Millisecond}
	must(t, s.History().Add(entry))
	first := s.History().List()[0]
	if first.ID == "" || first.At.IsZero() {
		t.Fatalf("ID/time not assigned: %+v", first)
	}
	if got := openStore(t, dir).History().List(); !reflect.DeepEqual(got, []model.HistoryEntry{first}) {
		t.Fatalf("reopened history = %+v", got)
	}
	second := model.HistoryEntry{ID: "provided", At: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Request: request("second"), Error: "offline"}
	must(t, s.History().Add(second))
	got := openStore(t, dir).History().List()
	if !reflect.DeepEqual(got, []model.HistoryEntry{second, first}) {
		t.Fatalf("history must prepend and preserve supplied ID/time: %+v", got)
	}
	must(t, s.History().Delete(first.ID))
	if got := openStore(t, dir).History().List(); len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("delete did not persist: %+v", got)
	}
	must(t, s.History().Clear())
	if got := openStore(t, dir).History().List(); len(got) != 0 {
		t.Fatalf("clear did not persist: %+v", got)
	}
	data, err := os.ReadFile(filepath.Join(dir, "history.json"))
	must(t, err)
	if strings.TrimSpace(string(data)) != "[]" {
		t.Fatalf("empty history must be an array: %s", data)
	}
}

func TestHistoryLimitsAndSettingsTrim(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	must(t, s.Settings().Save(model.Settings{HistoryLimit: 3}))
	for i := range 5 {
		must(t, s.History().Add(model.HistoryEntry{ID: fmt.Sprint(i), Request: request("sent")}))
	}
	if got := openStore(t, dir).History().List(); len(got) != 3 || got[0].ID != "4" || got[2].ID != "2" {
		t.Fatalf("limit did not retain newest entries: %+v", got)
	}
	must(t, s.Settings().Save(model.Settings{HistoryLimit: 1}))
	s = openStore(t, dir)
	if got := s.History().List(); len(got) != 1 || got[0].ID != "4" {
		t.Fatalf("settings did not persist trim: %+v", got)
	}
	must(t, s.Settings().Save(model.Settings{HistoryLimit: 4}))
	if got := s.History().List(); len(got) != 1 {
		t.Fatal("increasing limit restored discarded entries")
	}
	// Zero settings use the default limit, including when a pre-existing
	// history array exceeds it at startup.
	records := make([]model.HistoryEntry, 202)
	must(t, storage.WriteJSON(filepath.Join(dir, "history.json"), records))
	must(t, storage.WriteJSON(filepath.Join(dir, "settings.json"), model.Settings{}))
	if got := openStore(t, dir).History().List(); len(got) != 200 {
		t.Fatalf("default history limit = %d entries", len(got))
	}
}

func TestCollectionsRoundTripAndNestedFolders(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	col, err := s.Collections().Create("Zulu")
	must(t, err)
	_, err = s.Collections().Create("Alpha")
	must(t, err)
	if got := s.Collections().List(); len(got) != 2 || got[0].Name != "Alpha" {
		t.Fatalf("collections not sorted: %+v", got)
	}
	root, err := s.Collections().AddFolder(col.ID, "", "Root")
	must(t, err)
	nested, err := s.Collections().AddFolder(col.ID, root.ID, "Nested")
	must(t, err)
	r, err := s.Collections().SaveRequest(col.ID, nested.ID, request("inside"))
	must(t, err)
	must(t, s.Collections().RenameItem(col.ID, nested.ID, "Renamed folder"))
	must(t, s.Collections().RenameItem(col.ID, r.ID, "Renamed request"))
	got, ok := openStore(t, dir).Collections().Get(col.ID)
	if !ok || got.Version != model.SchemaVersion || got.Items[0].Items[0].Name != "Renamed folder" {
		t.Fatalf("nested tree did not persist: %+v", got)
	}
	item := model.FindItem(got.Items, r.ID)
	if item == nil || item.Name != "Renamed request" || item.Request.Name != item.Name {
		t.Fatalf("request rename did not synchronize names: %+v", item)
	}
	got.Name, got.Variables = "Changed collection", []model.KV{{Key: "path", Value: "api"}}
	must(t, s.Collections().Save(got))
	reopened, _ := openStore(t, dir).Collections().Get(col.ID)
	if !reflect.DeepEqual(got, reopened) {
		t.Fatalf("collection save did not round-trip: %+v", reopened)
	}
	must(t, s.Collections().DeleteItem(col.ID, root.ID))
	got, _ = openStore(t, dir).Collections().Get(col.ID)
	if len(got.Items) != 0 || model.FindItem(got.Items, r.ID) != nil {
		t.Fatalf("folder deletion left descendants: %+v", got)
	}
	must(t, s.Collections().Delete(col.ID))
	if _, ok := openStore(t, dir).Collections().Get(col.ID); ok {
		t.Fatal("collection deletion did not persist")
	}
}

func TestSaveRequestReplacesInPlaceOrAppends(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	col, err := s.Collections().Create("Requests")
	must(t, err)
	folder, err := s.Collections().AddFolder(col.ID, "", "Folder")
	must(t, err)
	first, err := s.Collections().SaveRequest(col.ID, folder.ID, request("first"))
	must(t, err)
	second, err := s.Collections().SaveRequest(col.ID, folder.ID, request("second"))
	must(t, err)
	first.Name, first.URL = "updated", "https://example.test/updated"
	// An existing request is replaced where it already lives, regardless of
	// the requested destination (even an absent destination).
	_, err = s.Collections().SaveRequest(col.ID, "absent", first)
	must(t, err)
	third := request("third")
	third.ID = "supplied"
	_, err = s.Collections().SaveRequest(col.ID, "", third)
	must(t, err)
	got, _ := openStore(t, dir).Collections().Get(col.ID)
	children := got.Items[0].Items
	if len(got.Items) != 2 || len(children) != 2 || children[0].ID != first.ID || children[1].ID != second.ID {
		t.Fatalf("replacement changed tree order or appended a duplicate: %+v", got.Items)
	}
	if children[0].Name != first.Name || !reflect.DeepEqual(*children[0].Request, first) || got.Items[1].ID != third.ID {
		t.Fatalf("saved request content differs: %+v", got.Items)
	}
}

func TestImportAssignsFreshIDsWithoutMutatingInput(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	r := request("request")
	r.ID = "old-request"
	col := model.Collection{ID: "old-collection", Name: "Imported", Variables: []model.KV{{Key: "a", Value: "b"}}, Items: []model.Item{
		{ID: "old-folder", Name: "Folder", Type: model.ItemFolder, Items: []model.Item{
			{ID: "old-item", Name: r.Name, Type: model.ItemRequest, Request: &r},
		}},
	}}
	before, err := json.Marshal(col)
	must(t, err)
	a, err := s.Collections().Import(col)
	must(t, err)
	b, err := s.Collections().Import(col)
	must(t, err)
	after, err := json.Marshal(col)
	must(t, err)
	if string(before) != string(after) {
		t.Fatal("Import mutated its input")
	}
	seen := map[string]bool{"old-collection": true, "old-folder": true, "old-item": true, "old-request": true}
	for _, imported := range []model.Collection{a, b} {
		for _, id := range []string{imported.ID, imported.Items[0].ID, imported.Items[0].Items[0].ID} {
			if id == "" || seen[id] {
				t.Fatalf("ID %q was not fresh", id)
			}
			seen[id] = true
		}
		item := imported.Items[0].Items[0]
		if item.Request.ID != item.ID || imported.Version != model.SchemaVersion {
			t.Fatalf("imported request identity/version mismatch: %+v", imported)
		}
		got, ok := openStore(t, dir).Collections().Get(imported.ID)
		if !ok || !reflect.DeepEqual(got, imported) {
			t.Fatal("import did not persist")
		}
	}
	changeRequest(a.Items[0].Items[0].Request)
	got, _ := s.Collections().Get(a.ID)
	if got.Items[0].Items[0].Request.Name != "request" {
		t.Fatal("Import return value aliases stored request")
	}
	changeRequest(got.Items[0].Items[0].Request)
	got.Items[0].Items[0].Name = "changed"
	got, _ = s.Collections().Get(a.ID)
	if got.Items[0].Items[0].Name != "request" || got.Items[0].Items[0].Request.Name != "request" {
		t.Fatal("Get return value aliases nested items or requests")
	}
}

func TestEnvironmentsRoundTripAndSorting(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	env, err := s.Environments().Create("Zulu")
	must(t, err)
	_, err = s.Environments().Create("Alpha")
	must(t, err)
	env.Variables = []model.KV{{Key: "base", Value: "https://example.test", Disabled: true}}
	env.Version = 99
	must(t, s.Environments().Save(env))
	env.Version = model.SchemaVersion
	got, ok := openStore(t, dir).Environments().Get(env.ID)
	if !ok || !reflect.DeepEqual(got, env) {
		t.Fatalf("environment did not round-trip: %+v", got)
	}
	if got := s.Environments().List(); len(got) != 2 || got[0].Name != "Alpha" {
		t.Fatalf("environments not sorted: %+v", got)
	}
	env.Name = "Renamed"
	must(t, s.Environments().Save(env))
	got, _ = openStore(t, dir).Environments().Get(env.ID)
	if got.Name != env.Name {
		t.Fatal("renamed environment did not persist")
	}
	must(t, s.Environments().Delete(env.ID))
	if _, ok := openStore(t, dir).Environments().Get(env.ID); ok {
		t.Fatal("environment deletion did not persist")
	}
}

func TestCorruptFilesAreSkippedAndUntouched(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	validCol, err := s.Collections().Create("Healthy")
	must(t, err)
	validEnv, err := s.Environments().Create("Healthy")
	must(t, err)
	paths := []string{"settings.json", "history.json", "collections/bad.json", "environments/bad.json"}
	for _, path := range paths {
		must(t, os.WriteFile(filepath.Join(dir, path), []byte("{broken"), 0o600))
	}
	s, err = store.Open(dir)
	must(t, err)
	if got := s.LoadErrors(); len(got) != len(paths) {
		t.Fatalf("load errors = %v", got)
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(dir, path))
		must(t, err)
		if string(data) != "{broken" {
			t.Fatalf("corrupt file %s was changed: %s", path, data)
		}
		found := false
		for _, err := range s.LoadErrors() {
			found = found || strings.Contains(err.Error(), filepath.Base(path))
		}
		if !found {
			t.Fatalf("load errors do not identify %s", path)
		}
	}
	if s.Settings().Get() != model.DefaultSettings() || len(s.History().List()) != 0 {
		t.Fatal("corrupt preferences/history did not fall back to defaults")
	}
	if got := s.Collections().List(); len(got) != 1 || got[0].ID != validCol.ID {
		t.Fatalf("healthy collection was not loaded: %+v", got)
	}
	if got := s.Environments().List(); len(got) != 1 || got[0].ID != validEnv.ID {
		t.Fatalf("healthy environment was not loaded: %+v", got)
	}
	errs := s.LoadErrors()
	errs[0] = nil
	if s.LoadErrors()[0] == nil {
		t.Fatal("LoadErrors returned internal slice")
	}
}

func TestReadAndWriteValuesAreDeepCopies(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	r := request("original")
	want := r.Clone()
	must(t, s.History().Add(model.HistoryEntry{Request: r}))
	changeRequest(&r)
	assertRequest(t, s.History().List()[0].Request, want)
	entries := s.History().List()
	changeRequest(&entries[0].Request)
	assertRequest(t, s.History().List()[0].Request, want)

	col, err := s.Collections().Create("Copies")
	must(t, err)
	col.Variables = []model.KV{{Key: "key", Value: "original"}}
	must(t, s.Collections().Save(col))
	col.Variables[0].Value = "changed"
	saved, err := s.Collections().SaveRequest(col.ID, "", want)
	must(t, err)
	want.ID = saved.ID
	changeRequest(&saved)
	changeRequest(&r)
	for _, fromList := range []bool{false, true} {
		var got model.Collection
		if fromList {
			got = s.Collections().List()[0]
		} else {
			got, _ = s.Collections().Get(col.ID)
		}
		assertRequest(t, *got.Items[0].Request, want)
		if got.Variables[0].Value != "original" {
			t.Fatal("collection variables aliased input or prior read")
		}
		got.Variables[0].Value = "changed"
		changeRequest(got.Items[0].Request)
		got.Items[0].Name = "changed"
	}
	got, _ := s.Collections().Get(col.ID)
	assertRequest(t, *got.Items[0].Request, want)
	// Save must detach the full input tree, including Request pointers.
	must(t, s.Collections().Save(got))
	changeRequest(got.Items[0].Request)
	got, _ = s.Collections().Get(col.ID)
	assertRequest(t, *got.Items[0].Request, want)

	env, err := s.Environments().Create("Copies")
	must(t, err)
	env.Variables = []model.KV{{Key: "key", Value: "original"}}
	must(t, s.Environments().Save(env))
	env.Variables[0].Value = "changed"
	get, _ := s.Environments().Get(env.ID)
	if get.Variables[0].Value != "original" {
		t.Fatal("environment Save retained input slice")
	}
	get.Variables[0].Value = "changed"
	list := s.Environments().List()
	if list[0].Variables[0].Value != "original" {
		t.Fatal("environment Get returned internal slice")
	}
	list[0].Variables[0].Value = "changed"
	get, _ = s.Environments().Get(env.ID)
	if get.Variables[0].Value != "original" {
		t.Fatal("environment List returned internal slice")
	}
}

func TestInvalidNamesAndMissingParentsDoNotChangeData(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	col, err := s.Collections().Create("Valid")
	must(t, err)
	env, err := s.Environments().Create("Valid")
	must(t, err)
	checks := map[string]func() error{
		"create collection":  func() error { _, err := s.Collections().Create(" \t"); return err },
		"create environment": func() error { _, err := s.Environments().Create(""); return err },
		"save collection":    func() error { col.Name = ""; return s.Collections().Save(col) },
		"save environment":   func() error { env.Name = ""; return s.Environments().Save(env) },
		"add folder":         func() error { _, err := s.Collections().AddFolder(col.ID, "", ""); return err },
		"save request":       func() error { _, err := s.Collections().SaveRequest(col.ID, "", model.NewRequest("")); return err },
		"rename item":        func() error { return s.Collections().RenameItem(col.ID, "missing", "") },
		"import":             func() error { _, err := s.Collections().Import(model.Collection{}); return err },
		"missing parent":     func() error { _, err := s.Collections().AddFolder(col.ID, "missing", "Folder"); return err },
		"missing collection": func() error { _, err := s.Collections().SaveRequest("missing", "", request("Request")); return err },
		"missing item":       func() error { return s.Collections().DeleteItem(col.ID, "missing") },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	s = openStore(t, dir)
	if got := s.Collections().List(); len(got) != 1 || got[0].Name != "Valid" || len(got[0].Items) != 0 {
		t.Fatalf("invalid mutation changed collections: %+v", got)
	}
	if got := s.Environments().List(); len(got) != 1 || got[0].Name != "Valid" {
		t.Fatalf("invalid mutation changed environments: %+v", got)
	}
}

func TestFailedWritesLeaveMemoryUnchanged(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	col, err := s.Collections().Create("Before")
	must(t, err)
	// A directory at the destination works as a portable write failure on
	// Windows as well as Unix, unlike permission bits under an elevated user.
	path := filepath.Join(dir, "collections", col.ID+".json")
	must(t, os.Remove(path))
	must(t, os.Mkdir(path, 0o755))
	if _, err := s.Collections().AddFolder(col.ID, "", "After"); err == nil {
		t.Fatal("expected persistence failure")
	}
	got, _ := s.Collections().Get(col.ID)
	if len(got.Items) != 0 {
		t.Fatal("failed folder write changed memory")
	}
	must(t, os.Mkdir(filepath.Join(dir, "history.json"), 0o755))
	if err := s.History().Add(model.HistoryEntry{}); err == nil || len(s.History().List()) != 0 {
		t.Fatal("failed history write changed memory or reported success")
	}
	must(t, os.Mkdir(filepath.Join(dir, "settings.json"), 0o755))
	if err := s.Settings().Save(model.Settings{Theme: "dark"}); err == nil || s.Settings().Get() != model.DefaultSettings() {
		t.Fatal("failed settings write changed memory or reported success")
	}
}

func TestFilesCarrySchemaVersion(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	must(t, s.Settings().Save(model.Settings{}))
	must(t, s.History().Add(model.HistoryEntry{Request: request("Saved")}))
	col, err := s.Collections().Create("Collection")
	must(t, err)
	env, err := s.Environments().Create("Environment")
	must(t, err)
	for _, path := range []string{"settings.json", "collections/" + col.ID + ".json", "environments/" + env.ID + ".json"} {
		var file struct{ Version int }
		must(t, storage.ReadJSON(filepath.Join(dir, path), &file))
		if file.Version != model.SchemaVersion {
			t.Fatalf("%s version = %d", path, file.Version)
		}
	}
	var history []struct{ Version int }
	must(t, storage.ReadJSON(filepath.Join(dir, "history.json"), &history))
	if len(history) != 1 || history[0].Version != model.SchemaVersion {
		t.Fatalf("history array version = %+v", history)
	}
}

func TestConcurrentServicesDoNotLoseMutations(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			if err := s.History().Add(model.HistoryEntry{ID: fmt.Sprint(i), Request: request("Concurrent")}); err != nil {
				t.Error(err)
			}
			if _, err := s.Collections().Create(fmt.Sprint(i)); err != nil {
				t.Error(err)
			}
			if _, err := s.Environments().Create(fmt.Sprint(i)); err != nil {
				t.Error(err)
			}
			_ = s.History().List()
			_ = s.Collections().List()
			_ = s.Environments().List()
			_ = s.Settings().Get()
			_ = s.LoadErrors()
		})
	}
	wg.Wait()
	s = openStore(t, dir)
	if len(s.History().List()) != 12 || len(s.Collections().List()) != 12 || len(s.Environments().List()) != 12 {
		t.Fatal("concurrent mutations were lost")
	}
}

func TestIDsCannotEscapeDataDirectory(t *testing.T) {
	s := openStore(t, t.TempDir())
	for _, id := range []string{"", "..", "../outside", `..\outside`, "stream:bad"} {
		if err := s.Collections().Save(model.Collection{ID: id, Name: "Invalid"}); err == nil {
			t.Fatalf("collection accepted invalid ID %q", id)
		}
		if err := s.Environments().Save(model.Environment{ID: id, Name: "Invalid"}); err == nil {
			t.Fatalf("environment accepted invalid ID %q", id)
		}
		if err := s.Collections().Delete(id); err == nil {
			t.Fatalf("collection Delete accepted invalid ID %q", id)
		}
		if err := s.Environments().Delete(id); err == nil {
			t.Fatalf("environment Delete accepted invalid ID %q", id)
		}
	}
	if _, err := s.Collections().AddFolder("absent", "", "Folder"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing collection error = %v", err)
	}
}
