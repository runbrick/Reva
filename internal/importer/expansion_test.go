package importer

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
)

func parseOK(t testing.TB, input string) Result {
	t.Helper()
	r, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	n := 0
	for _, present := range []bool{r.Collection != nil, r.Environment != nil, r.Request != nil} {
		if present {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("success payload count = %d", n)
	}
	return r
}
func parseFails(t testing.TB, input, code string) *ParseError {
	t.Helper()
	r, err := Parse(input)
	var e *ParseError
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("Parse error=%v, want %s", err, code)
	}
	if !reflect.DeepEqual(r, Result{}) {
		t.Fatalf("failure returned usable data: %#v", r)
	}
	if !strings.HasPrefix(e.Path, "$") || strings.Contains(e.Error(), "private-marker") {
		t.Fatalf("unsafe diagnostic: %v", e)
	}
	return e
}
func hasWarning(r Result, code string) bool {
	for _, w := range r.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}
func encode(t testing.TB, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func pmRequest(body, auth string) string {
	if body == "" {
		body = "null"
	}
	if auth == "" {
		auth = "null"
	}
	return `{"info":{"name":"API"},"item":[{"name":"request","request":{"url":"{{baseUrl}}/path","body":` + body + `,"auth":` + auth + `}}]}`
}

func TestImportNativeCollectionRoundTrip(t *testing.T) {
	r := model.NewRequest("source-request")
	r.Name = "请求"
	r.URL = "{{base}}/x"
	r.Auth = model.Auth{Type: model.AuthInherit, Token: "inactive"}
	r.Headers = []model.KV{{Key: "X", Value: "", Description: "note"}, {Key: "X", Value: "{{v}}", Disabled: true}}
	r.Params = []model.KV{{Key: "q", Value: "1"}, {Key: "q", Value: "2"}}
	r.Body = model.Body{Mode: model.BodyRaw, Text: "hello", ContentType: "text/custom", Form: []model.FormField{{Key: "inactive", Value: "text"}}}
	r.Options = model.RequestOptions{TimeoutMs: 5000, FollowRedirects: model.ToggleOff, VerifyTLS: model.ToggleOn}
	r.PreScript = "throw 'never execute';"
	r.TestScript = "while (true) {}"
	col := model.Collection{Version: 1, ID: "../source", Name: "测试", Auth: model.Auth{Type: model.AuthBearer, Token: "{{root}}"}, Variables: []model.KV{{Key: "v", Value: "测试", Description: "变量"}}, Items: []model.Item{{Type: model.ItemFolder, ID: "folder", Name: "parent", Auth: model.Auth{Type: model.AuthNone}, Items: []model.Item{{Type: model.ItemRequest, ID: r.ID, Name: r.Name, Request: &r}}}, {Type: model.ItemFolder, ID: "empty", Name: "empty"}}}
	input := encode(t, col)
	first := parseOK(t, input)
	second := parseOK(t, input)
	if first.Kind != KindNativeCollection || len(first.Warnings) != 0 {
		t.Fatalf("native result=%#v", first)
	}
	got := first.Collection
	if got.ID == col.ID || second.Collection.ID == got.ID || got.Items[0].ID == col.Items[0].ID {
		t.Fatal("import reused source identities")
	}
	if got.Items[0].Items[0].ID != got.Items[0].Items[0].Request.ID {
		t.Fatal("request/item ID equality lost")
	}
	got.ID = col.ID
	for i := range got.Items {
		got.Items[i].ID = col.Items[i].ID
	}
	got.Items[0].Items[0].ID = r.ID
	got.Items[0].Items[0].Request.ID = r.ID
	if !reflect.DeepEqual(*got, col) {
		t.Fatalf("native fields changed:\n%#v\n%#v", got, col)
	}
	got.Items[0].Items[0].Request.Headers[0].Value = "changed"
	if col.Items[0].Items[0].Request.Headers[0].Value != "" || encode(t, col) != input {
		t.Fatal("input mutated")
	}
}

func TestImportNativeEnvironmentRoundTrip(t *testing.T) {
	e := model.Environment{Version: 1, ID: "../../old", Name: "环境", Variables: []model.KV{{Key: "x", Value: "1", Description: "说明"}, {Key: "x", Value: "2", Disabled: true}}}
	r := parseOK(t, " \n\ufeff"+encode(t, e))
	if r.Kind != KindNativeEnvironment || r.Environment.ID == e.ID {
		t.Fatalf("environment result=%#v", r)
	}
	r.Environment.ID = e.ID
	if !reflect.DeepEqual(*r.Environment, e) {
		t.Fatal("environment fields changed")
	}
	legacy := parseOK(t, `{"version":1,"name":"Legacy","items":[{"type":"folder","name":"F","items":[{"type":"request","name":"R","request":{"name":"R","method":"GET","url":"","body":{"mode":"none"}}}]}]}`)
	if legacy.Collection.Items[0].Items[0].Request.Auth != (model.Auth{}) {
		t.Fatal("legacy no-auth acquired inheritance")
	}
}

func TestImportNativeValidationAndFileReferences(t *testing.T) {
	r := model.NewRequest("r")
	r.Name = "R"
	r.URL = "https://example.test"
	r.Auth = model.Auth{Type: model.AuthAPIKey, Key: "{{key}}", Value: "{{value}}", In: model.APIKeyInQuery}
	r.Body = model.Body{Mode: model.BodyFormData, Form: []model.FormField{{Key: "f", Value: "/never-open/private-marker", File: true, Disabled: true}}, URLEncoded: []model.KV{{Key: "inactive", Value: "keep", Description: "keep", Disabled: true}}, FilePath: "inactive.bin"}
	wrap := func() string {
		return encode(t, model.Collection{Version: 1, Name: "C", Items: []model.Item{{Type: model.ItemRequest, Name: r.Name, ID: r.ID, Request: &r}}})
	}
	out := parseOK(t, wrap())
	if !reflect.DeepEqual(out.Collection.Items[0].Request.Body, r.Body) || !hasWarning(out, "local_file_references") {
		t.Fatal("native file metadata lost or file reference undisclosed")
	}
	for _, mode := range []model.BodyMode{model.BodyNone, model.BodyJSON, model.BodyRaw, model.BodyURLEncoded, model.BodyBinary} {
		r.Body.Mode = mode
		parseOK(t, wrap())
	}
	r.Options.FollowRedirects = "future"
	parseFails(t, wrap(), "invalid_structure")
	r.Options.FollowRedirects = ""
	r.Options.TimeoutMs = -1
	parseFails(t, wrap(), "invalid_structure")
	r.Options.TimeoutMs = 0
	r.Auth.In = "cookie"
	parseFails(t, wrap(), "invalid_auth")
	r.Auth.In = model.APIKeyInHeader
	r.Auth.Type = "oauth2"
	parseFails(t, wrap(), "unsupported_auth")
	r.Auth.Type = model.AuthNone
	r.Body.Mode = "future"
	parseFails(t, wrap(), "unsupported_body")
	parseFails(t, `{"version":1,"name":"C","items":[{"type":"request","name":"R","items":[],"request":{"name":"R","method":"GET","url":"","body":{"mode":"none"}}}]}`, "invalid_structure")
}

func TestImportPostmanCompatibleExports(t *testing.T) {
	// Synthetic compatibility shapes, not authentic Apifox/ApiPost exports.
	var fixtures []struct {
		Name  string
		Input json.RawMessage
	}
	if err := json.Unmarshal(fixture(t, "expansion.json"), &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			if !strings.HasPrefix(f.Name, "synthetic ") {
				t.Fatal("compatibility fixture must identify its synthetic provenance")
			}
			r := parseOK(t, string(f.Input))
			if r.Kind != KindPostman {
				t.Fatal(r.Kind)
			}
			col := r.Collection
			if len(col.Items) != 2 || col.Items[1].Type != model.ItemFolder {
				t.Fatal("empty folder/order lost")
			}
			if len(col.Items[0].Items) != 2 || col.Items[0].Items[0].Name != col.Items[0].Items[1].Name || len(col.Items[1].Items) != 0 {
				t.Fatal("duplicate sibling names or empty folders were lost")
			}
			if col.Auth.Type != model.AuthBearer || col.Auth.Token != "{{root}}" || col.Items[0].Auth.Type != model.AuthNone || col.Items[1].Auth.Type != model.AuthInherit {
				t.Fatal("shared auth scopes changed during compatible export conversion")
			}
			if len(col.Variables) != 2 || col.Variables[0].Key != col.Variables[1].Key || col.Variables[0].Description != "Source base" || !col.Variables[1].Disabled {
				t.Fatal("collection variable scope/order/duplicates/descriptions/disabled state changed")
			}
			req := col.Items[0].Items[0].Request
			if req.URL != "{{baseUrl}}/path#frag" || len(req.Params) != 2 || len(req.Headers) != 2 || !req.Headers[1].Disabled || req.Headers[0].Description != "保留" {
				t.Fatalf("rows/templates changed: %#v", req)
			}
			if req.Body.Mode != model.BodyRaw || req.Body.ContentType != "application/xml" {
				t.Fatalf("XML metadata lost: %#v", req.Body)
			}
			effective, _, err := auth.Resolve(*col, req.ID, req.Auth)
			if err != nil || effective.Type != model.AuthNone {
				t.Fatal("None barrier lost")
			}
		})
	}
	for _, v := range []string{"v2.0.0", "v2.1.0"} {
		input := strings.Replace(string(fixture(t, "postman-v21.json")), "v2.1.0", v, 1)
		r := parseOK(t, input)
		if !hasWarning(r, "scripts_omitted") || !hasWarning(r, "local_file_references") {
			t.Fatal("legacy fidelity warnings absent")
		}
	}
	r := parseOK(t, `{"info":{"name":"Query"},"item":[{"name":"R","request":{"url":{"raw":"{{base}}?old=1#f","query":[]}}}]}`)
	if req := r.Collection.Items[0].Request; req.URL != "{{base}}#f" || len(req.Params) != 0 {
		t.Fatal("explicit empty query was not authoritative")
	}
}

func TestImportUnsupportedSemanticsAreVisible(t *testing.T) {
	for _, scheme := range []string{"oauth1", "oauth2", "digest", "awsv4", "ntlm", "future"} {
		t.Run(scheme, func(t *testing.T) {
			parseFails(t, pmRequest("", `{"type":"`+scheme+`","token":"private-marker"}`), "unsupported_auth")
		})
	}
	for _, a := range []string{`{}`, `{"type":""}`, `42`, `{"type":"apikey","apikey":{"key":"x","value":"private-marker","in":"cookie"}}`, `{"type":"bearer","bearer":[{"key":"token","value":"x"},{"key":"token","value":"y"}]}`, `{"type":"bearer","bearer":{"token":false}}`} {
		parseFails(t, pmRequest("", a), "invalid_auth")
	}
	for _, b := range []string{`{"mode":"graphql","graphql":{"query":"private-marker"}}`, `{"mode":"future"}`, `{"mode":"raw","raw":"x","encoding":"base64"}`} {
		parseFails(t, pmRequest(b, ""), "unsupported_body")
	}
	for _, node := range []string{`{"name":"private-marker"}`, `{"name":"R","item":[],"request":"x"}`, `{"name":" ","request":"x"}`, `{"name":"R","item":null}`} {
		parseFails(t, `{"info":{"name":"C"},"item":[`+node+`]}`, "invalid_structure")
	}
	input := `{"info":{"name":"private-marker","description":"private-marker"},"event":[{}],"item":[{"name":"R","event":[{}],"response":[{}],"request":{"description":"private-marker","url":{"raw":"{{base}}/:id?d={{$guid}}","variable":[{"key":"id","value":"private-marker"}]},"header":[{"key":"X","value":"{{vault:private-marker}}"}],"body":{"mode":"formdata","formdata":[{"key":"file","type":"file","src":"/never-open/private-marker","contentType":"image/png","description":"private-marker"}]}}}]}`
	r := parseOK(t, input)
	for _, code := range []string{"scripts_omitted", "examples_omitted", "metadata_omitted", "path_variables_unresolved", "dynamic_variables_unsupported", "local_file_references", "form_metadata_omitted"} {
		if !hasWarning(r, code) {
			t.Errorf("missing warning %s: %#v", code, r.Warnings)
		}
	}
	for _, w := range r.Warnings {
		if strings.Contains(w.Code+w.Path, "private-marker") {
			t.Fatal("warning leaked data")
		}
	}
	r = parseOK(t, pmRequest(`{"mode":"raw","raw":"private-marker","disabled":true}`, ""))
	if r.Collection.Items[0].Request.Body.Mode != model.BodyNone || !hasWarning(r, "metadata_omitted") {
		t.Fatal("disabled body became active")
	}
	r = parseOK(t, `{"info":{"name":"C"},"item":[],"private-marker":{"private-marker":"private-marker"}}`)
	if len(r.Warnings) != 1 || r.Warnings[0] != (Diagnostic{Code: "metadata_omitted", Path: "$"}) {
		t.Fatalf("unknown metadata diagnostics=%#v", r.Warnings)
	}
}

func TestImportEnvironmentScopesAndDisabledRows(t *testing.T) {
	r := parseOK(t, `{"_postman_variable_scope":"environment","name":"环境","values":[{"key":"x","value":"{{vault:secret}}","enabled":false,"type":"secret","description":{"content":"说明","type":"text/plain"}},{"key":"x","value":1.20},{"key":"object","value":{"b":true,"a":[1,2]}},{"key":"bool","value":false}]}`)
	e := r.Environment
	if r.Kind != KindPostmanEnvironment || e.Version != 1 || e.ID == "" || !e.Variables[0].Disabled || e.Variables[1].Disabled || e.Variables[0].Description != "说明" || e.Variables[1].Value != "1.20" || e.Variables[2].Value != `{"a":[1,2],"b":true}` || e.Variables[3].Value != "false" || !hasWarning(r, "dynamic_variables_unsupported") {
		t.Fatalf("environment mapping=%#v", r)
	}
	parseFails(t, `{"_postman_variable_scope":"globals","name":"G","values":[]}`, "unsupported_format")
	parseFails(t, `{"_postman_variable_scope":"environment","name":"E","values":[{"key":"x","enabled":null}]}`, "invalid_structure")
	parseFails(t, `{"name":"not an environment"}`, "unknown_format")
}

func harInput(post string) string {
	suffix := ""
	if post != "" {
		suffix = `,"postData":` + post
	}
	return `{"log":{"version":"1.2","entries":[{"request":{"method":"POST","url":"https://example.test/x?old=1#f","headers":[{"name":"X","value":"1"},{"name":"X","value":"2"}],"queryString":[{"name":"q","value":"a"},{"name":"q","value":"b"}],"cookies":[{"name":"session","value":"private-marker"}]` + suffix + `},"response":{"status":200},"timings":{}}]}}`
}
func TestImportHARRequestsAndOmissions(t *testing.T) {
	for _, tc := range []struct {
		post string
		mode model.BodyMode
	}{{`{"mimeType":"application/json","text":"{\"x\":1}"}`, model.BodyRaw}, {`{"mimeType":"application/x-www-form-urlencoded","params":[{"name":"q","value":"1"},{"name":"q","value":"2"}],"text":"q=1&q=2"}`, model.BodyURLEncoded}, {`{"mimeType":"multipart/form-data","params":[{"name":"q","value":"1","comment":"note"}]}`, model.BodyFormData}} {
		t.Run(string(tc.mode), func(t *testing.T) {
			r := parseOK(t, harInput(tc.post))
			req := r.Collection.Items[0].Request
			if r.Kind != KindHAR || r.Collection.Name != "HAR" || req.Auth.Type != model.AuthNone || req.URL != "https://example.test/x#f" || len(req.Headers) != 2 || len(req.Params) != 2 || req.Params[0].Key != "q" || req.Body.Mode != tc.mode || !hasWarning(r, "har_response_omitted") || !hasWarning(r, "har_cookies_omitted") {
				t.Fatalf("HAR mapping=%#v %#v", r, req)
			}
		})
	}
	for _, post := range []string{`{"mimeType":"multipart/form-data","params":[{"name":"file","fileName":"private-marker"}]}`, `{"mimeType":"application/json","text":"e30=","encoding":"base64"}`, `{"mimeType":"application/octet-stream","text":"private-marker"}`, `{"mimeType":"application/x-www-form-urlencoded","params":[{"name":"x","value":"a"}],"text":"x=b"}`, `{"mimeType":"multipart/form-data","params":[{"name":"x","value":"a"}],"text":"not framing"}`, `{"mimeType":"application/x-www-form-urlencoded","params":[],"text":"x=a"}`} {
		parseFails(t, harInput(post), "unsupported_body")
	}
	input := strings.Replace(harInput(""), `{"name":"X","value":"1"}`, `{"name":"Cookie","value":"session=kept"}`, 1)
	if hasWarning(parseOK(t, input), "har_cookies_omitted") {
		t.Fatal("captured Cookie header was ignored")
	}
	input = `{"log":{"version":"1.2","entries":[{"request":{"method":"GET","url":"https://example.test/?q=a&q=b","headers":[]}}]}}`
	if rows := parseOK(t, input).Collection.Items[0].Request.Params; len(rows) != 2 || rows[1].Value != "b" {
		t.Fatal("HAR URL query fallback lost")
	}
}

func TestImportCurlTransportWarnings(t *testing.T) {
	for _, flags := range []string{"-k", "--insecure", "-L", "--location", "--compressed", "-sSILk"} {
		r := parseOK(t, "curl https://example.test "+flags)
		if !hasWarning(r, "transport_flags_ignored") || r.Request.Options != (model.RequestOptions{}) {
			t.Fatalf("transport flag changed defaults or disappeared: %s", flags)
		}
	}
	for _, cmd := range []string{`curl https://example.test -H 'X: --insecure'`, `curl https://example.test -u user:-k`, `curl https://example.test --data=-L`, `curl -- https://example.test/-k`} {
		if hasWarning(parseOK(t, cmd), "transport_flags_ignored") {
			t.Fatalf("argument value mistaken for flag: %s", cmd)
		}
	}
	r := parseOK(t, `curl https://example.test --data-binary @/never-open/private-marker`)
	if !hasWarning(r, "local_file_references") || r.Request.Body.FilePath != "/never-open/private-marker" {
		t.Fatal("file reference not preserved/disclosed")
	}
	parseFails(t, `curl https://example.test --future private-marker`, "invalid_curl")
}

func TestImportStructuredURLAndAggregateRows(t *testing.T) {
	r := parseOK(t, `{"info":{"name":"C"},"item":[{"name":"R","request":{"url":{"raw":"https://example.test/?a=1","hash":"fragment","query":[]}}}]}`)
	if r.Collection.Items[0].Request.URL != "https://example.test/#fragment" {
		t.Fatal("structured URL fragment lost")
	}
	parseFails(t, `{"info":{"name":"C"},"item":[{"name":"R","request":{"url":{"raw":"https://example.test/#one","hash":"two"}}}]}`, "invalid_structure")
	rows := strings.Repeat(`{"key":"x","value":"y"},`, maxRows/2-1) + `{"key":"x","value":"y"}`
	input := `{"info":{"name":"C"},"variable":[` + rows + `],"item":[{"name":"R","request":{"url":"x","header":[` + rows + `]}}]}`
	parseOK(t, input)
	parseFails(t, strings.Replace(input, `"url":"x"`, `"url":"x?a=1"`, 1), "limit_exceeded")
}

func TestImportOmittedMetadataAndUnsafeExtensions(t *testing.T) {
	r := parseOK(t, `{"info":{"name":"C","author":"private-marker"},"item":[],"vendor_notes":{"private-marker":true}}`)
	if !hasWarning(r, "metadata_omitted") {
		t.Fatal("unknown nonempty metadata disappeared")
	}
	for _, input := range []string{`{"info":{"name":"C"},"item":[],"event":{}}`, `{"info":{"name":"C"},"item":[{"name":"R","request":"x","response":null}]}`} {
		parseFails(t, input, "invalid_structure")
	}
	parseFails(t, `{"info":{"name":"C"},"item":[],"x-auth":{"type":"future"}}`, "unsupported_auth")
	parseFails(t, pmRequest(`{"mode":"urlencoded","urlencoded":[],"options":{"encoding":"future"}}`, ""), "unsupported_body")
	parseFails(t, harInput(`{"mimeType":"application/pdf","text":"private-marker"}`), "unsupported_body")
	parseFails(t, strings.Replace(harInput(""), `"request":{`, `"request":{"auth":{"type":"future"},`, 1), "unsupported_auth")
}

func TestImportValidationIsBounded(t *testing.T) {
	for _, input := range []string{"", `{`, `{} {}`, `{"info":{"name":"C","name":"D"},"item":[]}`, `{"info":{"name":"C"},"item":[],"x":{"private-marker":1,"private-marker":2}}`, string([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'})} {
		parseFails(t, input, "invalid_json")
	}
	for _, input := range []string{`null`, `[]`, `{"info":null,"item":[]}`, `{"info":{"name":"C"},"item":null}`, `{"version":1,"name":"N","variables":[],"future":true}`, `{"version":1,"name":"N","variables":[{"key":"x","value":null}]}`, `{"version":1,"Name":"N","variables":[]}`, `{"version":1,"name":"C","items":[{"type":"future","name":"N"}]}`} {
		parseFails(t, input, "invalid_structure")
	}
	for _, input := range []string{`{"info":{"name":"C"},"item":[],"log":{}}`, `{"version":1,"name":"N","variables":[],"values":[]}`, `{"version":1,"collections":[]}`} {
		parseFails(t, input, "ambiguous_format")
	}
	for _, input := range []string{`{"openapi":"3.0.0"}`, `{"swagger":"2.0"}`, `{"_type":"export","resources":[]}`, `{"format":"reva-backup-v1","data":"private-marker"}`, `{"collections":[],"environments":[]}`} {
		parseFails(t, input, "unsupported_format")
	}
	for _, input := range []string{`{"version":2,"name":"N","variables":[]}`, `{"log":{"version":"1.3","entries":[]}}`, `{"info":{"name":"P","schema":"https://private-marker/v2.1.0/fake"},"item":[]}`} {
		parseFails(t, input, "unsupported_version")
	}
	base := `{"info":{"name":"C"},"item":[]}`
	parseOK(t, base+strings.Repeat(" ", MaxInputBytes-len(base)))
	parseFails(t, base+strings.Repeat(" ", MaxInputBytes-len(base)+1), "input_too_large")
	rows := strings.Repeat(`{"key":"x","value":"y"},`, maxRows-1) + `{"key":"x","value":"y"}`
	input := `{"_postman_variable_scope":"environment","name":"E","values":[` + rows + `]}`
	if len(parseOK(t, input).Environment.Variables) != maxRows {
		t.Fatal("row boundary changed")
	}
	parseFails(t, strings.Replace(input, `"values":[`, `"values":[{"key":"extra"},`, 1), "limit_exceeded")
	nodes := strings.Repeat(`{"name":"N","request":"x"},`, maxNodes-1) + `{"name":"N","request":"x"}`
	parseOK(t, `{"info":{"name":"C"},"item":[`+nodes+`]}`)
	parseFails(t, `{"info":{"name":"C"},"item":[{"name":"extra","request":"x"},`+nodes+`]}`, "limit_exceeded")
	deep := `{"name":"leaf","request":"x"}`
	for range maxDepth {
		deep = `{"name":"F","item":[` + deep + `]}`
	}
	parseOK(t, `{"info":{"name":"C"},"item":[`+deep+`]}`)
	parseFails(t, `{"info":{"name":"C"},"item":[{"name":"extra","item":[`+deep+`]}]}`, "limit_exceeded")
}

func FuzzParseNeverPanics(f *testing.F) {
	for _, s := range []string{
		string(fixture(f, "postman-v20.json")), string(fixture(f, "postman-v21.json")),
		`{"version":1,"name":"C","items":[]}`, `{"version":1,"name":"E","variables":[]}`,
		`{"_postman_variable_scope":"environment","name":"E","values":[]}`,
		harInput(`{"mimeType":"text/plain","text":"hello"}`), `curl https://example.test -k`,
		strings.Repeat("[", 300), `{"info":null,"info":{}}`,
		`{"info":{"name":"C","\u006eame":"duplicate"},"item":[]}`,
		`{"version":1,"name":"C","items":[],"log":null}`,
		`{"version":1,"name":"E","variables":[{"key":"x","value":null}]}`,
		`{"_postman_variable_scope":"globals","name":"G","values":[]}`,
		pmRequest(`{"mode":"graphql","graphql":{"query":"literal"}}`, ""),
		pmRequest("", `{"type":"bearer","bearer":[{"key":"token","value":null}]}`),
		" \n\ufeff" + `{"version":1,"name":"空","items":null}`,
		`{"format":"reva-backup-v1","ciphertext":"literal"}`,
		harInput(`{"mimeType":"application/json","text":"e30=","encoding":"base64"}`),
	} {
		f.Add(s)
	}
	structuralPath := regexp.MustCompile(`^\$(\.[A-Za-z_][A-Za-z_0-9]*|\[[0-9]+\])*$`)
	f.Fuzz(func(t *testing.T, input string) {
		r, e := Parse(input)
		if e != nil {
			var p *ParseError
			if !errors.As(e, &p) || !reflect.DeepEqual(r, Result{}) {
				t.Fatal("invalid failure contract")
			}
			if p.Code == "" || !structuralPath.MatchString(p.Path) || p.Error() != p.Code+": "+p.Path {
				t.Fatal("failure diagnostic is not code plus structural path")
			}
			return
		}
		payloads := 0
		for _, present := range []bool{r.Collection != nil, r.Environment != nil, r.Request != nil} {
			if present {
				payloads++
			}
		}
		if payloads != 1 {
			t.Fatal("success must have exactly one payload")
		}
		switch r.Kind {
		case KindPostman, KindNativeCollection, KindHAR:
			if r.Collection == nil {
				t.Fatal("collection kind without collection")
			}
		case KindPostmanEnvironment, KindNativeEnvironment:
			if r.Environment == nil {
				t.Fatal("environment kind without environment")
			}
		case KindCurl:
			if r.Request == nil {
				t.Fatal("curl kind without request")
			}
		default:
			t.Fatal("success has unknown kind")
		}
		seen := map[Diagnostic]bool{}
		for _, diagnostic := range r.Warnings {
			if diagnostic.Code == "" || !structuralPath.MatchString(diagnostic.Path) || seen[diagnostic] {
				t.Fatal("warning is not a unique code plus structural path")
			}
			seen[diagnostic] = true
		}
	})
}
