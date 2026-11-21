package gui

import (
	"strings"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/vars"
	"github.com/vivaui/viva/ui"
)

type graphQLSchemaState struct {
	endpoint string
	target   *graphQLSchemaTarget
	job      *graphQLSchemaJob
	load     *ui.EffectState[[]graphQLSchemaField, *graphQLSchemaJob]
	fields   []graphQLSchemaField
	matches  []int
	filter   string
	list     ui.ListState
	err      error
}

type graphQLSchemaTarget struct {
	url       string
	params    []model.KV
	queryAuth model.Auth
}

func graphQLSchemaQueryAuth(cfg model.Auth) model.Auth {
	if cfg.Type == model.AuthAPIKey && cfg.In == model.APIKeyInQuery {
		return model.Auth{Type: cfg.Type, In: cfg.In, Key: cfg.Key, Value: cfg.Value}
	}
	return model.Auth{}
}

func (a *App) graphQLSchemaTargetChanged(t *Tab, s *graphQLSchemaState) bool {
	if s.endpoint != t.Request.URL {
		return true
	}
	cfg, _, err := a.effectiveAuth(t)
	if err != nil {
		return true
	}
	queryAuth := graphQLSchemaQueryAuth(cfg)
	var values []model.KV
	needsValues := strings.Contains(t.Request.URL, "{{") || strings.Contains(queryAuth.Key, "{{") || strings.Contains(queryAuth.Value, "{{")
	for _, row := range t.Request.Params {
		needsValues = needsValues || !row.Disabled && (strings.Contains(row.Key, "{{") || strings.Contains(row.Value, "{{"))
	}
	if needsValues {
		values = a.VarsFor(t)
	}
	queryAuth.Key, queryAuth.Value = vars.Expand(queryAuth.Key, values), vars.Expand(queryAuth.Value, values)
	if vars.Expand(t.Request.URL, values) != s.target.url || queryAuth != s.target.queryAuth {
		return true
	}
	index := 0
	for _, row := range t.Request.Params {
		if row.Disabled {
			continue
		}
		if index == len(s.target.params) || vars.Expand(row.Key, values) != s.target.params[index].Key || vars.Expand(row.Value, values) != s.target.params[index].Value {
			return true
		}
		index++
	}
	return index != len(s.target.params)
}

func (a *App) graphQLSchemaAction(c ui.C, t *Tab, s *graphQLSchemaState) {
	if (s.target != nil && a.graphQLSchemaTargetChanged(t, s)) || (s.err != nil && s.endpoint != t.Request.URL) {
		*s = graphQLSchemaState{}
	}
	if s.job != nil {
		ui.Effect(c, s.load, s.job)
		if fields, err, ready := s.load.Take(); ready {
			s.job, s.load, s.fields, s.err = nil, nil, fields, err
			s.updateMatches()
		}
	}
	labels := &a.S.Request
	if s.job == nil {
		if ui.Button(c.Key("load"), labels.GraphQLSchemaLoad).Outline().Small().Clicked() {
			c.After(func() { a.startGraphQLSchema(t, s) })
		}
	} else if ui.Button(c.Key("cancel"), labels.GraphQLSchemaCancel).Outline().Small().Clicked() {
		c.After(func() { s.job, s.load, s.err = nil, nil, errGraphQLSchemaCanceled })
	}
}

func (a *App) startGraphQLSchema(t *Tab, s *graphQLSchemaState) {
	*s = graphQLSchemaState{endpoint: t.Request.URL}
	cfg, _, err := a.effectiveAuth(t)
	if err != nil {
		s.err = errGraphQLSchemaInheritedAuth
		return
	}
	req := vars.Apply(model.Request{
		Method: model.MethodPost, URL: t.Request.URL, Params: t.Request.Params,
		Headers: t.Request.Headers, Auth: cfg, Options: t.Request.Options.Resolve(a.Svc.Settings.Get()),
	}, a.VarsFor(t))
	if strings.TrimSpace(req.URL) == "" {
		s.err = errGraphQLSchemaEndpoint
		return
	}
	if err := auth.Validate(req.Auth); err != nil {
		s.err = errGraphQLSchemaAuth
		return
	}
	s.target = &graphQLSchemaTarget{url: req.URL, queryAuth: graphQLSchemaQueryAuth(req.Auth)}
	for _, row := range req.Params {
		if !row.Disabled {
			s.target.params = append(s.target.params, row)
		}
	}
	s.job = &graphQLSchemaJob{request: req}
	s.load = ui.NewEffectKeyed(loadGraphQLSchema)
}

func (s *graphQLSchemaState) updateMatches() {
	filter := strings.ToLower(strings.TrimSpace(s.filter))
	s.matches = s.matches[:0]
	for index, field := range s.fields {
		if strings.Contains(field.search, filter) {
			s.matches = append(s.matches, index)
		}
	}
	s.list.ScrollTo(0, ui.ListStart)
}

func (a *App) graphQLSchemaResults(c ui.C, s *graphQLSchemaState) {
	labels := &a.S.Request
	switch {
	case s.job != nil:
		ui.Text(c, labels.GraphQLSchemaLoading).Muted()
	case s.err != nil:
		message := labels.GraphQLSchemaFailed
		switch s.err {
		case errGraphQLSchemaInheritedAuth:
			message = labels.AuthMissingContext
		case errGraphQLSchemaAuth:
			message = labels.AuthInvalid
		case errGraphQLSchemaEndpoint:
			message = labels.GraphQLSchemaEndpoint
		case errGraphQLSchemaLarge:
			message = labels.GraphQLSchemaTooLarge
		case errGraphQLSchemaHTTP:
			message = labels.GraphQLSchemaHTTP
		case errGraphQLSchemaRejected:
			message = labels.GraphQLSchemaRejected
		case errGraphQLSchemaInvalid:
			message = labels.GraphQLSchemaInvalid
		case errGraphQLSchemaCanceled:
			ui.Text(c, labels.GraphQLSchemaCanceled).Muted()
			return
		}
		ui.Wrap(c, message).Color(c.Theme().Danger)
	case s.fields != nil:
		a.graphQLSchemaBrowser(c, s)
	}
}

func (a *App) graphQLSchemaBrowser(c ui.C, s *graphQLSchemaState) {
	labels := &a.S.Request
	if len(s.fields) == 0 {
		ui.Text(c, labels.GraphQLSchemaEmpty).Muted()
		return
	}
	if ui.TextField(c, &s.filter, labels.GraphQLSchemaFilter, labels.GraphQLSchemaFilter).FillWidth().Changed() {
		s.updateMatches()
	}
	if len(s.matches) == 0 {
		ui.Text(c, labels.GraphQLSchemaNoMatches).Muted()
		return
	}
	theme := c.Theme()
	rowHeight := theme.Metrics.ControlHeight
	ui.List(c, &s.list, len(s.matches), func(row ui.ListRow) {
		field := s.fields[s.matches[row.Index]]
		row.Element.Height(rowHeight).PadX(Space2).Gap(Space2).Items(ui.Center).Children(func() {
			ui.Text(row.C, field.root).Muted().FontSize(theme.Metrics.FontSizeS)
			ui.Text(row.C, field.signature).Mono().Ellipsis()
		})
	}).FillWidth().Height(rowHeight*float32(min(4, len(s.matches)))).
		Background(theme.Card).Border(1, theme.Border).Radius(theme.Metrics.RadiusControl)
}
