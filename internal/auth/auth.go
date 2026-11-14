// Package auth resolves collection authentication and applies concrete,
// already-expanded HTTP authentication without depending on GUI or services.
package auth

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/runbrick/reva/internal/model"
)

// SourceKind identifies the scope that supplied effective authentication.
type SourceKind string

const (
	SourceRequest    SourceKind = "request"
	SourceFolder     SourceKind = "folder"
	SourceCollection SourceKind = "collection"
)

// Source describes provenance without including authentication fields. ID is
// the stable item/collection ID; Name is the folder/collection display name.
// Explicit own authentication has request provenance and no display name.
type Source struct {
	Kind SourceKind
	ID   string
	Name string
}

// Resolve returns unexpanded effective authentication without changing inputs.
// Only an explicit request AuthInherit consults parents; empty request auth is
// legacy no-auth. The current item is excluded, allowing folder parent previews
// too. Explicit own authentication succeeds even without collection membership.
// Unknown explicit types are preserved for Validate after variable expansion.
func Resolve(col model.Collection, itemID string, own model.Auth) (model.Auth, Source, error) {
	if own.Type != model.AuthInherit {
		return concrete(own), Source{Kind: SourceRequest, ID: itemID}, nil
	}
	if col.ID == "" || itemID == "" {
		return model.Auth{}, Source{}, errors.New("inherited authentication requires collection and item context")
	}
	root := concrete(col.Auth)
	if root.Type == model.AuthInherit {
		root = model.Auth{Type: model.AuthNone}
	}
	rootSource := Source{Kind: SourceCollection, ID: col.ID, Name: col.Name}
	var find func([]model.Item, model.Auth, Source) (model.Auth, Source, bool)
	find = func(items []model.Item, parent model.Auth, source Source) (model.Auth, Source, bool) {
		for _, item := range items {
			if item.ID == itemID {
				return parent, source, true
			}
			if item.Type != model.ItemFolder {
				continue
			}
			next, nextSource := parent, source
			if item.Auth.Type != "" && item.Auth.Type != model.AuthInherit {
				next = item.Auth
				nextSource = Source{Kind: SourceFolder, ID: item.ID, Name: item.Name}
			}
			if cfg, origin, found := find(item.Items, next, nextSource); found {
				return cfg, origin, true
			}
		}
		return model.Auth{}, Source{}, false
	}
	if cfg, source, found := find(col.Items, root, rootSource); found {
		return cfg, source, nil
	}
	return model.Auth{}, Source{}, errors.New("inherited authentication item is missing from collection")
}

func concrete(cfg model.Auth) model.Auth {
	if cfg.Type == "" {
		return model.Auth{Type: model.AuthNone}
	}
	return cfg
}

// Validate checks concrete, expanded authentication. Errors never echo fields
// that may contain credentials. Empty basic-auth passwords are supported.
func Validate(cfg model.Auth) error {
	switch cfg.Type {
	case "", model.AuthNone, model.AuthBasic:
		return nil
	case model.AuthBearer:
		if !headerValue(cfg.Token) {
			return errors.New("invalid bearer authentication header value")
		}
	case model.AuthAPIKey:
		if cfg.In != "" && cfg.In != model.APIKeyInHeader && cfg.In != model.APIKeyInQuery {
			return errors.New("invalid API key placement")
		}
		if cfg.In != model.APIKeyInQuery {
			if !headerName(cfg.Key) {
				return errors.New("invalid API key header name")
			}
			if !headerValue(cfg.Value) {
				return errors.New("invalid API key header value")
			}
		}
	case model.AuthInherit:
		return errors.New("inherited authentication must be resolved before sending")
	default:
		return errors.New("unsupported authentication type")
	}
	return nil
}

func headerName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		b := name[i]
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(b)) {
			continue
		}
		return false
	}
	return true
}

func headerValue(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < ' ' && value[i] != '\t' || value[i] == 0x7f {
			return false
		}
	}
	return true
}

// ApplyHeaders adds generated authentication only when no case-insensitive
// explicit header exists, including an explicit empty value. Existing values
// and repetitions are preserved. cfg must already have variables expanded.
func ApplyHeaders(headers http.Header, cfg model.Auth) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	var key, value string
	switch cfg.Type {
	case model.AuthBearer:
		key, value = "Authorization", "Bearer "+cfg.Token
	case model.AuthBasic:
		key, value = "Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cfg.Username+":"+cfg.Password))
	case model.AuthAPIKey:
		if cfg.In != model.APIKeyInQuery {
			key, value = cfg.Key, cfg.Value
		}
	}
	if key == "" {
		return nil
	}
	for existing := range headers {
		if strings.EqualFold(existing, key) {
			return nil
		}
	}
	if headers == nil {
		return errors.New("authentication requires a non-nil header map")
	}
	headers.Set(key, value)
	return nil
}

// ApplyQuery replaces all existing values for an API key in query mode once.
// Other modes leave the query untouched. cfg must already be expanded.
func ApplyQuery(query url.Values, cfg model.Auth) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	if cfg.Type == model.AuthAPIKey && cfg.In == model.APIKeyInQuery {
		if query == nil {
			return errors.New("authentication requires a non-nil query map")
		}
		query.Set(cfg.Key, cfg.Value)
	}
	return nil
}
