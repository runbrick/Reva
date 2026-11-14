package auth

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
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

func TestValidationRejectsInvalidAuthWithoutCredentials(t *testing.T) {
	for _, cfg := range []model.Auth{
		{Type: model.AuthInherit, Token: "credential-marker"},
		{Type: "credential-marker"},
		{Type: model.AuthBearer, Token: "credential-marker\r\nInjected: yes"},
		{Type: model.AuthBearer, Token: "credential-marker\x00"},
		{Type: model.AuthAPIKey, Key: "credential-marker bad", Value: "credential-marker"},
		{Type: model.AuthAPIKey, Key: "", Value: "credential-marker"},
		{Type: model.AuthAPIKey, Key: "X-Key", Value: "credential-marker\x7f"},
		{Type: model.AuthAPIKey, Key: "X-Key", Value: "credential-marker", In: "credential-marker"},
	} {
		t.Run(string(cfg.Type)+"/"+cfg.Key+"/"+string(cfg.In), func(t *testing.T) {
			h, q := http.Header{"X-Existing": {"before"}}, url.Values{"existing": {"before"}}
			for _, err := range []error{Validate(cfg), ApplyHeaders(h, cfg), ApplyQuery(q, cfg)} {
				if err == nil || strings.Contains(err.Error(), "credential-marker") {
					t.Fatalf("invalid auth accepted or credentials reflected: %v", err)
				}
			}
			if !reflect.DeepEqual(h, http.Header{"X-Existing": {"before"}}) || !reflect.DeepEqual(q, url.Values{"existing": {"before"}}) {
				t.Fatal("invalid auth partially modified request")
			}
		})
	}
}

func TestAuthHelpersPreserveExplicitValuesAndQueryMultiset(t *testing.T) {
	for _, cfg := range []model.Auth{
		{Type: model.AuthBearer, Token: "generated"},
		{Type: model.AuthBasic, Username: "u"},
		{Type: model.AuthAPIKey, Key: "Authorization", Value: "generated"},
	} {
		h := http.Header{"aUtHoRiZaTiOn": {"", "first", "second"}}
		before := h.Clone()
		if err := ApplyHeaders(h, cfg); err != nil || !reflect.DeepEqual(h, before) {
			t.Fatalf("explicit values lost: %#v, %v", h, err)
		}
	}
	h := make(http.Header)
	if err := ApplyHeaders(h, model.Auth{Type: model.AuthBasic, Username: "u"}); err != nil || h.Get("Authorization") != "Basic dTo=" {
		t.Fatalf("empty Basic password: %#v, %v", h, err)
	}
	q := url.Values{"key": {"old", "duplicate"}, "keep": {"a", "a", "b"}}
	before := q.Encode()
	if err := ApplyQuery(q, model.Auth{Type: model.AuthAPIKey, Key: "key", Value: "header"}); err != nil || q.Encode() != before {
		t.Fatal("default header mode modified query")
	}
	if err := ApplyQuery(q, model.Auth{Type: model.AuthAPIKey, Key: "key", Value: "a & b", In: model.APIKeyInQuery}); err != nil || !reflect.DeepEqual(q, url.Values{"key": {"a & b"}, "keep": {"a", "a", "b"}}) {
		t.Fatalf("query Set lost unrelated repetitions: %#v, %v", q, err)
	}
	if ApplyHeaders(nil, model.Auth{Type: model.AuthBearer}) == nil || ApplyQuery(nil, model.Auth{Type: model.AuthAPIKey, In: model.APIKeyInQuery}) == nil {
		t.Fatal("nil destination must fail without panic")
	}
}

func TestResolveDoesNotTraverseRequestChildrenOrMatchPrefixes(t *testing.T) {
	col := model.Collection{ID: "root", Name: "Visible name", Auth: model.Auth{Type: model.AuthBearer, Token: "private-marker"}, Items: []model.Item{
		{ID: "member-suffix", Type: model.ItemRequest, Items: []model.Item{{ID: "member", Type: model.ItemRequest}}},
		{ID: "other", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthNone}, Items: []model.Item{{ID: "sibling", Type: model.ItemRequest}}},
	}}
	if _, _, err := Resolve(col, "member", model.Auth{Type: model.AuthInherit}); err == nil || strings.Contains(err.Error(), "private-marker") {
		t.Fatalf("nonmember resolved or credential leaked: %v", err)
	}
	cfg, source, err := Resolve(col, "member-suffix", model.Auth{Type: model.AuthInherit})
	if err != nil || cfg != col.Auth || source != (Source{Kind: SourceCollection, ID: "root", Name: "Visible name"}) {
		t.Fatalf("sibling barrier affected exact member: %#v %#v %v", cfg, source, err)
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
