package gui

import (
	"bytes"
	"encoding/json"

	"github.com/runbrick/reqo/internal/model"
	"github.com/vivaui/viva/ui"
)

// BodyJSON always sends application/json. This value distinguishes a GraphQL
// editor from an ordinary JSON request whose payload also has a query field.
const graphQLBodyTag = "application/graphql+json"

func initGraphQLBody(r *model.Request) {
	r.Method = model.MethodPost
	r.Body = model.Body{Mode: model.BodyJSON, ContentType: graphQLBodyTag, Text: "{\n  \"query\": \"query {\\n  \\n}\",\n  \"variables\": {}\n}"}
}

func (a *App) initGraphQLRequest(t *Tab) {
	t.Request.Name = a.S.Collections.NewGraphQL
	initGraphQLBody(&t.Request)
	t.Dirty = true
	a.requestTab(t).section = requestBody
}

func graphQLFields(body string) (map[string]json.RawMessage, string, string, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &fields) != nil || fields == nil {
		return nil, "", "", false
	}
	var query string
	if json.Unmarshal(fields["query"], &query) != nil {
		return nil, "", "", false
	}
	variables := "{}"
	if value, ok := fields["variables"]; ok {
		var formatted bytes.Buffer
		if json.Indent(&formatted, value, "", "  ") == nil {
			variables = formatted.String()
		} else {
			variables = string(value)
		}
	}
	return fields, query, variables, true
}

func (a *App) graphQLBodySection(c ui.C, t *Tab, s *requestTabState) bool {
	fields, query, variables, ok := graphQLFields(t.Request.Body.Text)
	if !ok {
		s.graphQLError = false
		return false
	}
	if s.graphQLBody != t.Request.Body.Text {
		s.graphQLBody, s.graphQLQuery, s.graphQLVariables = t.Request.Body.Text, query, variables
		s.graphQLError = false
	}
	label := a.S.Request.GraphQLRaw
	if s.graphQLRaw {
		label = a.S.Request.GraphQLEdit
	}
	if ui.Button(c, label).Ghost().Small().Clicked() {
		s.graphQLRaw = !s.graphQLRaw
	}
	if s.graphQLRaw {
		s.graphQLError = false
		return false
	}
	changed := false
	ui.Text(c, a.S.Request.GraphQLQuery).Muted()
	if ui.TextArea(c, &s.graphQLQuery, a.S.Request.GraphQLQuery, "").Mono().FillWidth().Height(Space6 * 4).Changed() {
		t.Dirty = true
		changed = true
	}
	ui.Text(c, a.S.Request.GraphQLVariables).Muted()
	if ui.TextArea(c, &s.graphQLVariables, a.S.Request.GraphQLVariables, "{}").Mono().FillWidth().Height(Space6 * 3).Changed() {
		t.Dirty = true
		changed = true
	}
	var values map[string]json.RawMessage
	s.graphQLError = json.Unmarshal([]byte(s.graphQLVariables), &values) != nil || values == nil
	if s.graphQLError {
		ui.Text(c, a.S.Request.InvalidJSON).Color(c.Theme().Danger)
		return true
	}
	encodedQuery, _ := json.Marshal(s.graphQLQuery)
	fields["query"], fields["variables"] = encodedQuery, json.RawMessage(s.graphQLVariables)
	if encoded, err := json.MarshalIndent(fields, "", "  "); changed && err == nil && string(encoded) != t.Request.Body.Text {
		t.Request.Body.Text = string(encoded)
		s.graphQLBody = t.Request.Body.Text
	}
	return true
}
