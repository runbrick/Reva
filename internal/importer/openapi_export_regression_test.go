package importer

import (
	"context"
	"encoding/json"
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

// Synthetic fixture reproduces exporter shapes without retaining customer data.
func TestOpenAPIVendorExportTemplates(t *testing.T) {
	input := string(fixture(t, "openapi-vendor-compat.json"))
	for _, options := range []Options{{}, {AddHost: true, AddBasePath: true}} {
		r, err := ParseWithOptions(input, options)
		if err != nil {
			t.Fatal(err)
		}
		requests := apiRequests(r.Collection.Items)
		if len(requests) != 9 || len(r.Collection.Items) != 1 {
			t.Fatalf("request/folder count=%d/%d", len(requests), len(r.Collection.Items))
		}
		byURL := map[string]*model.Request{}
		for _, r := range requests {
			byURL[r.URL] = r
			if strings.Contains(r.URL, "other.example.test") {
				t.Fatal("empty server silently switched environment")
			}
		}
		if r := byURL["/empty"]; r == nil || r.Body.Mode != model.BodyNone {
			t.Fatalf("empty exported request body became data: %#v", r)
		}
		var decoded map[string]any
		if r := byURL["/json"]; r == nil || json.Unmarshal([]byte(r.Body.Text), &decoded) != nil || decoded["title"] != "示例" || decoded["enabled"] != true {
			t.Fatalf("JSON text example was encoded twice: %#v", r)
		}
		if r := byURL["/scalar"]; r == nil || r.Body.Text != `"true"` {
			t.Fatalf("true JSON string example was coerced: %#v", r)
		}
		want := []model.KV{{Key: "directory", Disabled: true}, {Key: "workspace", Disabled: true}}
		if r := byURL["/query"]; r == nil || !reflect.DeepEqual(r.Params, want) {
			t.Fatalf("optional object query changed: %#v", r)
		}
		if r := byURL["/schema"]; r == nil || r.Body.Mode != model.BodyJSON || r.Body.Text != "null" {
			t.Fatalf("incomplete schema lost the request: %#v", r)
		}
		wantForm := []model.FormField{{Key: "file", Value: "C:/not-opened/example.csv", File: true, Disabled: true}, {Key: "label", Value: "demo"}}
		if r := byURL["/upload"]; r == nil || r.Body.Mode != model.BodyFormData || !reflect.DeepEqual(r.Body.Form, wantForm) {
			t.Fatalf("file reference lost or enabled: %#v", r)
		}
		for _, address := range []string{"https://fixed.example.test/items", "{{baseUrl}}/items", "localhost:14001/items"} {
			if byURL[address] == nil {
				t.Fatalf("explicit address changed: %s", address)
			}
		}
		codes := map[string]int{}
		for _, w := range r.Warnings {
			codes[w.Code]++
			if strings.Contains(w.Path, "MissingExportDefinition") {
				t.Fatal("source key in diagnostic")
			}
		}
		for _, code := range []string{"openapi_empty_server", "openapi_selection", "openapi_schema_choice", "openapi_schema_incomplete", "local_file_references", "openapi_nonstandard_path"} {
			if codes[code] == 0 {
				t.Fatalf("missing warning %s", code)
			}
		}
		if codes["openapi_empty_server"] != 1 || codes["openapi_selection"] != 1 {
			t.Fatal("root server notices repeated for every operation")
		}
	}
}

func TestOpenAPIEmptyServersKeepSelectionAndReportActualLocation(t *testing.T) {
	for _, tc := range []struct{ name, root, entry, op, path string }{
		{"root", `"servers":[{"url":""},{"url":"https://other.example.test"}],`, "", "", "$.servers[0].url"},
		{"path", "", `"servers":[{"url":""}],`, "", "$.paths[0].servers[0].url"},
		{"operation", "", "", `"servers":[{"url":""}],`, "$.paths[0].get.servers[0].url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := `{"openapi":"3.0.3","info":{"title":"Server"},` + tc.root + `"paths":{"/p":{` + tc.entry + `"get":{` + tc.op + `"responses":{}}}}}`
			for _, doc := range []string{"", "http://docs.example.test/spec/openapi.json"} {
				r, err := ParseWithOptions(input, Options{AddHost: true, AddBasePath: true, DocumentURL: doc})
				if err != nil {
					t.Fatal(err)
				}
				want := "/p"
				if doc != "" {
					want = "http://docs.example.test/p"
				}
				if got := apiRequests(r.Collection.Items)[0].URL; got != want {
					t.Fatalf("URL=%s want %s", got, want)
				}
				found := false
				for _, w := range r.Warnings {
					if w.Code == "openapi_empty_server" && w.Path == tc.path {
						found = true
					}
				}
				if !found {
					t.Fatalf("warning points outside actual server scope: %#v", r.Warnings)
				}
			}
		})
	}
	parseFails(t, `{"openapi":"3.0.3","info":{"title":"Server"},"servers":[{}],"paths":{"/p":{"get":{}}}}`, "invalid_structure")
}

func TestOpenAPIExportJSONExamplesAndObjectQueryOnWire(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/json":
			data, err := io.ReadAll(r.Body)
			var value map[string]any
			if err != nil || json.Unmarshal(data, &value) != nil || value["title"] != "示例" || r.Header.Get("Content-Type") != "application/json" {
				t.Error("JSON example did not reach server as an object")
			}
		case "/query":
			if r.URL.Query().Get("directory") != "hello world" || r.URL.Query().Get("workspace") != "demo" || r.URL.Query().Has("scope") {
				t.Error("query object did not become wire fields")
			}
		case "/empty":
			data, err := io.ReadAll(r.Body)
			if err != nil || len(data) != 0 {
				t.Error("empty placeholder sent a JSON string body")
			}
		default:
			t.Error("unexpected request")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	input := string(fixture(t, "openapi-vendor-compat.json"))
	input = strings.Replace(input, `"url": ""`, `"url": `+fmt.Sprintf("%q", server.URL), 1)
	r := parseOK(t, input)
	if calls != 0 {
		t.Fatal("import executed requests")
	}
	sender := requestservice.New()
	for _, item := range apiRequests(r.Collection.Items) {
		if !strings.HasSuffix(item.URL, "/json") && !strings.HasSuffix(item.URL, "/query") && !strings.HasSuffix(item.URL, "/empty") {
			continue
		}
		request := item.Clone()
		if strings.HasSuffix(item.URL, "/query") {
			request.Params = []model.KV{{Key: "directory", Value: "hello world"}, {Key: "workspace", Value: "demo"}}
		}
		response, err := sender.Send(context.Background(), request, nil)
		if err != nil || response.StatusCode != http.StatusNoContent {
			t.Fatalf("send=%v status=%d", err, response.StatusCode)
		}
	}
	if calls != 3 {
		t.Fatalf("sent %d requests", calls)
	}
}

func TestOpenAPIExportObjectQuerySerialization(t *testing.T) {
	for _, example := range []string{`{"a":"hello world","b":false}`, `"{\"a\":\"hello world\",\"b\":false}"`} {
		input := `{"openapi":"3.0.3","info":{"title":"Query"},"paths":{"/q":{"get":{"parameters":[{"name":"filter","in":"query","schema":{"type":"object"},"example":` + example + `}]}}}}`
		r := parseOK(t, input)
		if got := apiRequests(r.Collection.Items)[0].Params; !reflect.DeepEqual(got, []model.KV{{Key: "a", Value: "hello world"}, {Key: "b", Value: "false"}}) {
			t.Fatal(got)
		}
	}
	for _, settings := range []string{`"style":"deepObject",`, `"explode":false,`, `"allowReserved":true,`} {
		parseFails(t, `{"openapi":"3.0.3","info":{"title":"Query"},"paths":{"/q":{"get":{"parameters":[{"name":"filter","in":"query",`+settings+`"schema":{"type":"object"}}]}}}}`, "unsupported_parameter")
	}
}

func TestOpenAPIExportSchemaChoicesAndReferenceBoundaries(t *testing.T) {
	for _, kind := range []string{"anyOf", "oneOf"} {
		input := `{"openapi":"3.0.3","info":{"title":"Choice"},"paths":{"/p":{"post":{"requestBody":{"content":{"application/json":{"schema":{"` + kind + `":[{"type":"object","properties":{"label":{"type":"string","example":"first"}}},{"type":"boolean"}]}}}}}}}}`
		r := parseOK(t, input)
		var body map[string]any
		if err := json.Unmarshal([]byte(apiRequests(r.Collection.Items)[0].Body.Text), &body); err != nil || body["label"] != "first" {
			t.Fatal("choice example changed")
		}
		found := false
		for _, w := range r.Warnings {
			if w.Code == "openapi_schema_choice" {
				found = true
			}
		}
		if !found {
			t.Fatal("schema branch selection was silent")
		}
	}
	for _, ref := range []string{"https://example.test/external.json", "#/components/schemas/Cycle"} {
		input := `{"openapi":"3.0.3","info":{"title":"Boundary"},"components":{"schemas":{"Cycle":{"$ref":"#/components/schemas/Cycle"}}},"paths":{"/p":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"` + ref + `"}}}}}}}}`
		parseFails(t, input, "unsupported_reference")
	}
	// A missing parameter definition cannot be reconstructed as a body template.
	parseFails(t, `{"openapi":"3.0.3","info":{"title":"Parameter"},"paths":{"/p":{"get":{"parameters":[{"$ref":"#/components/parameters/Missing"}]}}}}`, "unsupported_reference")
}
