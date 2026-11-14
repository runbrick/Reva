package gui

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func graphQLHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.App.S = i18n.For(i18n.En)
	h.App.mainSplit.Ratio = 0
	h.App.mainSplit.Pos = 550
	requestRepaint(h)
	return h
}

func TestGraphQLMenuCreatesEditablePostAndSendsVariables(t *testing.T) {
	h := graphQLHarness(t)
	h.click(h.App.S.Collections.Add)
	h.click(h.App.S.Collections.NewGraphQL)
	tab := h.App.Active()
	if len(h.App.Tabs()) != 2 || tab.Request.Method != model.MethodPost || tab.Request.Body.Mode != model.BodyJSON || tab.Request.Body.ContentType != graphQLBodyTag || !tab.Dirty {
		t.Fatalf("GraphQL tab = %+v", tab)
	}
	if !h.shows(h.App.S.Request.GraphQLQuery) || !h.shows(h.App.S.Request.GraphQLVariables) {
		t.Fatalf("GraphQL editor missing, split=%+v hits=%v", h.App.mainSplit, h.hitLabels())
	}
	requestType(h, h.App.S.Request.GraphQLQuery, "query User($id: ID!) { user(id: $id) { name } }", true)
	requestType(h, h.App.S.Request.GraphQLVariables, `{"id":"42"}`, true)
	h.typeInto("URL", "https://example.test/graphql")
	requests := make(chan model.Request, 1)
	h.Svc.Requests.(*fake.Requests).Fn = func(_ context.Context, req model.Request) (model.Response, error) {
		requests <- req
		return model.Response{StatusCode: 200, Status: "200 OK"}, nil
	}
	h.click(h.App.S.Request.Send)
	h.waitFor("GraphQL response", func() bool { return tab.Response != nil })
	select {
	case req := <-requests:
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal([]byte(req.Body.Text), &body); err != nil {
			t.Fatal(err)
		}
		if req.Method != model.MethodPost || req.URL != "https://example.test/graphql" || body.Query != "query User($id: ID!) { user(id: $id) { name } }" || body.Variables["id"] != "42" {
			t.Fatalf("sent GraphQL request = %+v; body = %+v", req, body)
		}
	default:
		t.Fatal("request was not sent")
	}
}

func TestGraphQLCreationInsideFolderPersistsAndReopens(t *testing.T) {
	h := graphQLHarness(t)
	col, err := h.Svc.Collections.Create("API")
	if err != nil {
		t.Fatal(err)
	}
	collectionsPaint(h)
	if _, err := h.Svc.Collections.AddFolder(col.ID, "", "Queries"); err != nil {
		t.Fatal(err)
	}
	collectionsPaint(h)
	collectionsContext(h, "Queries", h.App.S.Collections.NewGraphQL)
	tab := h.App.Active()
	stored, _ := h.Svc.Collections.Get(col.ID)
	if len(stored.Items) != 1 || len(stored.Items[0].Items) != 1 || stored.Items[0].Items[0].Request == nil {
		t.Fatalf("GraphQL request not stored inside folder: %+v", stored.Items)
	}
	if tab.CollectionID != col.ID || tab.Request.Method != model.MethodPost || !h.shows(h.App.S.Request.GraphQLQuery) {
		t.Fatal("created request did not open in GraphQL editor")
	}
	requestType(h, h.App.S.Request.GraphQLQuery, "{ viewer { name } }", true)
	h.App.RequestSave(tab)
	stored, _ = h.Svc.Collections.Get(col.ID)
	if stored.Items[0].Items[0].Request.Body.Text != tab.Request.Body.Text || tab.Dirty {
		t.Fatal("query was not saved")
	}
	h.App.CloseTab(h.App.active)
	requestRepaint(h)
	h.click(h.App.S.Collections.NewGraphQL)
	h.click(h.App.S.Request.Body)
	if !h.shows(h.App.S.Request.GraphQLQuery) || h.App.requestTab(h.App.Active()).graphQLQuery != "{ viewer { name } }" {
		t.Fatalf("saved query did not reopen in GraphQL editor: body=%q hits=%v", h.App.Active().Request.Body.Text, h.hitLabels())
	}
}

func TestGraphQLInvalidVariablesBlockSendAndKeepExtraFields(t *testing.T) {
	h := graphQLHarness(t)
	h.click(h.App.S.Collections.Add)
	h.click(h.App.S.Collections.NewGraphQL)
	tab := h.App.Active()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(tab.Request.Body.Text), &payload); err != nil {
		t.Fatal(err)
	}
	payload["operationName"] = json.RawMessage(`"Viewer"`)
	encoded, _ := json.Marshal(payload)
	tab.Request.Body.Text = string(encoded)
	requestRepaint(h)
	requestType(h, h.App.S.Request.GraphQLVariables, `{broken`, true)
	if !h.shows(h.App.S.Request.InvalidJSON) {
		t.Fatal("invalid variables not shown")
	}
	h.click(h.App.S.Request.Send)
	if err := h.D.Key(gfx.KeyEnter, ui.Primary); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Request.SendMenu)
	h.click(h.App.S.Request.SendDownload)
	if tab.Sending || len(h.Svc.History.List()) != 0 {
		t.Fatal("invalid variables were sent")
	}
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	requestType(h, h.App.S.Request.GraphQLVariables, `{"limit":2}`, true)
	if err := json.Unmarshal([]byte(tab.Request.Body.Text), &payload); err != nil {
		t.Fatal(err)
	}
	var variables map[string]int
	if err := json.Unmarshal(payload["variables"], &variables); err != nil {
		t.Fatal(err)
	}
	if string(payload["operationName"]) != `"Viewer"` || variables["limit"] != 2 {
		t.Fatalf("GraphQL fields lost: %s", tab.Request.Body.Text)
	}
}

func TestGraphQLDoesNotTakeOverOrdinaryJSONQuery(t *testing.T) {
	h := graphQLHarness(t)
	tab := h.App.Active()
	tab.Request.Body = model.Body{Mode: model.BodyJSON, Text: `{"query":"search term"}`}
	h.click(h.App.S.Request.Body)
	if h.shows(h.App.S.Request.GraphQLQuery) {
		t.Fatal("ordinary JSON request was treated as GraphQL")
	}
	requestField(h, h.App.S.Request.BodyText, 0)
}
