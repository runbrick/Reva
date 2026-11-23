package gui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/request"
	"github.com/vivaui/viva/ui"
)

const graphQLSchemaFixture = `{"data":{"__schema":{
	"queryType":{"fields":[{"name":"catalogItems","type":{"kind":"NON_NULL","ofType":{"kind":"LIST","ofType":{"kind":"NON_NULL","ofType":{"kind":"OBJECT","name":"Item"}}}}}]},
	"mutationType":{"fields":[{"name":"renameItem","type":{"kind":"OBJECT","name":"Item"}}]},
	"subscriptionType":{"fields":[{"name":"itemChanged","type":{"kind":"SCALAR","name":"ID"}}]}
}}}`

func graphQLSchemaHarness(t *testing.T, svc service.Services, draft model.Request, collectionID string) (*harness, *Tab) {
	t.Helper()
	h := newHarness(t, svc)
	h.App.mainSplit.Ratio = 0
	h.App.mainSplit.Pos = 550
	h.App.OpenRequest(draft, collectionID)
	tab := h.App.Active()
	h.App.requestTab(tab).section = requestBody
	requestEnglish(h)
	requestRepaint(h)
	return h, tab
}

func TestGraphQLSchemaExplicitLoadUsesContextAndPreservesOperation(t *testing.T) {
	type sentOperation struct {
		Method, Path, Query, Auth, Tenant, Header, ContentType, ScriptHeader string
		Payload                                                              map[string]json.RawMessage
	}
	operations := make(chan sentOperation, 4)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		op := sentOperation{Method: r.Method, Path: r.URL.Path, Auth: r.Header.Get("Authorization"),
			Tenant: r.URL.Query().Get("tenant"), Header: r.Header.Get("X-Context"),
			ContentType: r.Header.Get("Content-Type"), ScriptHeader: r.Header.Get("X-Pre")}
		if err := json.NewDecoder(r.Body).Decode(&op.Payload); err != nil {
			t.Error(err)
		}
		if err := json.Unmarshal(op.Payload["query"], &op.Query); err != nil {
			t.Error(err)
		}
		operations <- op
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(op.Query, "__schema") {
			fmt.Fprint(w, graphQLSchemaFixture)
		} else {
			fmt.Fprint(w, `{"data":{"viewer":{"id":"42"}}}`)
		}
	}))
	t.Cleanup(server.Close)

	svc := fake.New()
	svc.Requests = request.New()
	col, err := svc.Collections.Create("GraphQL API")
	authMust(t, err)
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
	col.Variables = []model.KV{{Key: "token", Value: "wrong-token"}, {Key: "tenant", Value: "demo"}}
	env, err := svc.Environments.Create("Local")
	authMust(t, err)
	env.Variables = []model.KV{{Key: "host", Value: server.URL}, {Key: "path", Value: "graphql"}, {Key: "token", Value: "local-token"}}
	authMust(t, svc.Environments.Save(env))
	col.EnvironmentScope.ActiveID = env.ID
	authMust(t, svc.Collections.Save(col))
	draft := model.NewRequest("graphql-schema")
	initGraphQLBody(&draft)
	draft.URL = "{{host}}/{{path}}?tenant={{tenant}}"
	draft.Auth = model.Auth{Type: model.AuthInherit}
	draft.Headers = []model.KV{{Key: "X-Context", Value: "{{tenant}}"}}
	draft.Body.Text = `{"query":"query Viewer($id: ID!) { viewer(id: $id) { id } }","operationName":"Viewer","variables":{"id":"42"}}`
	draft.PreScript = `pm.request.headers.upsert({key: "X-Pre", value: "ran"});`
	draft.TestScript = `pm.test("Normal GraphQL response", function () { pm.response.to.have.status(200); });`
	draft, err = svc.Collections.SaveRequest(col.ID, "", draft)
	authMust(t, err)
	h, tab := graphQLSchemaHarness(t, svc, draft, col.ID)
	labels := h.App.S.Request
	if calls.Load() != 0 || h.shows(labels.GraphQLSchemaLoading) {
		t.Fatal("schema request started before loading explicitly")
	}
	h.click(labels.GraphQLSchemaLoad)
	h.waitFor("root field signatures", func() bool {
		return h.shows("catalogItems: [Item!]!") && h.shows("renameItem: Item") && h.shows("itemChanged: ID")
	})
	schema := <-operations
	if schema.Method != http.MethodPost || schema.Path != "/graphql" || schema.Auth != "Bearer local-token" || schema.Tenant != "demo" || schema.Header != "demo" || schema.ContentType != "application/json" || schema.ScriptHeader != "" {
		t.Fatalf("unexpected schema request: %+v", schema)
	}
	if len(schema.Payload) != 1 || !strings.Contains(schema.Query, "queryType") || !strings.Contains(schema.Query, "mutationType") || !strings.Contains(schema.Query, "subscriptionType") {
		t.Fatalf("unexpected introspection query: %s", schema.Query)
	}
	requestType(h, labels.GraphQLSchemaFilter, " MUTATION ", true)
	if !h.shows("renameItem: Item") || h.shows("catalogItems:") || h.shows("itemChanged:") {
		t.Fatal("case-insensitive root filter did not narrow the results")
	}
	requestType(h, labels.GraphQLSchemaFilter, "missing field", true)
	if !h.shows(labels.GraphQLSchemaNoMatches) {
		t.Fatal("no-match state is missing")
	}
	if calls.Load() != 1 || !reflect.DeepEqual(tab.Request, draft) || tab.Dirty || tab.Sending || tab.Response != nil || len(tab.Tests) != 0 || len(svc.History.List()) != 0 {
		t.Fatal("schema load changed the request, send state, or history")
	}
	stored, _ := svc.Collections.Get(col.ID)
	if !reflect.DeepEqual(*stored.Items[0].Request, draft) {
		t.Fatal("schema load changed the stored operation")
	}
	h.click(labels.Send)
	h.waitFor("normal GraphQL response", func() bool { return tab.Response != nil && !tab.Sending })
	normal := <-operations
	if calls.Load() != 2 || normal.ScriptHeader != "ran" || !strings.HasPrefix(normal.Query, "query Viewer(") || string(normal.Payload["operationName"]) != `"Viewer"` || string(normal.Payload["variables"]) != `{"id":"42"}` || len(tab.Tests) != 1 {
		t.Fatalf("normal sending did not use the original operation and scripts: %+v", normal)
	}
	env.Variables[1].Value = "other-graphql"
	authMust(t, svc.Environments.Save(env))
	requestRepaint(h)
	if h.shows(labels.GraphQLSchemaFilter) || h.shows(labels.GraphQLSchemaNoMatches) || calls.Load() != 2 {
		t.Fatal("schema survived a resolved endpoint change or automatically reloaded")
	}
	h.click(labels.GraphQLSchemaLoad)
	h.waitFor("schema for new environment endpoint", func() bool { return h.shows("catalogItems:") })
	if next := <-operations; next.Path != "/other-graphql" || calls.Load() != 3 {
		t.Fatalf("reloading used the wrong endpoint: %+v", next)
	}
}

func TestGraphQLSchemaCancelsWhenEndpointChangesAndOnRequest(t *testing.T) {
	started := make(chan string, 4)
	stopped := make(chan string, 4)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		started <- r.URL.Path
		if r.URL.Path == "/fast" {
			fmt.Fprint(w, graphQLSchemaFixture)
			return
		}
		select {
		case <-r.Context().Done():
			stopped <- r.URL.Path
		case <-release:
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	draft := model.NewRequest("cancel-schema")
	initGraphQLBody(&draft)
	draft.URL = server.URL + "/slow"
	h, _ := graphQLSchemaHarness(t, fake.New(), draft, "")
	labels := h.App.S.Request
	h.click(labels.GraphQLSchemaLoad)
	h.waitFor("slow endpoint request", func() bool { return len(started) == 1 })
	<-started
	requestType(h, labels.URL, server.URL+"/fast", true)
	h.waitFor("canceled old endpoint request", func() bool { return len(stopped) == 1 })
	if endpoint := <-stopped; endpoint != "/slow" || len(started) != 0 || h.shows(labels.GraphQLSchemaLoading) || h.shows(labels.GraphQLSchemaFilter) {
		t.Fatal("changing endpoint did not clear the in-flight schema without reloading")
	}
	h.click(labels.GraphQLSchemaLoad)
	h.waitFor("new endpoint schema", func() bool { return h.shows("renameItem: Item") })
	<-started
	requestType(h, labels.URL, server.URL+"/cancel", true)
	if h.shows("renameItem: Item") {
		t.Fatal("stale completed schema still visible after URL edit")
	}
	h.click(labels.GraphQLSchemaLoad)
	h.waitFor("cancellable request", func() bool { return len(started) == 1 && h.shows(labels.GraphQLSchemaLoading) })
	<-started
	h.click(labels.GraphQLSchemaCancel)
	h.waitFor("explicit cancellation", func() bool { return len(stopped) == 1 && h.shows(labels.GraphQLSchemaCanceled) })
	if endpoint := <-stopped; endpoint != "/cancel" || len(started) != 0 {
		t.Fatal("cancellation did not stop the requested load")
	}
	h.click(labels.GraphQLSchemaLoad)
	h.waitFor("request before leaving body", func() bool { return len(started) == 1 })
	<-started
	h.click(labels.Headers)
	h.waitFor("effect cancellation after leaving body", func() bool { return len(stopped) == 1 })
	<-stopped
	h.click(labels.Body)
	h.waitFor("schema state after returning to body", func() bool { return h.shows(labels.GraphQLSchemaCanceled) })
	if len(started) != 0 {
		t.Fatal("returning to the body restarted an old schema load")
	}
}

func TestGraphQLSchemaBoundsResponseAndHandlesErrors(t *testing.T) {
	overflowClosed := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		switch r.URL.Path {
		case "/large":
			fmt.Fprint(w, strings.Repeat(" ", graphQLSchemaMaxBytes+1))
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				close(overflowClosed)
			case <-release:
			}
		case "/rejected":
			fmt.Fprint(w, `{"errors":[{"message":"sensitive server details"}]}`)
		case "/invalid":
			fmt.Fprint(w, `{"data":{}}`)
		case "/redirect":
			http.Redirect(w, r, "/unexpected", http.StatusTemporaryRedirect)
		default:
			t.Error("unexpected HTTP request")
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	draft := model.NewRequest("schema-errors")
	initGraphQLBody(&draft)
	draft.URL = server.URL
	h, _ := graphQLSchemaHarness(t, fake.New(), draft, "")
	labels := h.App.S.Request
	for _, example := range []struct{ path, message string }{
		{"/large", labels.GraphQLSchemaTooLarge},
		{"/rejected", labels.GraphQLSchemaRejected},
		{"/invalid", labels.GraphQLSchemaInvalid},
		{"/redirect", labels.GraphQLSchemaHTTP},
	} {
		requestType(h, labels.URL, server.URL+example.path, true)
		h.click(labels.GraphQLSchemaLoad)
		h.waitFor(example.path, func() bool { return h.shows(example.message) })
		if h.shows("sensitive server details") || h.shows(labels.GraphQLSchemaFilter) {
			t.Fatalf("unexpected schema output for %s", example.path)
		}
	}
	h.waitFor("connection closed at the response limit", func() bool {
		select {
		case <-overflowClosed:
			return true
		default:
			return false
		}
	})
}

func BenchmarkGraphQLSchemaPaint(b *testing.B) {
	fields, err := parseGraphQLSchema([]byte(graphQLSchemaFixture))
	if err != nil {
		b.Fatal(err)
	}
	for _, mode := range []struct {
		name   string
		fields []graphQLSchemaField
	}{{"idle", nil}, {"loaded", fields}} {
		b.Run(mode.name, func(b *testing.B) {
			app := New(fake.New())
			tab := app.Active()
			initGraphQLBody(&tab.Request)
			tab.Request.URL = "https://example.test/graphql"
			state := app.requestTab(tab)
			if mode.fields != nil {
				state.graphQLSchema = graphQLSchemaState{
					endpoint: tab.Request.URL, target: &graphQLSchemaTarget{url: tab.Request.URL}, fields: mode.fields,
				}
				state.graphQLSchema.updateMatches()
			}
			authBenchmarkPaint(b, app, func(c ui.C) {
				ui.Column(c).Fill().Pad(Space4).Gap(Space2).Children(func() { app.graphQLBodySection(c, tab, state) })
			}, 800, 400)
		})
	}
}
