package auth

import (
	"encoding/json"
	"testing"

	"github.com/runbrick/reqo/internal/model"
)

func TestResolveInheritance(t *testing.T) {
	root := model.Auth{Type: model.AuthBearer, Token: "{{root_token}}"}
	shared := model.Auth{Type: model.AuthBasic, Username: "{{user}}", Password: ""}
	col := model.Collection{ID: "collection", Name: "API", Auth: root, Items: []model.Item{
		{ID: "direct", Type: model.ItemRequest, Auth: shared}, // Request item auth is ignored.
		{ID: "folder", Name: "Shared", Type: model.ItemFolder, Auth: shared, Items: []model.Item{
			{ID: "implicit", Type: model.ItemFolder, Items: []model.Item{
				{ID: "deep", Type: model.ItemRequest},
			}},
			{ID: "inherit", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthInherit}, Items: []model.Item{
				{ID: "nested", Type: model.ItemRequest},
			}},
			{ID: "stop", Name: "Public", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthNone}, Items: []model.Item{
				{ID: "public", Type: model.ItemRequest},
			}},
		}},
	}}
	before, err := json.Marshal(col)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id     string
		want   model.Auth
		source Source
	}{
		{"direct", root, Source{Kind: SourceCollection, ID: "collection", Name: "API"}},
		{"folder", root, Source{Kind: SourceCollection, ID: "collection", Name: "API"}}, // Parent preview excludes own folder.
		{"implicit", shared, Source{Kind: SourceFolder, ID: "folder", Name: "Shared"}},
		{"deep", shared, Source{Kind: SourceFolder, ID: "folder", Name: "Shared"}},
		{"nested", shared, Source{Kind: SourceFolder, ID: "folder", Name: "Shared"}},
		{"stop", shared, Source{Kind: SourceFolder, ID: "folder", Name: "Shared"}},
		{"public", model.Auth{Type: model.AuthNone}, Source{Kind: SourceFolder, ID: "stop", Name: "Public"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			got, source, err := Resolve(col, tc.id, model.Auth{Type: model.AuthInherit})
			if err != nil || got != tc.want || source != tc.source {
				t.Fatalf("Resolve = %#v, %#v, %v; want %#v, %#v", got, source, err, tc.want, tc.source)
			}
		})
	}
	after, _ := json.Marshal(col)
	if string(before) != string(after) {
		t.Fatal("Resolve mutated the collection")
	}
	for _, cfg := range []model.Auth{{}, {Type: model.AuthNone}, {Type: model.AuthInherit}} {
		col.Auth = cfg
		got, source, err := Resolve(col, "direct", model.Auth{Type: model.AuthInherit})
		if err != nil || got != (model.Auth{Type: model.AuthNone}) || source.Kind != SourceCollection {
			t.Fatalf("empty/no-auth/inheriting root = %#v, %#v, %v", got, source, err)
		}
	}
}

func TestResolveExplicitAndMissingContext(t *testing.T) {
	for _, own := range []model.Auth{{}, {Type: model.AuthNone}, {Type: model.AuthBearer, Token: "{{secret}}"}, {Type: model.AuthBasic, Username: "user"}, {Type: model.AuthAPIKey, Key: "X-Key", Value: "key"}, {Type: "future", Token: "unknown"}} {
		got, source, err := Resolve(model.Collection{}, "unsaved", own)
		want := own
		if want.Type == "" {
			want = model.Auth{Type: model.AuthNone}
		}
		if err != nil || got != want || source != (Source{Kind: SourceRequest, ID: "unsaved"}) {
			t.Fatalf("explicit %q = %#v, %#v, %v", own.Type, got, source, err)
		}
	}
	col := model.Collection{ID: "collection", Auth: model.Auth{Type: model.AuthBearer, Token: "secret"}, Items: []model.Item{{ID: "member", Type: model.ItemRequest}}}
	for _, tc := range []struct {
		col model.Collection
		id  string
	}{
		{model.Collection{}, "member"},
		{col, ""},
		{col, "missing"},
	} {
		if _, _, err := Resolve(tc.col, tc.id, model.Auth{Type: model.AuthInherit}); err == nil {
			t.Fatal("inheritance without exact collection membership succeeded")
		}
	}
	col.Items = []model.Item{{ID: "unknown", Type: model.ItemFolder, Auth: model.Auth{Type: "future"}, Items: []model.Item{{ID: "member", Type: model.ItemRequest}}}}
	got, _, err := Resolve(col, "member", model.Auth{Type: model.AuthInherit})
	if err != nil || got.Type != "future" || Validate(got) == nil {
		t.Fatal("unknown explicit folder auth must be preserved and fail validation")
	}
}
