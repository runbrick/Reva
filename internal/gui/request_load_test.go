package gui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	requestservice "github.com/runbrick/reva/internal/service/request"
)

func TestRequestLoadPreservesAuthoredURLParamsAndStoredMetadata(t *testing.T) {
	t.Setenv("REVA_HOME", t.TempDir())
	svc := fake.New()
	previewFailOnSend(t, svc)
	r := model.NewRequest("divergent-request")
	r.Method = model.MethodPost
	r.URL = "{{host}}/wire?tag=a&tag=a&api-key=old&api-key=older#original"
	r.Params = []model.KV{
		{Key: "tag", Value: "a", Description: "First occurrence"},
		{Key: "tag", Value: "a", Description: "Second occurrence"},
		{Key: "tag", Value: "b", Description: "Params-only occurrence"},
		{Key: "tag", Value: "a", Description: "Disabled matching row", Disabled: true},
		{Key: "{{key}}", Value: "{{value}}", Description: "Raw placeholders"},
		{Key: "", Value: "blank key", Description: "Authored empty key"},
	}
	col := model.Collection{ID: "collection", Name: "Load test", Variables: []model.KV{{Key: "host", Value: "https://not-expanded.test"}, {Key: "key", Value: "x&y"}, {Key: "value", Value: "private=value"}}, Items: []model.Item{{ID: r.ID, Type: model.ItemRequest, Request: &r}}}
	if err := svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(col)
	svc.Collections = previewReadOnlyCollections{svc.Collections}
	svc.Environments = previewReadOnlyEnvironments{svc.Environments}
	svc.Settings = previewReadOnlySettings{svc.Settings}
	svc.History = previewReadOnlyHistory{svc.History}
	h := newHarness(t, svc)
	requestEnglish(h)
	h.App.OpenRequest(r, col.ID)
	for i := 0; i < 3; i++ {
		requestRepaint(h)
		if !reflect.DeepEqual(h.App.Active().Request, r) || h.App.Active().Dirty {
			t.Fatalf("opening changed raw draft or metadata: %+v, dirty %v", h.App.Active().Request, h.App.Active().Dirty)
		}
	}
	h.click(h.App.S.Request.Preview)
	requestRepaint(h)
	h.click(h.App.S.Request.PreviewClose)
	stored, _ := svc.Collections.Get(col.ID)
	after, _ := json.Marshal(stored)
	if string(before) != string(after) || !reflect.DeepEqual(h.App.Active().Request, r) || h.App.Active().Dirty {
		t.Fatal("load/preview changed the collection, metadata or draft")
	}
	if entries, err := os.ReadDir(os.Getenv("REVA_HOME")); err != nil || len(entries) != 0 {
		t.Fatal("opening wrote local data")
	}
}

func TestRequestLoadPreservesExpandedHTTPQuerySemantics(t *testing.T) {
	for _, tc := range []struct {
		name, query       string
		params, variables []model.KV
		want              url.Values
	}{
		{name: "URL-only pairs and authored repetitions", query: "?tag=a&tag=a&api-key=old&api-key=older", params: []model.KV{{Key: "tag", Value: "a"}, {Key: "tag", Value: "a"}, {Key: "tag", Value: "b"}, {Key: "tag", Value: "a", Disabled: true}}, want: url.Values{"tag": {"a", "a", "b"}, "api-key": {"old", "older"}}},
		{name: "pairs equal only after expansion", query: "?x={{v}}", params: []model.KV{{Key: "x", Value: "a", Description: "Keep raw equality distinct"}}, variables: []model.KV{{Key: "v", Value: "a"}}, want: url.Values{"x": {"a"}}},
		{name: "Params key delimiter", params: []model.KV{{Key: "{{key}}", Value: "a"}}, variables: []model.KV{{Key: "key", Value: "x&y"}}, want: url.Values{"x&y": {"a"}}},
		{name: "Params value delimiters", params: []model.KV{{Key: "x", Value: "{{value}}"}}, variables: []model.KV{{Key: "value", Value: "a&y=b"}}, want: url.Values{"x": {"a&y=b"}}},
		{name: "URL-only value delimiters", query: "?x={{value}}", variables: []model.KV{{Key: "value", Value: "a&y=b"}}, want: url.Values{"x": {"a"}, "y": {"b"}}},
		{name: "URL escaped placeholder remains literal", query: "?x=%7B%7Bv%7D%7D", variables: []model.KV{{Key: "v", Value: "a"}}, want: url.Values{"x": {"{{v}}"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			received := make(chan url.Values, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received <- r.URL.Query()
			}))
			defer server.Close()
			r := model.NewRequest("wire-request")
			r.Method, r.URL, r.Params = model.MethodPost, "{{host}}/wire"+tc.query, tc.params
			values := append([]model.KV{{Key: "host", Value: server.URL}}, tc.variables...)
			if _, err := requestservice.New().Send(context.Background(), r, values); err != nil {
				t.Fatal(err)
			}
			if got := <-received; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("original request semantics = %v, want %v", got, tc.want)
			}
			svc := fake.New()
			svc.Requests = requestservice.New()
			col := model.Collection{ID: "collection", Variables: values, Items: []model.Item{{ID: r.ID, Type: model.ItemRequest, Request: &r}}}
			if err := svc.Collections.Save(col); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			requestEnglish(h)
			h.App.OpenRequest(r, col.ID)
			requestRepaint(h)
			h.click(h.App.S.Request.Send)
			h.waitFor("GUI HTTP response", func() bool { return !h.App.Active().Sending && h.App.Active().Response != nil })
			if got := <-received; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("GUI changed initial query semantics: %v, want %v", got, tc.want)
			}
			if !reflect.DeepEqual(h.App.Active().Request, r) || h.App.Active().Dirty {
				t.Fatal("opening or sending rewrote/expanded the authored draft")
			}
		})
	}
}

func TestRequestLoadKeepsLaterTableEditsAndDeletionsAuthoritative(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	r := model.NewRequest("table-edits")
	r.URL = "https://example.test?url-only=preserved-until-edit&tag=a"
	r.Params = []model.KV{{Key: "tag", Value: "a", Description: "First"}, {Key: "tag", Value: "b", Description: "Second"}, {Key: "off", Value: "hidden", Disabled: true}}
	h.App.OpenRequest(r, "")
	requestRepaint(h)
	requestType(h, h.App.S.Common.Value, "changed", true)
	if got := h.App.Active().Request.URL; got != "https://example.test?tag=changed&tag=b" || !h.App.Active().Dirty {
		t.Fatalf("explicit row edit did not rebuild URL: %q", got)
	}
	h.click(h.App.S.Common.Delete)
	if got := h.App.Active().Request.URL; got != "https://example.test?tag=b" {
		t.Fatalf("explicit deletion restored an obsolete URL pair: %q", got)
	}
	if rows := h.App.Active().Request.Params; len(rows) != 2 || rows[0].Description != "Second" || !rows[1].Disabled {
		t.Fatalf("explicit deletion lost surviving row metadata: %+v", rows)
	}
}
