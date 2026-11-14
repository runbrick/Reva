package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	requestservice "github.com/runbrick/reva/internal/service/request"
)

const swaggerFixture = `{
 "swagger":"2.0","info":{"title":"宠物 API","version":"1"},
 "host":"api.example.test:8443","basePath":"/v1/","schemes":["https"],
 "securityDefinitions":{"key":{"type":"apiKey","name":"X-Key","in":"header"}},"security":[{"key":[]}],
 "parameters":{"id":{"name":"id","in":"path","required":true,"type":"integer","default":42}},
 "definitions":{"Pet":{"type":"object","properties":{"id":{"type":"integer","readOnly":true},"name":{"type":"string","example":"小猫"}}}},
 "paths":{
  "/pets/{id}":{"parameters":[{"$ref":"#/parameters/id"},{"name":"q","in":"query","type":"string","default":"old"}],
   "get":{"summary":"获取宠物","tags":["宠物"],"parameters":[{"name":"q","in":"query","type":"string","default":"new","description":"搜索"},{"name":"X-Test","in":"header","type":"boolean","default":true}]},
   "put":{"security":[],"parameters":[{"name":"body","in":"body","schema":{"$ref":"#/definitions/Pet"}}]}},
  "/form":{"post":{"consumes":["application/x-www-form-urlencoded"],"parameters":[{"name":"tag","in":"formData","type":"string","required":true,"default":"你好"}]}}
 }} `

const openapiFixture = `{
 "openapi":"3.1.0","info":{"title":"OpenAPI 示例","version":"1"},
 "servers":[{"url":"https://{region}.example.test/api","variables":{"region":{"default":"cn"}}},{"url":"https://other.example.test"}],
 "components":{
  "securitySchemes":{"token":{"type":"http","scheme":"bearer"}},
  "schemas":{"Pet":{"type":"object","properties":{"name":{"type":"string","default":"小猫"},"active":{"type":"boolean","default":true}}}},
  "parameters":{"tags":{"name":"tag","in":"query","schema":{"type":"array","items":{"type":"string"}},"example":["one","two"]}},
  "requestBodies":{"Pet":{"content":{"application/problem+json":{"schema":{"$ref":"#/components/schemas/Pet"}}}}}
 },"security":[{"token":[]}],
 "paths":{
  "/pets":{"get":{"parameters":[{"$ref":"#/components/parameters/tags"}]},"post":{"requestBody":{"$ref":"#/components/requestBodies/Pet"}}},
  "/local":{"servers":[{"url":"https://local.example.test/v2"}],"get":{"servers":[{"url":"/override"}],"security":[]}}
 }} `

func apiRequests(items []model.Item) []*model.Request {
	var requests []*model.Request
	for _, item := range items {
		if item.Request != nil {
			requests = append(requests, item.Request)
		}
		requests = append(requests, apiRequests(item.Items)...)
	}
	return requests
}

func TestSwaggerCollectionTemplatesAndPathOptions(t *testing.T) {
	for _, host := range []bool{false, true} {
		for _, base := range []bool{false, true} {
			t.Run(fmt.Sprintf("host=%t/base=%t", host, base), func(t *testing.T) {
				r, err := ParseWithOptions(swaggerFixture, Options{AddHost: host, AddBasePath: base})
				if err != nil {
					t.Fatal(err)
				}
				if Detect(swaggerFixture) != KindSwagger || r.Kind != KindSwagger || r.Collection.Name != "宠物 API" {
					t.Fatal("not recognized as Swagger")
				}
				requests := apiRequests(r.Collection.Items)
				if len(requests) != 3 {
					t.Fatalf("requests=%#v", requests)
				}
				prefix := ""
				if host {
					prefix = "https://api.example.test:8443"
				}
				if base {
					prefix += "/v1"
				}
				get := requests[1]
				if get.Name != "获取宠物" || get.URL != prefix+"/pets/{{id}}" || get.Method != model.MethodGet || get.Auth.Type != model.AuthAPIKey || get.Auth.Key != "X-Key" {
					t.Fatalf("request=%#v", get)
				}
				if !reflect.DeepEqual(get.Params, []model.KV{{Key: "q", Value: "new", Description: "搜索"}}) || len(get.Headers) != 1 || get.Headers[0].Value != "true" {
					t.Fatalf("rows=%#v %#v", get.Params, get.Headers)
				}
				if len(r.Collection.Variables) != 1 || r.Collection.Variables[0].Key != "id" || r.Collection.Variables[0].Value != "42" {
					t.Fatal(r.Collection.Variables)
				}
				put := requests[2]
				var body map[string]any
				if json.Unmarshal([]byte(put.Body.Text), &body) != nil || len(body) != 1 || body["name"] != "小猫" || put.Auth.Type != model.AuthNone {
					t.Fatalf("put=%#v", put)
				}
				if requests[0].Body.Mode != model.BodyURLEncoded || requests[0].Body.URLEncoded[0].Value != "你好" {
					t.Fatal(requests[0])
				}
				if len(r.Warnings) == 0 || r.Warnings[0].Code != "openapi_review" {
					t.Fatal(r.Warnings)
				}
				other := parseOK(t, swaggerFixture)
				if other.Collection.ID == r.Collection.ID || apiRequests(other.Collection.Items)[0].ID == requests[0].ID {
					t.Fatal("reused imported identities")
				}
			})
		}
	}
}

func TestOpenAPIReferencesServersBodiesAndAuth(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.0"} {
		t.Run(version, func(t *testing.T) {
			input := strings.Replace(openapiFixture, "3.1.0", version, 1)
			r, err := ParseWithOptions(input, Options{AddHost: true, AddBasePath: true, DocumentURL: "https://docs.example.test/spec/openapi.json"})
			if err != nil {
				t.Fatal(err)
			}
			requests := apiRequests(r.Collection.Items)
			if Detect(input) != KindOpenAPI || len(requests) != 3 {
				t.Fatal(r)
			}
			if requests[0].URL != "https://docs.example.test/override/local" || requests[0].Auth.Type != model.AuthNone {
				t.Fatal(requests[0])
			}
			get, post := requests[1], requests[2]
			if get.URL != "https://cn.example.test/api/pets" || get.Auth.Type != model.AuthBearer || get.Auth.Token != "{{token}}" {
				t.Fatal(get)
			}
			if !reflect.DeepEqual(get.Params, []model.KV{{Key: "tag", Value: "one"}, {Key: "tag", Value: "two"}}) {
				t.Fatal(get.Params)
			}
			if post.Body.Mode != model.BodyJSON || len(post.Headers) != 1 || post.Headers[0].Value != "application/problem+json" {
				t.Fatal(post)
			}
			var body map[string]any
			if json.Unmarshal([]byte(post.Body.Text), &body) != nil || body["name"] != "小猫" || body["active"] != true {
				t.Fatal(post.Body)
			}
			found := false
			for _, w := range r.Warnings {
				if w.Code == "openapi_selection" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing server-selection notice")
			}
		})
	}
}

func TestOpenAPIBlocksUnsupportedAndMalformedSemantics(t *testing.T) {
	for _, tc := range []struct{ name, fragment, code string }{
		{"external ref", `"get":{"parameters":[{"$ref":"https://SECRET.example.test/p.json"}]}`, "unsupported_reference"},
		{"missing ref", `"get":{"parameters":[{"$ref":"#/components/missing/SECRET"}]}`, "unsupported_reference"},
		{"oauth", `"get":{"security":[{"oauth":[]}]}`, "unsupported_auth"},
		{"combined security", `"get":{"security":[{"oauth":[],"other":[]}]}`, "unsupported_auth"},
		{"cookie", `"get":{"parameters":[{"name":"x","in":"cookie","schema":{"type":"string"}}]}`, "unsupported_parameter"},
		{"deep object", `"get":{"parameters":[{"name":"x","in":"query","style":"deepObject","schema":{"type":"object"}}]}`, "unsupported_parameter"},
		{"binary", `"post":{"requestBody":{"content":{"application/octet-stream":{"schema":{"type":"string","format":"binary"}}}}}`, "unsupported_body"},
		{"composition", `"post":{"requestBody":{"content":{"application/json":{"schema":{"oneOf":[{"type":"string"}]}}}}}`, "unsupported_body"},
		{"encoding", `"post":{"requestBody":{"content":{"multipart/form-data":{"schema":{"type":"object"},"encoding":{"x":{"contentType":"image/png"}}}}}}`, "unsupported_body"},
		{"null operation", `"get":null`, "invalid_structure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := `{"openapi":"3.0.3","info":{"title":"Test"},"components":{"securitySchemes":{"oauth":{"type":"oauth2"}}},"paths":{"/test":{` + tc.fragment + `}}}`
			parseFails(t, input, tc.code)
			_, err := Parse(input)
			if strings.Contains(err.Error(), "SECRET") {
				t.Fatal("diagnostic exposed source content")
			}
		})
	}
	for _, input := range []string{`{"swagger":"3.0"}`, `{"openapi":"3.2.0"}`, `{"openapi":"3.0.bad"}`} {
		parseFails(t, input, "unsupported_version")
	}
	for _, input := range []string{`{"swagger":"2.0","openapi":"3.0.0"}`, `{"openapi":"3.0.0","item":[]}`, `{"swagger":"2.0","version":1}`} {
		parseFails(t, input, "ambiguous_format")
	}
	cycle := `{"openapi":"3.0.3","info":{"title":"Cycle"},"components":{"schemas":{"Node":{"type":"object","properties":{"next":{"$ref":"#/components/schemas/Node"}}}}},"paths":{"/test":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Node"}}}}}}}}`
	parseFails(t, cycle, "unsupported_reference")
}

func TestOpenAPIGeneratedExpansionIsBounded(t *testing.T) {
	paths := make([]string, 9)
	for i := range paths {
		paths[i] = fmt.Sprintf(`"/p%d":{"post":{"requestBody":{"$ref":"#/components/requestBodies/Large"}}}`, i)
	}
	input := `{"openapi":"3.0.3","info":{"title":"Bounded"},"components":{"requestBodies":{"Large":{"content":{"text/plain":{"example":"` + strings.Repeat("x", 4<<20) + `"}}}}},"paths":{` + strings.Join(paths, ",") + `}}`
	parseFails(t, input, "limit_exceeded")
}

func TestImportedSwaggerRequestsSendParametersAuthAndBody(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.Method {
		case http.MethodGet:
			if r.URL.Path != "/v1/pets/42" || r.URL.Query().Get("q") != "new" || r.Header.Get("X-Key") != "test-key" || r.Header.Get("X-Test") != "true" {
				t.Errorf("GET changed on wire: %s %#v", r.URL, r.Header)
			}
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			var value map[string]any
			if err != nil || json.Unmarshal(body, &value) != nil || value["name"] != "小猫" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-Key") != "" || r.URL.Path != "/v1/pets/42" {
				t.Errorf("PUT changed on wire: %s %#v %s", r.URL, r.Header, body)
			}
		default:
			t.Errorf("unexpected imported method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	input := strings.ReplaceAll(swaggerFixture, "api.example.test:8443", strings.TrimPrefix(server.URL, "http://"))
	input = strings.Replace(input, `"schemes":["https"]`, `"schemes":["http"]`, 1)
	parsed := parseOK(t, input)
	variables := append(parsed.Collection.Variables, model.KV{Key: "apiKey", Value: "test-key"})
	if calls != 0 {
		t.Fatal("parse sent requests")
	}
	sender := requestservice.New()
	for _, r := range apiRequests(parsed.Collection.Items)[1:] {
		response, err := sender.Send(context.Background(), *r, variables)
		if err != nil || response.StatusCode != http.StatusNoContent {
			t.Fatalf("send=%#v %v", response, err)
		}
	}
	if calls != 2 {
		t.Fatal("unexpected send count")
	}
}

func TestFetchSwaggerDocumentRedirectErrorsAndCancellation(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/spec/openapi.json", http.StatusFound)
		case "/spec/openapi.json":
			if r.Header.Get("Accept") != "application/json" {
				t.Error("missing JSON accept header")
			}
			fmt.Fprint(w, openapiFixture)
		case "/large":
			fmt.Fprint(w, strings.Repeat(" ", MaxInputBytes+1))
		case "/slow":
			<-r.Context().Done()
		default:
			http.Error(w, "SECRET", http.StatusUnauthorized)
		}
	}))
	t.Cleanup(server.Close)
	input, address, err := FetchDocument(context.Background(), server.URL+"/redirect")
	if err != nil || input != openapiFixture || address != server.URL+"/spec/openapi.json" {
		t.Fatalf("fetch=%s %v", address, err)
	}
	for _, tc := range []struct{ url, code string }{
		{server.URL + "/bad", "fetch_failed"}, {server.URL + "/large", "input_too_large"},
		{"file:///tmp/spec.json", "invalid_source_url"}, {"ftp://host/spec.json", "invalid_source_url"}, {"https://user:SECRET@host/spec.json", "invalid_source_url"}, {"https://host/spec.json#fragment", "invalid_source_url"},
	} {
		_, _, err := FetchDocument(context.Background(), tc.url)
		var failure *ParseError
		if !errors.As(err, &failure) || failure.Code != tc.code || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("error=%v, want %s", err, tc.code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := FetchDocument(ctx, server.URL+"/slow"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
