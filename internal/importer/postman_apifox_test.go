package importer

import (
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
)

func TestPostmanApifoxEmptyCollection(t *testing.T) {
	// Anonymized shape of an Apifox export: empty auth and no item property.
	for _, version := range []string{"v2.0.0", "v2.1.0"} {
		t.Run(version, func(t *testing.T) {
			input := `{"info":{"name":"Empty Apifox project","schema":"https://schema.getpostman.com/json/collection/` + version + `/collection.json"},"variable":[],"event":[],"auth":{}}`
			if Detect(input) != KindPostman {
				t.Fatal("empty export was not detected as Postman")
			}
			result := parseOK(t, input)
			col := result.Collection
			if result.Kind != KindPostman || col.Name != "Empty Apifox project" || col.ID == "" || col.Version != model.SchemaVersion || len(col.Items) != 0 || len(col.Variables) != 0 || col.Auth.Type != model.AuthNone {
				t.Fatalf("empty export changed: %#v", result)
			}
			if len(result.Warnings) != 0 {
				t.Fatalf("empty placeholders produced omission warnings: %#v", result.Warnings)
			}
			legacy, err := Postman([]byte(input))
			if err != nil || legacy.ID == col.ID || len(legacy.Items) != 0 {
				t.Fatalf("legacy import failed or reused identity: %v", err)
			}
			// A present but malformed item must not become an empty collection.
			for _, items := range []string{"null", "{}", `""`, "false"} {
				e := parseFails(t, strings.TrimSuffix(input, "}")+`,"item":`+items+`}`, "invalid_structure")
				if e.Path != "$.item" {
					t.Fatalf("wrong malformed-item path: %s", e.Path)
				}
			}
		})
	}
}

func TestPostmanApifoxEmptyAuthPreservesInheritance(t *testing.T) {
	for _, root := range []string{`{}`, `{"type":"bearer","bearer":{"token":"{{root}}"}}`} {
		t.Run(root, func(t *testing.T) {
			col := parseOK(t, `{"info":{"name":"Apifox auth"},"auth":`+root+`,"item":[
				{"name":"Direct","request":{"url":"https://example.test/direct","auth":{}}},
				{"name":"Transparent","auth":{},"item":[{"name":"Child","request":{"url":"https://example.test/child","auth":{}}}]},
				{"name":"Private","auth":{"type":"basic","basic":{"username":"{{user}}"}},"item":[
					{"name":"Nested","auth":{},"item":[{"name":"Child","request":{"url":"https://example.test/private","auth":{}}}]}
				]},
				{"name":"Public","auth":{"type":"noauth"},"item":[{"name":"Child","request":{"url":"https://example.test/public","auth":{}}}]}
			]}`).Collection
			if col.Items[1].Auth.Type != model.AuthInherit || col.Items[2].Items[0].Auth.Type != model.AuthInherit {
				t.Fatal("empty folder auth lost inheritance")
			}
			requests := []*model.Request{
				col.Items[0].Request,
				col.Items[1].Items[0].Request,
				col.Items[2].Items[0].Items[0].Request,
				col.Items[3].Items[0].Request,
			}
			for i, request := range requests {
				if request.Auth.Type != model.AuthInherit {
					t.Fatal("empty request auth lost inheritance")
				}
				want := col.Auth
				if i == 2 {
					want = col.Items[2].Auth
				} else if i == 3 {
					want = model.Auth{Type: model.AuthNone}
				}
				got, _, err := auth.Resolve(*col, request.ID, request.Auth)
				if err != nil || got != want {
					t.Fatalf("request %d effective auth = %#v, %v; want %#v", i, got, err, want)
				}
			}
			// Editing a parent must still affect imported inheriting requests.
			col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{edited}}"}
			got, _, err := auth.Resolve(*col, requests[1].ID, requests[1].Auth)
			if err != nil || got != col.Auth {
				t.Fatalf("parent edits no longer flow through empty auth: %#v, %v", got, err)
			}
		})
	}
}

func TestPostmanApifoxNonemptyTypelessAuthStaysInvalid(t *testing.T) {
	for _, invalid := range []string{`{"bearer":[]}`, `{"type":null}`, `{"type":""}`, `{"token":"private-marker"}`} {
		for _, input := range []string{
			`{"info":{"name":"Invalid"},"item":[],"auth":` + invalid + `}`,
			`{"info":{"name":"Invalid"},"item":[{"name":"Folder","item":[],"auth":` + invalid + `}]}`,
			pmRequest("", invalid),
		} {
			parseFails(t, input, "invalid_auth")
		}
	}
}
