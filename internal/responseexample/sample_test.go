package responseexample

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/request"
)

func TestWireMockMappingFromHTTPResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		writer.Header().Set("Content-Type", "application/json; profile=private-profile; charset=utf-8")
		writer.Header().Set("Set-Cookie", "session=private-cookie")
		writer.Header().Set("Location", "https://private-host.test/?secret=private-redirect")
		writer.Header().Set("X-Api-Key", "private-response-key")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id":42}`))
	}))
	defer server.Close()
	req := model.NewRequest("local-fixture")
	req.Method = model.MethodPost
	req.URL = server.URL + "/v1/items%20list?secret=private-query#private-fragment"
	req.Headers = []model.KV{{Key: "Authorization", Value: "Bearer private-token"}}
	result, err := request.New().Send(context.Background(), req, nil)
	if err != nil {
		t.Fatal(err)
	}
	sample := FromResponse(req.Method, HTTPPath(req.URL), &result)
	preview, err := sample.Mapping(false)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "request": {
    "method": "POST",
    "urlPath": "/v1/items%20list"
  },
  "response": {
    "status": 201,
    "headers": {
      "Content-Type": "application/json"
    }
  }
}
`
	if string(preview) != want {
		t.Fatalf("metadata-only mapping differs from fixture:\n%s", preview)
	}
	withBody, err := sample.Mapping(true)
	if err != nil || !strings.Contains(string(withBody), `"body": "{\"id\":42}"`) || strings.Contains(string(withBody), "private-") {
		t.Fatalf("reviewed body mapping: %v\n%s", err, withBody)
	}
}

func TestExampleBodyAndPathBounds(t *testing.T) {
	for _, path := range []string{"//secret.test/path", "https://secret.test/path", "/items?key=secret", "/items#secret", "/%2Fsecret.test/path", "/{{token}}", "/item;token=secret", "/p%0a", "/" + strings.Repeat("a", 2048)} {
		if ValidPath(path) {
			t.Errorf("accepted invalid path %q", path)
		}
	}
	if path := HTTPPath("https://user:private-password@private-host.test/items?key=private-key"); path != "/items" {
		t.Fatalf("unexpected path %q", path)
	}
	for _, response := range []model.Response{
		{StatusCode: 200, Body: []byte("partial"), Truncated: true},
		{StatusCode: 200, Body: []byte("partial"), Size: 100},
		{StatusCode: 200, Body: []byte(strings.Repeat("x", MaxBodyBytes+1))},
		{StatusCode: 200, Body: []byte{0, 255}},
		{StatusCode: 200, Body: []byte("ascii binary"), Headers: []model.KV{{Key: "Content-Type", Value: "application/octet-stream"}}},
	} {
		sample := FromResponse(model.MethodGet, "/", &response)
		if sample.Body != "" {
			t.Fatal("stored a body outside the text or size limits")
		}
		if _, err := sample.Mapping(false); err != nil {
			t.Fatal(err)
		}
		if _, err := sample.Mapping(true); err == nil {
			t.Fatal("accepted unavailable body for export")
		}
	}
	template := Sample{Method: model.MethodGet, Path: "/", Status: 200, Body: "{{request.url}}"}
	if !template.valid() || template.CanExportBody() {
		t.Fatal("template markers should be viewable in local examples and unavailable for export")
	}
}
