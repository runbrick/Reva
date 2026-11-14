package gui

import (
	"errors"
	"fmt"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/ui"
)

// authEditorState belongs to its request tab or collection dialog. Inherited
// descriptions cache only scheme and provenance; credentials never enter labels.
type authEditorState struct {
	kind                        model.AuthType
	location                    model.APIKeyIn
	previewKind                 model.AuthType
	source                      auth.Source
	scheme, format, description string
}

var inheritedAuthTypes = []model.AuthType{model.AuthInherit, model.AuthNone, model.AuthBearer, model.AuthBasic, model.AuthAPIKey}

// authEditor edits a value draft, retaining inactive fields when the type changes.
// Empty legacy requests/root configs display None; empty folders display Inherit.
func (a *App) authEditor(c ui.C, cfg *model.Auth, s *authEditorState, inherit, emptyInherits bool) bool {
	s.kind = cfg.Type
	if s.kind == "" {
		s.kind = model.AuthNone
		if emptyInherits {
			s.kind = model.AuthInherit
		}
	}
	choices := model.AuthTypes
	if inherit {
		choices = inheritedAuthTypes
	}
	changed := ui.Choice(c, &s.kind, choices, a.requestAuthTypeLabel).Label(a.S.Request.Auth).Changed()
	if changed {
		cfg.Type = s.kind
		if cfg.Type == model.AuthAPIKey && cfg.In == "" {
			cfg.In = model.APIKeyInHeader
		}
	}
	labels := &a.S.Request
	switch s.kind {
	case model.AuthBearer:
		changed = ui.TextField(c, &cfg.Token, labels.Token, "").Secure().FillWidth().Changed() || changed
	case model.AuthBasic:
		changed = ui.TextField(c, &cfg.Username, labels.Username, "").FillWidth().Changed() || changed
		changed = ui.TextField(c, &cfg.Password, labels.Password, "").Secure().FillWidth().Changed() || changed
	case model.AuthAPIKey:
		changed = ui.TextField(c, &cfg.Key, labels.APIKeyName, "").FillWidth().Changed() || changed
		changed = ui.TextField(c, &cfg.Value, labels.APIKeyValue, "").Secure().FillWidth().Changed() || changed
		s.location = cfg.In
		if s.location == "" {
			s.location = model.APIKeyInHeader
		}
		if ui.Choice(c, &s.location, requestAPIKeyLocations, a.requestAPIKeyInLabel).Label(labels.APIKeyLocation).Changed() {
			cfg.In, changed = s.location, true
		}
	}
	return changed
}

// effectiveAuth reads the latest parent config on every preview/send/copy. A
// missing collection remains an error only for explicitly inherited requests.
func (a *App) effectiveAuth(t *Tab) (model.Auth, auth.Source, error) {
	if t.Request.Auth.Type != model.AuthInherit {
		return auth.Resolve(model.Collection{}, t.Request.ID, t.Request.Auth)
	}
	col, _ := a.Svc.Collections.Get(t.CollectionID)
	return auth.Resolve(col, t.Request.ID, t.Request.Auth)
}

func (a *App) authPreview(c ui.C, s *authEditorState, col model.Collection, itemID string) {
	cfg, source, err := auth.Resolve(col, itemID, model.Auth{Type: model.AuthInherit})
	if err != nil {
		ui.Wrap(c, a.S.Request.AuthMissingContext).Color(c.Theme().Danger)
		return
	}
	scheme := a.requestAuthTypeLabel(cfg.Type)
	format := a.S.Request.AuthFromCollection
	if source.Kind == auth.SourceFolder {
		format = a.S.Request.AuthFromFolder
	}
	if s.previewKind != cfg.Type || s.source != source || s.scheme != scheme || s.format != format {
		s.previewKind, s.source, s.scheme, s.format = cfg.Type, source, scheme, format
		s.description = fmt.Sprintf(format, scheme, source.Name)
	}
	ui.Wrap(c, s.description).Muted()
	if cfg.Type == model.AuthNone || cfg.Type == "" {
		ui.Wrap(c, a.S.Request.AuthParentNone).Muted()
	}
}

// Failures deliberately avoid reflecting credential values or backend text.
func (a *App) authFailure(t *Tab, missing bool) error {
	message := a.S.Request.AuthInvalid
	if missing {
		message = a.S.Request.AuthMissingContext
	}
	t.Err = message
	a.Toast(a.S.Request.AuthError, message, ui.ToastError)
	return errors.New(message)
}
