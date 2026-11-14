package importer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	requestservice "github.com/runbrick/reva/internal/service/request"
)

func TestImportRegressionNativeEveryCurrentFieldAndCollidingIDs(t *testing.T) {
	// These are export-shaped models. Inactive credentials/body fields are real
	// editor state and must survive even when another scheme/mode is selected.
	allAuth := model.Auth{Type: model.AuthBasic, Token: "inactive-token", Username: "{{user}}", Password: "{{password}}", Key: "inactive-key", Value: "inactive-value", In: model.APIKeyInQuery}
	rows := []model.KV{{Key: "重复", Value: "{{literal}}", Description: "说明"}, {Key: "重复", Value: "", Disabled: true, Description: "disabled"}}
	source := model.Collection{Version: 1, ID: "../../collision", Name: "原生集合", Auth: allAuth, Variables: rows}
	for i, mode := range []model.BodyMode{"", model.BodyNone, model.BodyRaw, model.BodyJSON, model.BodyURLEncoded, model.BodyFormData, model.BodyBinary} {
		r := model.NewRequest(source.ID)
		r.Name, r.Method, r.URL = "同名请求", model.MethodPatch, "{{base}}/路径?literal={{v}}#锚点"
		r.Params, r.Headers, r.Auth = rows, rows, allAuth
		r.Body = model.Body{Mode: mode, Text: "原文\n{{v}}", ContentType: "application/custom; charset=utf-8", Form: []model.FormField{{Key: "file", Value: "/never-read/import-regression", File: true, Disabled: true}, {Key: "text", Value: "{{v}}"}}, URLEncoded: rows, FilePath: "/never-read/inactive-binary"}
		r.PreScript, r.TestScript = "throw 'must not run during import'", "while (true) {}"
		r.Options = model.RequestOptions{TimeoutMs: 9223372036854, FollowRedirects: model.ToggleOn, VerifyTLS: model.ToggleOff}
		r.Auth.Type = []model.AuthType{"", model.AuthNone, model.AuthInherit, model.AuthBearer, model.AuthBasic, model.AuthAPIKey, model.AuthNone}[i]
		source.Items = append(source.Items, model.Item{Type: model.ItemRequest, ID: source.ID, Name: r.Name, Request: &r})
	}
	source.Items = []model.Item{{Type: model.ItemFolder, ID: source.ID, Name: "认证文件夹", Auth: allAuth, Items: source.Items}, {Type: model.ItemFolder, ID: source.ID, Name: "空文件夹", Auth: model.Auth{Type: model.AuthNone}}}
	input := encode(t, source)
	first, second := parseOK(t, input), parseOK(t, input)
	seen := map[string]bool{source.ID: true}
	var restore func(*model.Collection)
	restore = func(col *model.Collection) {
		var walk func([]model.Item, []model.Item)
		walk = func(got, original []model.Item) {
			for i := range got {
				if got[i].ID == "" || seen[got[i].ID] {
					t.Fatal("source/colliding IDs were reused across imports")
				}
				seen[got[i].ID] = true
				if got[i].Request != nil {
					if got[i].Request.ID != got[i].ID {
						t.Fatal("request identity differs from its item")
					}
					got[i].Request.ID = original[i].Request.ID
				}
				got[i].ID = original[i].ID
				walk(got[i].Items, original[i].Items)
			}
		}
		if col.ID == "" || seen[col.ID] {
			t.Fatal("collection identity was reused")
		}
		seen[col.ID] = true
		col.ID = source.ID
		walk(col.Items, source.Items)
		if !reflect.DeepEqual(*col, source) {
			t.Fatal("native import changed a current model field")
		}
	}
	restore(first.Collection)
	restore(second.Collection)
	first.Collection.Items[0].Items[0].Request.Body.Form[0].Value = "edited"
	first.Collection.Variables[0].Value = "edited"
	first.Collection.Items[0].Items[0].Request.Headers[0].Description = "edited"
	if !reflect.DeepEqual(*second.Collection, source) || encode(t, source) != input {
		t.Fatal("independent parsed results or caller model share mutable data")
	}
}

func TestImportRegressionMalformedCandidatesStayBlocked(t *testing.T) {
	for _, tc := range []struct{ name, input, code string }{
		{"escaped duplicate", `{"info":{"name":"C","\u006eame":"secret-regression"},"item":[]}`, "invalid_json"},
		{"trailing null", `{"info":{"name":"C"},"item":[]} null`, "invalid_json"},
		{"trailing string", `{"version":1,"name":"E","variables":[]} "secret-regression"`, "invalid_json"},
		{"array root", `["secret-regression"]`, "invalid_structure"},
		{"ambiguous env HAR", `{"_postman_variable_scope":"environment","values":[],"log":null}`, "ambiguous_format"},
		{"malformed postman info", `{"info":"secret-regression","item":[]}`, "invalid_structure"},
		{"malformed HAR entries", `{"log":{"version":"1.2","entries":[null]}}`, "invalid_structure"},
		{"native scalar null", `{"version":1,"name":"E","variables":[{"key":"secret-regression","value":"v","disabled":null}]}`, "invalid_structure"},
		{"native request null body", `{"version":1,"name":"C","items":[{"type":"request","name":"R","request":{"name":"R","method":"GET","url":"","body":null}}]}`, "invalid_structure"},
		{"native request null auth", `{"version":1,"name":"C","items":[{"type":"request","name":"R","request":{"name":"R","method":"GET","url":"","body":{"mode":"none"},"auth":null}}]}`, "invalid_structure"},
		{"native timeout overflow", `{"version":1,"name":"C","items":[{"type":"request","name":"R","request":{"name":"R","method":"GET","url":"","body":{"mode":"none"},"options":{"timeoutMs":9223372036855}}}]}`, "invalid_structure"},
		{"native future field", `{"version":1,"name":"C","items":[],"secret-regression":true}`, "invalid_structure"},
		{"postman invalid method", `{"info":{"name":"C"},"item":[{"name":"R","request":{"method":"GET\r\nsecret-regression","url":"x"}}]}`, "invalid_structure"},
		{"postman malformed body array", pmRequest(`{"mode":"formdata","formdata":[null]}`, ""), "invalid_structure"},
		{"postman malformed auth array", pmRequest("", `{"type":"bearer","bearer":[null]}`), "invalid_auth"},
		{"env globals", `{"_postman_variable_scope":"globals","name":"G","values":[]}`, "unsupported_format"},
		{"fake vendor native", `{"name":"Apifox ApiPost","project":{"secret-regression":true}}`, "unknown_format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := parseFails(t, tc.input, tc.code)
			if strings.Contains(err.Error(), "secret-regression") {
				t.Fatal("diagnostic contains a source value or arbitrary property name")
			}
		})
	}
	// Nil native slices are emitted by encoding/json; scalar nulls above are
	// malformed. Postman null auth is its explicit inheritance representation.
	parseOK(t, `{"version":1,"name":"E","variables":null}`)
	parseOK(t, `{"version":1,"name":"C","items":null,"variables":null}`)
	if r := parseOK(t, pmRequest("", "null")); r.Collection.Items[0].Request.Auth.Type != model.AuthInherit {
		t.Fatal("Postman null auth lost inheritance")
	}
}

func TestImportRegressionWarningsAreStructuralAndStable(t *testing.T) {
	input := `{"info":{"name":"C","secret-regression":{"secret-regression":true}},"event":[{"secret-regression":"secret-regression"}],"item":[{"name":"R","response":[{"secret-regression":true}],"request":{"url":{"raw":"{{base}}/:id?v={{$guid}}","variable":[{"key":"id","value":"secret-regression"}]},"header":[{"key":"Authorization","value":"{{vault:secret-regression}}"}],"body":{"mode":"formdata","formdata":[{"key":"file","type":"file","src":["/never-read/secret-regression","/never-read/other"],"contentType":"secret-regression","description":"secret-regression"}]}}}],"secret-regression":{"secret-regression":true}}`
	first := parseOK(t, input)
	if len(first.Collection.Items[0].Request.Body.Form) != 2 {
		t.Fatal("multi-file reference rows were lost")
	}
	seen := map[Diagnostic]bool{}
	for _, w := range first.Warnings {
		if seen[w] || strings.Contains(w.Code+w.Path, "secret-regression") {
			t.Fatal("diagnostic is duplicated or contains source data")
		}
		seen[w] = true
	}
	for range 10 {
		if !reflect.DeepEqual(parseOK(t, input).Warnings, first.Warnings) {
			t.Fatal("review diagnostics change order across identical parses")
		}
	}
}

func TestImportRegressionNativeAndHARAggregateBudgets(t *testing.T) {
	// Use the documented numeric limits, independently of parser constants.
	rows := strings.Repeat(`{"key":"x","value":"y"},`, 49999) + `{"key":"x","value":"y"}`
	native := `{"version":1,"name":"C","variables":[` + rows + `],"items":[{"type":"request","name":"R","request":{"name":"R","method":"GET","url":"","body":{"mode":"none","urlencoded":[` + rows + `]}}}]}`
	parseOK(t, native)
	parseFails(t, strings.Replace(native, `"url":""`, `"url":"","headers":[{"key":"extra","value":"row"}]`, 1), "limit_exceeded")
	cookies := strings.Repeat(`{"name":"c","value":"v"},`, 99999) + `{"name":"c","value":"v"}`
	har := `{"log":{"version":"1.2","entries":[{"request":{"method":"GET","url":"https://example.test","headers":[],"cookies":[` + cookies + `]}}]}}`
	parseOK(t, har)
	parseFails(t, strings.Replace(har, `"headers":[]`, `"headers":[{"name":"extra","value":"row"}]`, 1), "limit_exceeded")
	leaf := `{"type":"folder","name":"Empty"}`
	for range 256 {
		leaf = `{"type":"folder","name":"Parent","items":[` + leaf + `]}`
	}
	parseOK(t, `{"version":1,"name":"C","items":[`+leaf+`]}`)
	parseFails(t, `{"version":1,"name":"C","items":[{"type":"folder","name":"Extra","items":[`+leaf+`]}]}`, "limit_exceeded")
	// Deep omitted metadata is still bounded before the converter skips it.
	parseFails(t, `{"info":{"name":"C"},"item":[],"metadata":`+strings.Repeat("[", 1025)+strings.Repeat("]", 1025)+`}`, "limit_exceeded")
}

func TestImportRegressionHAROrderBodiesAndExplicitCredentialsOnWire(t *testing.T) {
	type observed struct {
		path, body, cookie, authorization, contentType string
		form                                           url.Values
		query                                          url.Values
	}
	requests := make(chan observed, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := observed{path: r.URL.Path, cookie: r.Header.Get("Cookie"), authorization: r.Header.Get("Authorization"), contentType: r.Header.Get("Content-Type"), query: r.URL.Query()}
		if strings.HasPrefix(got.contentType, "multipart/form-data") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				http.Error(w, "invalid multipart", http.StatusBadRequest)
				return
			}
			got.form = r.MultipartForm.Value
			defer r.MultipartForm.RemoveAll()
		} else {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "read failed", http.StatusBadRequest)
				return
			}
			got.body = string(data)
		}
		requests <- got
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	// Synthetic HAR 1.2 extraction fixture. Captured auth/cookies are headers;
	// a cookies list alone must remain an acknowledged omission, never auth.
	input := `{"log":{"version":"1.2","pages":[{"id":"not-history"}],"entries":[
		{"request":{"method":"POST","url":"` + server.URL + `/text?q=a&q=b#fragment","headers":[{"name":"Cookie","value":"sid=captured"},{"name":"Authorization","value":"captured explicit"}],"cookies":[{"name":"sid","value":"must-not-override"}],"postData":{"mimeType":"application/problem+json; charset=utf-8","text":"{\"值\":1}"}},"response":{"status":500,"content":{"text":"must-not-be-restored"}}},
		{"request":{"method":"POST","url":"` + server.URL + `/encoded","headers":[],"cookies":[{"name":"sid","value":"omitted-cookie"}],"postData":{"mimeType":"application/x-www-form-urlencoded","params":[{"name":"q","value":"a b"},{"name":"q","value":"+"}],"text":"q=a+b&q=%2B"}}},
		{"request":{"method":"POST","url":"` + server.URL + `/multipart","headers":[],"postData":{"mimeType":"multipart/form-data","params":[{"name":"q","value":"one"},{"name":"q","value":"two"},{"name":"unicode","value":"中文"}]}}}
	]}}`
	parsed := parseOK(t, input)
	if len(parsed.Collection.Items) != 3 || !hasWarning(parsed, "har_response_omitted") || !hasWarning(parsed, "har_cookies_omitted") {
		t.Fatal("HAR request count/omission contract changed")
	}
	for i, item := range parsed.Collection.Items {
		r := item.Request
		if r.Auth.Type != model.AuthNone {
			t.Fatal("HAR inferred generated authentication from captured headers/cookies")
		}
		resp, err := requestservice.New().Send(context.Background(), *r, nil)
		if err != nil || resp.StatusCode != http.StatusNoContent {
			t.Fatal("supported extracted HAR body could not be sent locally")
		}
		got := <-requests
		switch i {
		case 0:
			if got.path != "/text" || got.body != `{"值":1}` || got.cookie != "sid=captured" || got.authorization != "captured explicit" || got.contentType != "application/problem+json; charset=utf-8" || !reflect.DeepEqual(got.query["q"], []string{"a", "b"}) {
				t.Fatal("HAR textual body, query, explicit headers, or order changed on wire")
			}
		case 1:
			if got.path != "/encoded" || got.body != "q=a+b&q=%2B" || got.cookie != "" || got.authorization != "" {
				t.Fatal("HAR URL-encoded rows changed or omitted cookies were generated")
			}
		case 2:
			if got.path != "/multipart" || !reflect.DeepEqual(got.form["q"], []string{"one", "two"}) || got.form.Get("unicode") != "中文" || got.cookie != "" {
				t.Fatal("HAR multipart text/repeated rows or request order changed")
			}
		}
	}
}

func TestImportRegressionJSONNestingAndExpandedFileRowBoundary(t *testing.T) {
	// JSON token depth counts the root as zero. At the boundary even omitted
	// metadata is decoded and validated; the next nested value must reject.
	metadata := func(n int) string {
		return `{"info":{"name":"C"},"item":[],"metadata":` + strings.Repeat("[", n) + "null" + strings.Repeat("]", n) + `}`
	}
	parseOK(t, metadata(1023))
	parseFails(t, metadata(1024), "limit_exceeded")
	// Expanding one Postman file row with several references consumes one row
	// per reference, in addition to collection variables and auth attributes.
	rows := strings.Repeat(`{"key":"x","value":"y"},`, 99995) + `{"key":"x","value":"y"}`
	input := `{"info":{"name":"C"},"auth":{"type":"bearer","bearer":[{"key":"token","value":"literal"}]},"variable":[` + rows + `],"item":[{"name":"R","request":{"url":"https://example.test","body":{"mode":"formdata","formdata":[{"key":"file","type":"file","src":["/never-read/one","/never-read/two","/never-read/three"]}]}}}]}`
	if got := parseOK(t, input).Collection.Items[0].Request.Body.Form; len(got) != 3 || got[2].Value != "/never-read/three" {
		t.Fatal("expanded file references lost order at aggregate row boundary")
	}
	parseFails(t, strings.Replace(input, `"/never-read/three"]`, `"/never-read/three","/never-read/four"]`, 1), "limit_exceeded")
}
