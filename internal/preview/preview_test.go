package preview_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/preview"
	"github.com/runbrick/reva/internal/service/request"
)

func TestPreviewLastEnabledWinsAndDisabledEnvironmentFallsBack(t *testing.T) {
	col := model.Collection{Variables: []model.KV{
		{Key: "host", Value: "first-host"}, {Key: "host", Value: "last-host"},
		{Key: "token", Value: "collection-token"},
	}}
	env := model.Environment{Name: "Staging", Variables: []model.KV{
		{Key: "host", Value: "disabled-host", Disabled: true},
		{Key: "token", Value: "first-token"}, {Key: "token", Value: "last-token"},
		{Key: "token", Value: "disabled-token", Disabled: true},
	}}
	r := model.NewRequest("request")
	r.URL = "https://{{ host }}/{{missing}}"
	s := preview.Build(r, col, env)
	want := []preview.State{preview.Shadowed, preview.Winner, preview.Shadowed, preview.Disabled, preview.Shadowed, preview.Winner, preview.Disabled}
	for i, state := range want {
		if s.Variables[i].State != state {
			t.Errorf("row %d state = %s, want %s", i, s.Variables[i].State, state)
		}
	}
	if s.Variables[3].Scope != preview.Environment || s.Variables[3].Row != 1 || s.Variables[1].Scope != preview.Collection {
		t.Fatalf("wrong provenance: %+v", s.Variables)
	}
	if s.URL != "https://[masked]/{{missing}}" || !slices.Equal(s.Unresolved, []string{"missing"}) {
		t.Fatalf("routing/unknown names = %q, %v", s.URL, s.Unresolved)
	}
	env.Variables[0].Disabled = false
	updated := preview.Build(r, col, env)
	if updated.Variables[1].State != preview.Shadowed || updated.Variables[3].State != preview.Winner {
		t.Fatal("enabled environment did not override the collection")
	}
}

func TestPreviewMasksCredentialsAndAllVariableValuesWithoutMutation(t *testing.T) {
	for _, cfg := range []model.Auth{
		{Type: model.AuthBasic, Username: "basic-user-secret", Password: "basic-password-secret"},
		{Type: model.AuthBearer, Token: "bearer-secret"},
		{Type: model.AuthAPIKey, Key: "X-Credential", Value: "apikey-secret"},
		{Type: model.AuthAPIKey, In: model.APIKeyInQuery, Key: "credential", Value: "query-auth-secret"},
	} {
		t.Run(string(cfg.Type)+string(cfg.In), func(t *testing.T) {
			r := model.NewRequest("request")
			r.Auth = cfg
			r.URL = "https://url-user-secret:url-password-secret@example.test/{{route}}?password=literal-query-secret&normal={{plain}}#fragment-secret"
			r.Headers = []model.KV{{Key: "Authorization", Value: "literal-authorization-secret"}, {Key: "Proxy-Authorization", Value: "proxy-secret"}, {Key: "Cookie", Value: "cookie-secret"}, {Key: "X-Normal", Value: "{{plain}}"}}
			r.Body = model.Body{Mode: model.BodyRaw, Text: "body-secret {{plain}}", FilePath: "file-path-secret"}
			col := model.Collection{Name: "body-secret", Variables: []model.KV{{Key: "route", Value: "route-secret"}, {Key: "plain", Value: "ordinary-variable-secret", Description: "description-secret"}, {Key: "off", Value: "disabled-variable-secret", Disabled: true}}}
			env := model.Environment{Name: "ordinary-variable-secret"}
			before, _ := json.Marshal([]any{r, col, env})
			s := preview.Build(r, col, env)
			encoded, _ := json.Marshal(s)
			for _, secret := range []string{cfg.Username, cfg.Password, cfg.Token, cfg.Key, cfg.Value, "url-user-secret", "url-password-secret", "route-secret", "literal-query-secret", "fragment-secret", "literal-authorization-secret", "proxy-secret", "cookie-secret", "ordinary-variable-secret", "body-secret", "file-path-secret", "description-secret", "disabled-variable-secret"} {
				if secret != "" && strings.Contains(string(encoded), secret) {
					t.Errorf("snapshot leaked %q: %s", secret, encoded)
				}
			}
			after, _ := json.Marshal([]any{r, col, env})
			if string(before) != string(after) {
				t.Fatal("preview mutated input")
			}
			if !strings.Contains(s.URL, "https://[masked]@example.test/[masked]") {
				t.Fatalf("unexpected masked route: %s", s.URL)
			}
		})
	}
}

func TestPreviewAuthInheritanceAndExplicitHeaderPrecedence(t *testing.T) {
	r := model.NewRequest("request")
	r.Auth.Type = model.AuthInherit
	r.URL = "https://example.test"
	col := model.Collection{ID: "collection", Auth: model.Auth{Type: model.AuthBearer, Token: "root-secret"}, Items: []model.Item{{ID: "folder", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthBasic, Username: "user-secret", Password: "password-secret"}, Items: []model.Item{{ID: r.ID, Type: model.ItemRequest, Request: &r}}}}}
	s := preview.Build(r, col, model.Environment{})
	if s.AuthSource != auth.SourceFolder || s.AuthType != model.AuthBasic || !slices.Equal(s.HeaderNames, []string{"Authorization"}) {
		t.Fatalf("folder inheritance: %+v", s)
	}
	r.Headers = []model.KV{{Key: "authorization", Value: ""}}
	s = preview.Build(r, col, model.Environment{})
	if !s.AuthOverridden || len(s.HeaderNames) != 1 {
		t.Fatalf("empty explicit header must win: %+v", s)
	}
	col.Items[0].Auth.Type = model.AuthNone
	s = preview.Build(r, col, model.Environment{})
	if s.AuthType != model.AuthNone || s.AuthSource != auth.SourceFolder {
		t.Fatalf("None barrier: %+v", s)
	}
	col.Items[0].Auth.Type = model.AuthInherit
	s = preview.Build(r, col, model.Environment{})
	if s.AuthType != model.AuthBearer || s.AuthSource != auth.SourceCollection {
		t.Fatalf("collection inheritance: %+v", s)
	}
	if !preview.Build(r, model.Collection{}, model.Environment{}).AuthMissing {
		t.Fatal("missing context accepted")
	}
	r.Auth.Type = model.AuthNone
	if preview.Build(r, model.Collection{}, model.Environment{}).AuthMissing {
		t.Fatal("explicit None must not require saved membership")
	}
}

func TestPreviewQueryMultiplicityMatchesHTTPService(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		rows        []model.KV
		cfg         model.Auth
	}{
		{"mirrored", "a=one&a=two", []model.KV{{Key: "a", Value: "one"}, {Key: "a", Value: "two"}}, model.Auth{}},
		{"extra duplicates", "a=one&a=one", []model.KV{{Key: "a", Value: "one"}, {Key: "a", Value: "one"}, {Key: "a", Value: "three"}}, model.Auth{}},
		{"disabled and params only", "a=one", []model.KV{{Key: "a", Value: "one", Disabled: true}, {Key: "added", Value: "two"}}, model.Auth{}},
		{"after expansion", "a={{value}}&space=a%20b", []model.KV{{Key: "a", Value: "{{value}}"}, {Key: "space", Value: "a b"}}, model.Auth{}},
		{"query auth replaces", "key=one&key=two", []model.KV{{Key: "key", Value: "three"}}, model.Auth{Type: model.AuthAPIKey, In: model.APIKeyInQuery, Key: "key", Value: "api-secret"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wire url.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { wire = r.URL.Query() }))
			defer server.Close()
			r := model.NewRequest("request")
			r.URL, r.Params, r.Auth = server.URL+"?"+tc.query, tc.rows, tc.cfg
			col := model.Collection{Variables: []model.KV{{Key: "value", Value: "expanded-secret"}}}
			s := preview.Build(r, col, model.Environment{})
			if _, err := request.New().Send(context.Background(), r, col.Variables); err != nil {
				t.Fatal(err)
			}
			_, raw, _ := strings.Cut(s.URL, "?")
			got, err := url.ParseQuery(raw)
			if err != nil {
				t.Fatal(err)
			}
			if tc.cfg.Type == model.AuthAPIKey {
				if len(got[preview.Mask]) != 1 || len(wire["key"]) != 1 {
					t.Fatalf("query auth must Set once: %v / %v", got, wire)
				}
			} else {
				for key, vals := range wire {
					if len(got[key]) != len(vals) {
						t.Fatalf("%s multiplicity: preview %v, wire %v", key, got, wire)
					}
				}
			}
			for _, vals := range got {
				for _, value := range vals {
					if value != preview.Mask {
						t.Fatal("query value was revealed")
					}
				}
			}
		})
	}
}

func TestPreviewUnknownNamesAcrossActiveInputs(t *testing.T) {
	r := model.NewRequest("request")
	r.URL = "https://example.test/{{url}}"
	r.Params = []model.KV{{Key: "{{param}}", Value: "{{ url }}"}, {Key: "off", Value: "{{disabled}}", Disabled: true}}
	r.Headers = []model.KV{{Key: "{{header}}", Value: "{{value}}"}}
	r.Body = model.Body{Mode: model.BodyFormData, Form: []model.FormField{{Key: "{{field}}", Value: "{{file}}", File: true}}, Text: "{{inactive}}"}
	r.Auth = model.Auth{Type: model.AuthBasic, Username: "{{user}}", Password: "{{password}}"}
	r.PreScript = "{{script}}"
	s := preview.Build(r, model.Collection{}, model.Environment{})
	if !reflect.DeepEqual(s.Unresolved, []string{"url", "param", "header", "value", "field", "file", "user", "password"}) {
		t.Fatalf("unresolved names = %v", s.Unresolved)
	}
}

func TestPreviewInjectedQueryAndInvalidErrorsDoNotRevealSecrets(t *testing.T) {
	r := model.NewRequest("request")
	r.URL = "https://example.test?normal={{injection}}"
	col := model.Collection{Variables: []model.KV{{Key: "injection", Value: "ok&injected-secret=secret-value"}}}
	s := preview.Build(r, col, model.Environment{})
	encoded, _ := json.Marshal(s)
	if strings.Contains(string(encoded), "injected-secret") || strings.Contains(string(encoded), "secret-value") {
		t.Fatalf("variable injection leaked: %s", encoded)
	}
	r.URL = "https://user-secret:password-secret@example.test?key=%zz-secret"
	s = preview.Build(r, col, model.Environment{})
	if !s.InvalidURL || s.URL != "" {
		t.Fatalf("unsafe URL parser error display: %+v", s)
	}
	for _, raw := range []string{"https://user:bad/credential@host.test", "https://user:bad?credential@host.test", "ws://user:credential@host.test"} {
		r.URL = raw
		s = preview.Build(r, col, model.Environment{})
		if !s.InvalidURL || s.URL != "" {
			t.Fatalf("invalid HTTP route must be hidden without parser details: %+v", s)
		}
	}
}

func TestPreviewAllBodyModesHidePayloadAndFileReferences(t *testing.T) {
	for _, mode := range model.BodyModes {
		t.Run(string(mode), func(t *testing.T) {
			r := model.NewRequest("request")
			r.URL = "https://example.test"
			r.Body = model.Body{
				Mode: mode, Text: "payload-secret", FilePath: "private-file-path",
				ContentType: "private-content-type",
				URLEncoded:  []model.KV{{Key: "password", Value: "encoded-secret"}},
				Form:        []model.FormField{{Key: "password", Value: "form-secret"}, {Key: "file", Value: "private-form-file", File: true}},
			}
			encoded, err := json.Marshal(preview.Build(r, model.Collection{}, model.Environment{}))
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"payload-secret", "private-file-path", "private-content-type", "encoded-secret", "form-secret", "private-form-file"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("%s leaked %q", mode, secret)
				}
			}
		})
	}
}
