package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/model"
)

func TestAuthPersistenceRoundtripAndCopyIsolation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REQO_HOME", dir)
	s := openStore(t, dir)
	r := model.NewRequest("request")
	r.Name, r.Auth = "Inherited", model.Auth{Type: model.AuthInherit}
	want := model.Collection{Version: 1, ID: "collection", Name: "API", Auth: model.Auth{Type: model.AuthBearer, Token: "{{root}}"}, Items: []model.Item{
		{ID: "folder", Name: "Shared", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthAPIKey, Key: "X-{{name}}", Value: "{{key}}"}, Items: []model.Item{{ID: r.ID, Name: r.Name, Type: model.ItemRequest, Request: &r}}},
	}}
	must(t, s.Collections().Save(want))
	got, _ := s.Collections().Get(want.ID)
	got.Auth.Token, got.Items[0].Auth.Value, got.Items[0].Items[0].Request.Auth.Type = "mutated", "mutated", model.AuthNone
	listed := s.Collections().List()
	listed[0].Items[0].Auth.Key = "mutated"
	restored, _ := s.Collections().Get(want.ID)
	if !reflect.DeepEqual(restored, want) {
		t.Fatal("returned collection aliases saved auth tree")
	}
	want.Items[0].Auth.Value = "changed after Save"
	r.Auth = model.Auth{Type: model.AuthNone}
	restored, _ = s.Collections().Get(want.ID)
	if restored.Items[0].Auth.Value != "{{key}}" || restored.Items[0].Items[0].Request.Auth.Type != model.AuthInherit {
		t.Fatal("Save retained aliases to caller's folder/request auth")
	}
	s = openStore(t, dir)
	restored, _ = s.Collections().Get(want.ID)
	if restored.Auth.Token != "{{root}}" || restored.Items[0].Auth.Value != "{{key}}" || restored.Items[0].Items[0].Request.Auth.Type != model.AuthInherit {
		t.Fatalf("reopened auth = %#v", restored)
	}
	data, err := os.ReadFile(filepath.Join(dir, "collections", want.ID+".json"))
	if err != nil || !strings.Contains(string(data), "{{root}}") || !strings.Contains(string(data), "{{key}}") {
		t.Fatalf("placeholders lost at rest: %s %v", data, err)
	}
}

func TestAuthPersistenceLegacyMissingFieldsStayEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REQO_HOME", dir)
	must(t, os.MkdirAll(filepath.Join(dir, "collections"), 0700))
	must(t, os.WriteFile(filepath.Join(dir, "collections", "legacy.json"), []byte(`{"version":1,"id":"legacy","name":"Old","items":[{"type":"folder","id":"f","name":"Folder","items":[{"type":"request","id":"r","name":"Old request","request":{"id":"r","name":"Old request","method":"GET","url":"https://example.test"}}]}]}`), 0600))
	s := openStore(t, dir)
	col, ok := s.Collections().Get("legacy")
	if !ok || col.Auth != (model.Auth{}) || col.Items[0].Auth != (model.Auth{}) || col.Items[0].Items[0].Request.Auth != (model.Auth{}) {
		t.Fatalf("legacy auth changed: %#v", col)
	}
	must(t, s.Collections().Save(col))
	col, _ = openStore(t, dir).Collections().Get("legacy")
	if col.Items[0].Items[0].Request.Auth.Type != "" {
		t.Fatal("saving legacy data silently opted request into inheritance")
	}
}
