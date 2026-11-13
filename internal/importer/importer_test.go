package importer

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/auth"
	"github.com/runbrick/reqo/internal/model"
)

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCurlFixtures(t *testing.T) {
	var cases []struct {
		Name, Command string
		Want          model.Request
		Error         bool
	}
	if err := json.Unmarshal(fixture(t, "curl.json"), &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Curl(tc.Command)
			if (err != nil) != tc.Error {
				t.Fatalf("Curl error = %v; want error %v", err, tc.Error)
			}
			if tc.Error {
				return
			}
			if got.ID == "" {
				t.Fatal("request has no ID")
			}
			got.ID = ""
			if tc.Want.Body.Mode == "" {
				tc.Want.Body.Mode = model.BodyNone
			}
			if tc.Want.Auth.Type == "" {
				tc.Want.Auth.Type = model.AuthNone
			}
			if !reflect.DeepEqual(got, tc.Want) {
				t.Errorf("got  %#v\nwant %#v", got, tc.Want)
			}
		})
	}
}

func TestPostmanFixtures(t *testing.T) {
	for _, tc := range []struct {
		file, name string
		requests   int
	}{
		{"postman-v21.json", "示例 API", 7},
		{"postman-v20.json", "Legacy API", 3},
	} {
		t.Run(tc.file, func(t *testing.T) {
			col, err := Postman(fixture(t, tc.file))
			if err != nil {
				t.Fatal(err)
			}
			if col.Name != tc.name || col.Version != model.SchemaVersion || col.ID == "" {
				t.Fatalf("collection metadata = %#v", col)
			}
			seen := map[string]bool{col.ID: true}
			var walk func([]model.Item)
			walk = func(items []model.Item) {
				for _, item := range items {
					if item.ID == "" || seen[item.ID] {
						t.Fatalf("missing or duplicate ID: %q", item.ID)
					}
					seen[item.ID] = true
					if item.Request != nil && (item.Request.ID != item.ID || item.Request.Name != item.Name) {
						t.Fatal("item and request identity differ")
					}
					walk(item.Items)
				}
			}
			walk(col.Items)
			requests := 0
			model.WalkRequests(col.Items, func(*model.Request) { requests++ })
			if requests != tc.requests {
				t.Fatalf("got %d requests, want %d", requests, tc.requests)
			}
			second, err := Postman(fixture(t, tc.file))
			if err != nil || col.ID == second.ID || col.Items[0].ID == second.Items[0].ID {
				t.Fatal("reimport must assign fresh IDs")
			}
		})
	}
}

func TestPostmanV21Contents(t *testing.T) {
	col, err := Postman(fixture(t, "postman-v21.json"))
	if err != nil {
		t.Fatal(err)
	}
	vars := []model.KV{{Key: "baseUrl", Value: "https://example.test"}, {Key: "token", Value: "{{secret}}", Disabled: true}, {Key: "limit", Value: "10"}}
	if !reflect.DeepEqual(col.Variables, vars) {
		t.Fatalf("variables = %#v", col.Variables)
	}
	if col.Items[0].Type != model.ItemFolder || col.Items[0].Name != "用户" || col.Items[0].Items[1].Type != model.ItemFolder {
		t.Fatal("nested folders lost")
	}
	requests := make(map[string]model.Request)
	model.WalkRequests(col.Items, func(r *model.Request) { requests[r.Name] = *r })
	rootAuth := model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
	folderAuth := model.Auth{Type: model.AuthBasic, Username: "{{user}}", Password: "p:a:ss"}
	if col.Auth != rootAuth || col.Items[0].Auth.Type != model.AuthInherit || col.Items[0].Items[1].Auth != folderAuth {
		t.Fatal("collection/folder authentication was not preserved")
	}
	list := requests["List users"]
	if list.URL != "{{baseUrl}}/users" || list.Auth.Type != model.AuthInherit || list.Body.Mode != model.BodyNone {
		t.Fatalf("list request = %#v", list)
	}
	if effective, source, err := auth.Resolve(col, list.ID, list.Auth); err != nil || effective != rootAuth || source.Kind != auth.SourceCollection || source.ID != col.ID {
		t.Fatalf("list effective auth = %#v, %#v, %v", effective, source, err)
	}
	if !reflect.DeepEqual(list.Params, []model.KV{{Key: "q", Value: "{{search}}"}, {Key: "unused", Value: "1", Disabled: true}}) || len(list.Headers) != 2 || !list.Headers[1].Disabled {
		t.Fatal("query/header disabled state lost")
	}
	create := requests["Create user"]
	if create.URL != "https://api.example.test:{{port}}/v1/users" || create.Method != model.MethodPost || create.Auth.Type != model.AuthInherit || create.Body.Mode != model.BodyJSON || create.Body.Text != `{"name":"{{name}}"}` {
		t.Fatalf("create request = %#v", create)
	}
	if effective, source, err := auth.Resolve(col, create.ID, create.Auth); err != nil || effective != folderAuth || source.Kind != auth.SourceFolder || source.ID != col.Items[0].Items[1].ID {
		t.Fatalf("create effective auth = %#v, %#v, %v", effective, source, err)
	}
	text := requests["Text"]
	if text.Method != model.MethodPatch || text.Auth.Type != model.AuthNone || text.Body.Mode != model.BodyRaw || text.Body.Text != "hello {{name}}" || text.URL != "{{baseUrl}}/text#result" || len(text.Params) != 2 || text.Params[0].Value != "a b" {
		t.Fatalf("text request = %#v", text)
	}
	login := requests["Login"]
	if login.Body.Mode != model.BodyURLEncoded || len(login.Body.URLEncoded) != 2 || !login.Body.URLEncoded[1].Disabled || login.Auth != (model.Auth{Type: model.AuthAPIKey, Key: "api_key", Value: "{{key}}", In: model.APIKeyInQuery}) {
		t.Fatalf("login request = %#v", login)
	}
	upload := requests["Upload"]
	wantForm := []model.FormField{{Key: "note", Value: "你好"}, {Key: "file", Value: `C:\uploads\one.png`, File: true}, {Key: "files", Value: "two.png", File: true, Disabled: true}, {Key: "files", Value: "three.png", File: true, Disabled: true}}
	if upload.Body.Mode != model.BodyFormData || !reflect.DeepEqual(upload.Body.Form, wantForm) {
		t.Fatalf("form = %#v", upload.Body)
	}
	binary := requests["Binary"]
	if binary.URL != "https://example.test/binary" || binary.Body.Mode != model.BodyBinary || binary.Body.FilePath != "/tmp/body.bin" {
		t.Fatalf("binary = %#v", binary)
	}
	if r := requests["URL only"]; r.Method != model.MethodGet || r.URL != "{{baseUrl}}/ping" || len(r.Params) != 2 || r.Params[1].Key != "flag" {
		t.Fatalf("URL only = %#v", r)
	}
	if empty := col.Items[len(col.Items)-1]; empty.Type != model.ItemFolder || len(empty.Items) != 0 {
		t.Fatal("empty folder lost")
	}
}

func TestPostmanAuthRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name, array, object string
		want                model.Auth
	}{
		{"bearer", `[{"key":"token","value":"{{token}}"}]`, `{"token":"{{token}}"}`, model.Auth{Type: model.AuthBearer, Token: "{{token}}"}},
		{"basic", `[{"key":"username","value":"u"},{"key":"password","value":"p"}]`, `{"username":"u","password":"p"}`, model.Auth{Type: model.AuthBasic, Username: "u", Password: "p"}},
		{"apikey", `[{"key":"key","value":"X-Key"},{"key":"value","value":"v"},{"key":"in","value":"header"}]`, `{"key":"X-Key","value":"v","in":"header"}`, model.Auth{Type: model.AuthAPIKey, Key: "X-Key", Value: "v", In: model.APIKeyInHeader}},
	} {
		for _, fields := range []string{tc.array, tc.object} {
			t.Run(tc.name+fields[:1], func(t *testing.T) {
				input := fmt.Sprintf(`{"info":{"name":"Auth"},"item":[{"name":"Request","request":{"url":"https://example.test","auth":{"type":%q,%q:%s}}}]}`, tc.name, tc.name, fields)
				col, err := Postman([]byte(input))
				if err != nil {
					t.Fatal(err)
				}
				if got := col.Items[0].Request.Auth; got != tc.want {
					t.Fatalf("auth = %#v; want %#v", got, tc.want)
				}
			})
		}
	}
}

func TestPostmanV20Contents(t *testing.T) {
	col, err := Postman(fixture(t, "postman-v20.json"))
	if err != nil {
		t.Fatal(err)
	}
	rootAuth := model.Auth{Type: model.AuthBasic, Username: "{{user}}", Password: "secret"}
	if col.Auth != rootAuth {
		t.Fatal("legacy collection auth lost")
	}
	if r := col.Items[0].Request; r.Auth.Type != model.AuthInherit || !reflect.DeepEqual(r.Headers, []model.KV{{Key: "Accept", Value: "application/json"}, {Key: "X-Test", Value: "yes"}}) {
		t.Fatalf("legacy basic request = %#v", r)
	}
	if effective, source, err := auth.Resolve(col, col.Items[0].ID, col.Items[0].Request.Auth); err != nil || effective != rootAuth || source.Kind != auth.SourceCollection || source.ID != col.ID {
		t.Fatalf("legacy effective auth = %#v, %#v, %v", effective, source, err)
	}
	if col.Items[1].Request.Auth != (model.Auth{Type: model.AuthBearer, Token: "{{token}}"}) || col.Items[2].Request.Auth != (model.Auth{Type: model.AuthAPIKey, Key: "X-API-Key", Value: "{{key}}", In: model.APIKeyInHeader}) {
		t.Fatal("legacy auth fields lost")
	}
}

func TestPostmanRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{
		``, `null`, `[]`, `{}`, `{"info":{"name":"x"}}`, `{"info":{"name":"x"},"item":null}`,
		`{"info":{"name":"x","schema":"v1"},"item":[]}`,
		`{"info":{"name":"x"},"item":[{"request":42}]}`,
		`{"info":{"name":"x"},"item":[{"request":{"url":[]}}]}`,
		`{"info":{"name":"x"},"item":[{"request":{"header":42}}]}`,
		`{"info":{"name":"x"},"item":[],"auth":{"type":"basic","basic":42}}`,
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := Postman([]byte(input)); err == nil {
				t.Fatal("accepted invalid collection")
			}
		})
	}
	deep := `{"name":"leaf","request":"https://example.test"}`
	for range 260 {
		deep = `{"name":"folder","item":[` + deep + `]}`
	}
	if _, err := Postman([]byte(`{"info":{"name":"x"},"item":[` + deep + `]}`)); err == nil {
		t.Fatal("accepted excessively deep nesting")
	}
}

func TestPostmanUnknownPartsAndEmptyCollection(t *testing.T) {
	col, err := Postman([]byte(`{"info":{"name":"Empty"},"item":[],"future":{"anything":true}}`))
	if err != nil || col.Name != "Empty" || len(col.Items) != 0 {
		t.Fatalf("empty collection = %#v, %v", col, err)
	}
	col, err = Postman([]byte(`{"info":{"name":"Future"},"item":[{"name":"ignored","future":{}},{"name":"request","request":{"method":"GET","url":{"raw":"{{baseUrl}}?old=value","query":[]},"body":{"mode":"future","future":{}},"auth":{"type":"future"}}}]}`))
	if err != nil || len(col.Items) != 1 {
		t.Fatalf("unknown fields = %#v, %v", col, err)
	}
	if r := col.Items[0].Request; r.Body.Mode != model.BodyNone || r.Auth.Type != model.AuthNone || r.URL != "{{baseUrl}}" || len(r.Params) != 0 {
		t.Fatalf("unknown fields changed request = %#v", r)
	}
}

func TestCurlEscapingEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name, command, header string
	}{
		{"cmd apostrophe in double quotes", "curl \"https://example.test\" -H \"X-Name: it's here\" ^\n --silent", "it's here"},
		{"cmd escaped space", `curl https://example.test -H X-Name:^ hello`, "hello"},
		{"bash literal caret", `curl https://example.test -H 'X-Name: a^&b'`, "a^&b"},
		{"bash double quoted caret", `curl https://example.test -H "X-Name: a^b"`, "a^b"},
		{"bash CRLF continuation", "curl https://example.test \\\r\n -H 'X-Name: hello'", "hello"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Curl(tc.command)
			if err != nil || len(r.Headers) != 1 || r.Headers[0].Value != tc.header {
				t.Fatalf("Curl = %#v, %v", r, err)
			}
		})
	}
	for _, command := range []string{
		`curl https://example.test --data-binary @`,
		`curl https://example.test --data-binary @-`,
		`curl https://example.test --data-binary @one --data-binary @two`,
		`curl https://example.test --request ''`,
		`curl https://example.test --header invalid`,
		"curl https://example.test ^\n -H ^",
	} {
		if _, err := Curl(command); err == nil {
			t.Errorf("accepted invalid command %q", command)
		}
	}
}

func TestPostmanHierarchicalAuthBarriersAndLiveParents(t *testing.T) {
	for _, version := range []string{"v2.0.0", "v2.1.0"} {
		t.Run(version, func(t *testing.T) {
			input := fmt.Sprintf(`{"info":{"name":"API","schema":"https://schema.getpostman.com/json/collection/%s/collection.json"},"auth":{"type":"bearer","bearer":{"token":"{{root}}"}},"item":[
			{"name":"Shared","auth":{"type":"basic","basic":{"username":"u","password":""}},"item":[
			{"name":"omitted","request":{"url":"https://example.test?x=a&x=b","header":[{"key":"X-Off","value":"kept","disabled":true}]}},
			{"name":"null","request":{"url":"https://example.test","auth":null}},
			{"name":"url-only","request":"https://example.test?x=1"},
			{"name":"none","request":{"url":"https://example.test","auth":{"type":"noauth"}}},
			{"name":"unknown","request":{"url":"https://example.test","auth":{"type":"oauth2"}}},
			{"name":"public","auth":{"type":"noauth"},"item":[{"name":"child","request":"https://example.test"}]},
			{"name":"unsupported","auth":{"type":"digest"},"item":[{"name":"child","request":"https://example.test"}]},
			{"name":"transparent","auth":null,"item":[{"name":"child","request":"https://example.test"}]}
			]}]}`, version)
			col, err := Postman([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			folder := &col.Items[0]
			for i, item := range folder.Items {
				if item.Type == model.ItemFolder {
					item = item.Items[0]
				}
				want := model.Auth{Type: model.AuthBasic, Username: "u"}
				if i == 3 || i == 4 || i == 5 || i == 6 {
					want = model.Auth{Type: model.AuthNone}
				}
				got, _, err := auth.Resolve(col, item.ID, item.Request.Auth)
				if err != nil || got != want {
					t.Fatalf("%s effective auth=%#v %v, want %#v", item.Name, got, err, want)
				}
				if i != 3 && i != 4 && item.Request.Auth.Type != model.AuthInherit {
					t.Fatalf("%s copied parent instead of inheriting", item.Name)
				}
			}
			if len(folder.Items[0].Request.Params) != 2 || !folder.Items[0].Request.Headers[0].Disabled {
				t.Fatal("auth import lost unrelated URL/disabled fields")
			}
			folder.Auth = model.Auth{Type: model.AuthInherit}
			col.Auth.Token = "{{edited}}"
			r := folder.Items[0].Request
			got, src, err := auth.Resolve(col, r.ID, r.Auth)
			if err != nil || got != col.Auth || src.ID != col.ID {
				t.Fatalf("parent edit not live after import: %#v %#v %v", got, src, err)
			}
		})
	}
}

func TestPostmanInvalidAPIKeyPlacementRemainsInvalid(t *testing.T) {
	col, err := Postman([]byte(`{"info":{"name":"Invalid placement"},"auth":{"type":"apikey","apikey":{"key":"X-Key","value":"private-marker","in":"cookie"}},"item":[{"name":"Inherited","request":"https://example.test"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	r := col.Items[0].Request
	resolved, _, err := auth.Resolve(col, r.ID, r.Auth)
	if err != nil || resolved.In != "cookie" || auth.Validate(resolved) == nil {
		t.Fatalf("invalid imported auth was silently changed: %#v %v", resolved, err)
	}
}

func TestDetect(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  Kind
	}{
		{string(fixture(t, "postman-v21.json")), KindPostman},
		{string(fixture(t, "postman-v20.json")), KindPostman},
		{"\ufeff" + string(fixture(t, "postman-v21.json")), KindPostman},
		{" \n curl --url https://example.test ", KindCurl},
		{`curl.exe "https://example.test"`, KindCurl},
		{`curl 'unfinished`, KindCurl},
		{`{"hello":"world"}`, KindUnknown},
		{"curling something", KindUnknown}, {"", KindUnknown}, {"{}", KindUnknown},
	} {
		if got := Detect(tc.input); got != tc.want {
			t.Errorf("Detect(%q) = %s, want %s", tc.input, got, tc.want)
		}
	}
}

func FuzzImportNeverPanics(f *testing.F) {
	f.Add(string(fixture(f, "postman-v21.json")))
	f.Add(string(fixture(f, "postman-v20.json")))
	f.Add(`curl 'https://example.test' --data '{"x":1}'`)
	f.Add("curl \"https://example.test\" ^\n --data-raw \"a=1\"")
	f.Add(strings.Repeat("[", 100))
	f.Fuzz(func(t *testing.T, input string) {
		Detect(input)
		_, _ = Curl(input)
		_, _ = Postman([]byte(input))
	})
}
