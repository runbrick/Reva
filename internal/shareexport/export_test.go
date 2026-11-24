package shareexport

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/model"
)

func fixture(t *testing.T) (model.Collection, []byte) {
	t.Helper()
	data, err := os.ReadFile("testdata/collection.json")
	if err != nil {
		t.Fatal(err)
	}
	var col model.Collection
	if err := json.Unmarshal(data, &col); err != nil {
		t.Fatal(err)
	}
	col.Variables = append(col.Variables, slotCollision)
	return col, data
}

var slotCollision = model.KV{Key: "REVA_REQUEST_" + "1_URL", Value: "SECRET_SLOT_COLLISION", Disabled: true}

func TestShareExportsRedactStoredValuesAndReportLoss(t *testing.T) {
	col, original := fixture(t)
	for _, format := range []Format{Native, Postman, Hurl, K6} {
		t.Run(format.Filename(), func(t *testing.T) {
			preview, err := Build(col, format)
			if err != nil {
				t.Fatal(err)
			}
			want := map[Format]int{Native: 6, Postman: 5, Hurl: 3, K6: 1}[format]
			if preview.Requests != 6 || preview.Included != want || len(preview.Data) == 0 {
				t.Fatalf("counts: %d of %d, want %d of 6", preview.Included, preview.Requests, want)
			}
			for _, sentinel := range []string{"SECRET_", "example.invalid", "{{localToken}}", "{{host}}"} {
				if strings.Contains(string(preview.Data), sentinel) {
					t.Fatalf("private source text survived: %s", sentinel)
				}
			}
			portable := format >= Hurl
			mustNotice(t, preview, Notice{Issue: Scripts, Request: 5, Skipped: portable})
			mustNotice(t, preview, Notice{Issue: Options, Request: 5, Skipped: portable})
			mustNotice(t, preview, Notice{Issue: QuerySyntax, Request: 1})
			if format != Native {
				mustNotice(t, preview, Notice{Issue: Protocol, Request: 6, Skipped: true})
			}
			if format == K6 {
				mustNotice(t, preview, Notice{Issue: DuplicateHeader, Request: 1, Skipped: true})
			}
			again, err := Build(col, format)
			if err != nil || !bytes.Equal(again.Data, preview.Data) || !slices.Equal(again.Notices, preview.Notices) {
				t.Fatal("export was not deterministic")
			}
		})
	}
	var baseline model.Collection
	if err := json.Unmarshal(original, &baseline); err != nil {
		t.Fatal(err)
	}
	baseline.Variables = append(baseline.Variables, slotCollision)
	before, _ := json.Marshal(baseline)
	after, _ := json.Marshal(col)
	if !bytes.Equal(before, after) {
		t.Fatal("building modified the original collection")
	}
}

func mustNotice(t *testing.T, preview Preview, want Notice) {
	t.Helper()
	if !slices.Contains(preview.Notices, want) {
		t.Fatalf("missing notice %#v in %v", want, preview.Notices)
	}
}

func TestGRPCShareAndNativeRoundTrip(t *testing.T) {
	const endpoint = "https://localhost:50051/example.Service/Call"
	request := model.NewRequest("grpc")
	request.Name, request.Method, request.URL = "Unary", model.MethodGRPC, endpoint
	request.Body = model.Body{Mode: model.BodyRaw, Text: "082a"}
	request.Headers = []model.KV{{Key: "x-test", Value: "fixture-value"}}
	collection := model.Collection{ID: "collection", Name: "Services", Version: model.SchemaVersion, Items: []model.Item{
		{ID: request.ID, Name: request.Name, Type: model.ItemRequest, Request: &request},
	}}
	data, err := json.Marshal(collection)
	if err != nil {
		t.Fatal(err)
	}
	result, err := importer.Parse(string(data))
	if err != nil || result.Collection == nil {
		t.Fatalf("native import: %v", err)
	}
	restored := result.Collection.Items[0].Request
	if restored.Method != model.MethodGRPC || restored.Body.Mode != model.BodyRaw || restored.Body.Text != "082a" || restored.URL != endpoint || !slices.Equal(restored.Headers, request.Headers) {
		t.Fatalf("native import lost gRPC fields: %+v", restored)
	}
	for _, format := range []Format{Native, Postman, Hurl, K6} {
		preview, err := Build(collection, format)
		if err != nil || preview.Requests != 1 {
			t.Fatalf("format %v: %+v, %v", format, preview, err)
		}
		if format != Native {
			if preview.Included != 0 {
				t.Fatalf("format %v exported gRPC", format)
			}
			mustNotice(t, preview, Notice{Issue: Protocol, Request: 1, Skipped: true})
		} else if preview.Included != 1 || strings.Contains(string(preview.Data), endpoint) || strings.Contains(string(preview.Data), "fixture-value") || strings.Contains(string(preview.Data), "082a") {
			t.Fatalf("native share did not preserve the request or redact its values: %+v", preview)
		}
	}
}

func TestShareJSONRoundTripWithRevaImporter(t *testing.T) {
	col, _ := fixture(t)
	for _, format := range []Format{Native, Postman} {
		t.Run(format.Filename(), func(t *testing.T) {
			preview, err := Build(col, format)
			if err != nil {
				t.Fatal(err)
			}
			result, err := importer.Parse(string(preview.Data))
			if err != nil || result.Collection == nil {
				t.Fatalf("failed to import output: %v", err)
			}
			got := *result.Collection
			if got.Name != col.Name || got.Auth.Type != model.AuthBearer || got.Auth.Token != "{{REVA_COLLECTION_BEARER_TOKEN}}" {
				t.Fatalf("collection or shared auth was lost: %#v", got)
			}
			listing := got.Items[0].Request
			if listing.URL != "{{REVA_REQUEST_1_URL_}}" || len(listing.Params) != 3 || listing.Params[1].Value != "" || !listing.Params[2].Disabled ||
				len(listing.Headers) != 4 || listing.Headers[1].Key != listing.Headers[2].Key || listing.Headers[2].Value != "" || !listing.Headers[3].Disabled {
				t.Fatalf("query/header shape changed: %#v", listing)
			}
			folder := got.Items[1]
			if folder.Auth.Type != model.AuthNone || folder.Items[0].Type != model.ItemFolder || len(folder.Items[0].Items) != 0 {
				t.Fatalf("folder structure/auth barrier changed: %#v", folder)
			}
			health := folder.Items[1].Request
			effective, _, err := auth.Resolve(got, health.ID, health.Auth)
			if err != nil || effective.Type != model.AuthNone || health.Auth.Type != model.AuthInherit {
				t.Fatalf("inheritance no longer stops at folder: %+v, %v", effective, err)
			}
			post := folder.Items[2].Request
			if post.Body.Mode != model.BodyJSON || post.Body.Text != "{{REVA_REQUEST_3_BODY}}" || post.Auth.Type != model.AuthBasic {
				t.Fatalf("body or request auth changed: %#v", post)
			}
			form := got.Items[2].Request.Body.Form
			if !slices.ContainsFunc(form, func(field model.FormField) bool { return field.Key == "note" && !field.File }) {
				t.Fatalf("multipart text field missing: %#v", form)
			}
			if format == Native {
				if len(form) != 2 || !form[0].Disabled || !form[0].File || form[0].Value != "" || got.Items[4].Request.Method != "WS" {
					t.Fatalf("safe native shape changed: %#v", got)
				}
				scope := got.EnvironmentScope
				if scope == nil || scope.ActiveID != "" || len(scope.VisibleIDs) != 0 || len(scope.Local) != 1 || scope.Local[0].Variables[0].Value != "" {
					t.Fatalf("native environment shape changed: %#v", scope)
				}
			} else if len(form) != 1 || got.EnvironmentScope != nil || got.Items[2].Request.Auth.Type != model.AuthNone {
				t.Fatalf("unexpected unsupported fields or legacy auth drift: %#v", got)
			}
		})
	}
}

func TestHurlShareFixture(t *testing.T) {
	col, _ := fixture(t)
	preview, err := Build(col, Hurl)
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("testdata/collection.hurl")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(golden, []byte("\r\n"), []byte("\n")), preview.Data) {
		t.Fatalf("Hurl fixture mismatch:\n%s", preview.Data)
	}
	if hurlfmt, err := exec.LookPath("hurlfmt"); err == nil {
		command := exec.Command(hurlfmt, "--out", "json")
		command.Stdin = bytes.NewReader(preview.Data)
		if output, err := command.CombinedOutput(); err != nil || !json.Valid(output) {
			t.Fatalf("hurlfmt parse: %v\n%s", err, output)
		}
	} else {
		t.Log("hurlfmt unavailable; golden fixture checked")
	}
}

func TestK6ShareJavaScriptSyntaxAndQueryShape(t *testing.T) {
	col, _ := fixture(t)
	preview, err := Build(col, K6)
	if err != nil {
		t.Fatal(err)
	}
	data := string(preview.Data)
	_, data, _ = strings.Cut(data, "const steps = ")
	data, _, _ = strings.Cut(data, ";\n")
	var steps []k6Step
	if err := json.Unmarshal([]byte(data), &steps); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].Ordinal != 2 || steps[0].Method != "HEAD" || steps[0].URL != "REVA_REQUEST_2_URL" ||
		len(steps[0].Headers) != 0 || !slices.Equal(steps[0].Query, []k6Binding{{Key: "tag", Env: "REVA_REQUEST_2_QUERY_1"}, {Key: "tag"}}) {
		t.Fatalf("k6 flow shape changed: %#v", steps)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Log("Node unavailable; static flow data checked")
		return
	}
	path := filepath.Join(t.TempDir(), "fixture.mjs")
	if err := os.WriteFile(path, preview.Data, 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(node, "--check", path).CombinedOutput(); err != nil {
		t.Fatalf("JS syntax check: %v\n%s", err, output)
	}
}

func TestShareBoundsAndEmptyFlow(t *testing.T) {
	col, _ := fixture(t)
	col.Items = col.Items[len(col.Items)-1:]
	preview, err := Build(col, Hurl)
	if err != nil || preview.Requests != 1 || len(preview.Data) != 0 {
		t.Fatalf("unsupported-only flow yielded output: %+v, %v", preview, err)
	}
	col.Variables = make([]model.KV, maxRows+1)
	if preview, err := Build(col, Native); !errors.Is(err, ErrLimit) || preview.Data != nil {
		t.Fatalf("oversized input yielded output: %+v, %v", preview, err)
	}
}

func TestFlowShareAuthPrecedenceAndIncompatibleScopes(t *testing.T) {
	request := func(name string, auth model.Auth, headers []model.KV) model.Item {
		return model.Item{Type: model.ItemRequest, Name: name, Request: &model.Request{
			Name: name, Method: model.MethodGet, URL: "{{baseUrl}}", Auth: auth,
			Headers: headers, Params: []model.KV{{Key: "key", Value: "SECRET_query"}, {Key: "key", Value: ""}},
		}}
	}
	inherit := model.Auth{Type: model.AuthInherit}
	col := model.Collection{Name: "Auth", Auth: model.Auth{Type: model.AuthAPIKey, Key: "key", In: model.APIKeyInQuery, Value: "SECRET_key"}, Items: []model.Item{
		request("Query replace", inherit, nil),
		request("Header override", model.Auth{Type: model.AuthBearer, Token: "SECRET_token"}, []model.KV{{Key: "authorization", Value: ""}}),
		request("Invalid name", model.Auth{Type: model.AuthAPIKey, Key: "{{dynamicKey}}", Value: "SECRET_dynamic"}, nil),
	}}
	for _, format := range []Format{Hurl, K6} {
		preview, err := Build(col, format)
		if err != nil || preview.Included != 2 {
			t.Fatalf("unexpected flow count: %d %v", preview.Included, err)
		}
		mustNotice(t, preview, Notice{Issue: AuthOverride, Request: 1})
		mustNotice(t, preview, Notice{Issue: UnsupportedAuth, Request: 3, Skipped: true})
		output := string(preview.Data)
		if strings.Count(output, "REVA_COLLECTION_API_KEY") != 1 || strings.Contains(output, "REVA_REQUEST_1_QUERY") ||
			strings.Contains(output, "REVA_REQUEST_2_BEARER_TOKEN") || strings.Contains(output, "SECRET_") || !strings.Contains(output, "authorization") {
			t.Fatalf("incorrect auth precedence: %s", output)
		}
	}
}
