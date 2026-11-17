// Package preview builds a deliberately redacted, read-only view of a draft
// before pre-request scripts. It does not construct a wire request or perform IO.
package preview

import (
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/vars"
)

const Mask = "[masked]"

type Scope string

const (
	Collection  Scope = "collection"
	Environment Scope = "environment"
)

type State string

const (
	Winner   State = "winner"
	Shadowed State = "shadowed"
	Disabled State = "disabled"
)

// Variable contains provenance only, never the configured value/description.
type Variable struct {
	Name  string
	Scope Scope
	Row   int // One-based position in its stored scope, including disabled rows.
	State State
}

// Snapshot contains only display-safe data. Auth source names, IDs, body content,
// file paths, scripts, credentials and variable values are intentionally absent.
type Snapshot struct {
	URL                                      string
	InvalidURL                               bool
	Variables                                []Variable
	Unresolved                               []string
	HeaderNames                              []string
	EnvironmentName                          string
	AuthType                                 model.AuthType
	AuthSource                               auth.SourceKind
	AuthMissing, AuthInvalid, AuthOverridden bool
	AuthQuery                                bool
	BodyMode                                 model.BodyMode
}

// Build reads only its arguments and returns a redacted snapshot. All variable
// substitutions in routing use Mask, even non-secret host/base-URL variables.
// Every query/header value and the entire body are hidden, regardless of name.
func Build(draft model.Request, col model.Collection, env model.Environment) Snapshot {
	values := append(slices.Clone(col.Variables), env.Variables...)
	cfg, source, err := auth.Resolve(col, draft.ID, draft.Auth)
	r := draft.Clone()
	r.Auth = cfg
	expanded := vars.Apply(r, values)
	redact := redactor(values, draft.Auth, col.Auth, cfg, expanded.Auth)
	// Explicit credential headers can also be repeated in literal URL routing.
	for _, h := range expanded.Headers {
		if strings.EqualFold(h.Key, "Authorization") || strings.EqualFold(h.Key, "Proxy-Authorization") || strings.EqualFold(h.Key, "Cookie") {
			redact = append(redact, h.Value)
		}
	}
	safe := redact.cleaner()
	s := Snapshot{EnvironmentName: safe(env.Name), AuthSource: source.Kind, AuthMissing: err != nil}
	s.AuthType = cfg.Type
	switch s.AuthType {
	case model.AuthNone, model.AuthBearer, model.AuthBasic, model.AuthAPIKey:
	default:
		s.AuthType = model.AuthNone
	}
	s.AuthInvalid = err == nil && auth.Validate(expanded.Auth) != nil
	s.AuthQuery = cfg.Type == model.AuthAPIKey && cfg.In == model.APIKeyInQuery && !s.AuthMissing && !s.AuthInvalid
	s.BodyMode = draft.Body.Mode
	switch s.BodyMode {
	case "", model.BodyNone, model.BodyRaw, model.BodyJSON, model.BodyURLEncoded, model.BodyFormData, model.BodyBinary:
	default:
		s.BodyMode = model.BodyNone
	}
	last := make(map[string]int)
	for i, kv := range values {
		if !kv.Disabled {
			last[kv.Key] = i
		}
	}
	for i, kv := range values {
		v := Variable{Name: safe(kv.Key), Scope: Collection, Row: i + 1, State: Winner}
		if i >= len(col.Variables) {
			v.Scope, v.Row = Environment, i-len(col.Variables)+1
		}
		if kv.Disabled {
			v.State = Disabled
		} else if last[kv.Key] != i {
			v.State = Shadowed
		}
		s.Variables = append(s.Variables, v)
	}
	// Reuse the real expansion scanner for unknown names; only active inputs
	// participate. Scripts and inactive body/auth fields are never evaluated.
	unknown := func(text string) {
		for _, name := range vars.Unresolved(text, values) {
			name = safe(name)
			if !slices.Contains(s.Unresolved, name) {
				s.Unresolved = append(s.Unresolved, name)
			}
		}
	}
	unknown(draft.URL)
	for _, rows := range [][]model.KV{draft.Params, draft.Headers} {
		for _, kv := range rows {
			if !kv.Disabled {
				unknown(kv.Key)
				unknown(kv.Value)
			}
		}
	}
	switch draft.Body.Mode {
	case model.BodyJSON, model.BodyRaw:
		unknown(draft.Body.Text)
		unknown(draft.Body.ContentType)
	case model.BodyBinary:
		unknown(draft.Body.FilePath)
	case model.BodyURLEncoded:
		for _, kv := range draft.Body.URLEncoded {
			if !kv.Disabled {
				unknown(kv.Key)
				unknown(kv.Value)
			}
		}
	case model.BodyFormData:
		for _, field := range draft.Body.Form {
			if !field.Disabled {
				unknown(field.Key)
				unknown(field.Value)
			}
		}
	}
	switch cfg.Type {
	case model.AuthBearer:
		unknown(cfg.Token)
	case model.AuthBasic:
		unknown(cfg.Username)
		unknown(cfg.Password)
	case model.AuthAPIKey:
		unknown(cfg.Key)
		unknown(cfg.Value)
	}
	generated := ""
	if cfg.Type == model.AuthBasic || cfg.Type == model.AuthBearer {
		generated = "Authorization"
	} else if cfg.Type == model.AuthAPIKey && cfg.In != model.APIKeyInQuery {
		generated = expanded.Auth.Key
	}
	for _, h := range expanded.Headers {
		if h.Disabled || h.Key == "" {
			continue
		}
		s.HeaderNames = append(s.HeaderNames, safe(h.Key))
		if generated != "" && strings.EqualFold(h.Key, generated) {
			s.AuthOverridden = true
		}
	}
	if generated != "" && !s.AuthOverridden && !s.AuthInvalid && !s.AuthMissing {
		s.HeaderNames = append(s.HeaderNames, safe(generated))
	}
	s.URL, s.InvalidURL = previewURL(draft, expanded, values, safe, !s.AuthInvalid && !s.AuthMissing)
	return s
}

type secrets []string

func redactor(values []model.KV, configs ...model.Auth) secrets {
	var out secrets
	for _, kv := range values {
		out = append(out, kv.Value)
	}
	for _, cfg := range configs {
		out = append(out, cfg.Token, cfg.Username, cfg.Password, cfg.Key, cfg.Value)
	}
	return out
}

func (values secrets) cleaner() func(string) string {
	// One replacement pass avoids a short secret corrupting a mask inserted
	// for a longer one. Also cover escaped copies in literal routing/metadata.
	unique := make(map[string]bool)
	for _, value := range values {
		if value != "" {
			unique[value], unique[url.QueryEscape(value)], unique[url.PathEscape(value)] = true, true, true
		}
	}
	variants := make([]string, 0, len(unique))
	for value := range unique {
		variants = append(variants, value)
	}
	sort.Slice(variants, func(i, j int) bool { return len(variants[i]) > len(variants[j]) })
	var pairs []string
	for _, value := range variants {
		pairs = append(pairs, value, Mask)
	}
	if len(pairs) == 0 {
		return func(text string) string { return text }
	}
	return strings.NewReplacer(pairs...).Replace
}

func previewURL(draft, expanded model.Request, values []model.KV, safe func(string) string, applyAuth bool) (string, bool) {
	// Validate routing without reflecting parser errors. Replace only unknown
	// placeholders for this check, so an unresolved host still has a usable
	// preview. No parsed credential or expanded target enters the snapshot.
	var unknown []model.KV
	for _, name := range vars.Unresolved(expanded.URL, nil) {
		unknown = append(unknown, model.KV{Key: name, Value: "redacted"})
	}
	check := vars.Expand(expanded.URL, unknown)
	if !strings.Contains(check, "://") {
		check = "http://" + check
	}
	u, parseErr := url.Parse(check)
	if parseErr != nil || u.Hostname() == "" || u.Opaque != "" || u.Scheme != "http" && u.Scheme != "https" {
		return "", true
	}
	// Compare real expanded query pairs before masking, matching the service's
	// multiset merge. Distinct values must not collapse into identical masks.
	raw, _, _ := strings.Cut(expanded.URL, "#")
	_, query, _ := strings.Cut(raw, "?")
	q, err := url.ParseQuery(query)
	if err != nil {
		return "", true // Never echo a URL parser error containing credentials.
	}
	type pair struct{ key, value string }
	remaining := make(map[pair]int)
	for key, vals := range q {
		for _, value := range vals {
			remaining[pair{key, value}]++
		}
	}
	for _, row := range expanded.Params {
		if row.Disabled {
			continue
		}
		p := pair{row.Key, row.Value}
		if remaining[p] > 0 {
			remaining[p]--
		} else {
			q.Add(row.Key, row.Value)
		}
	}
	if applyAuth {
		_ = auth.ApplyQuery(q, expanded.Auth)
	}
	masked := slices.Clone(values)
	for i := range masked {
		masked[i].Value = Mask
	}
	// Only show query names that came from an authored key. A variable value
	// may inject '&secret=value'; those injected names must remain hidden too.
	keyNames := make(map[string]string)
	_, originalQuery, _ := strings.Cut(strings.SplitN(draft.URL, "#", 2)[0], "?")
	original, _ := url.ParseQuery(originalQuery)
	rememberKey := func(key string) {
		keyNames[vars.Expand(key, values)] = vars.Expand(key, masked)
	}
	for key := range original {
		rememberKey(key)
	}
	for _, row := range draft.Params {
		if !row.Disabled {
			rememberKey(row.Key)
		}
	}
	if applyAuth && expanded.Auth.Type == model.AuthAPIKey && expanded.Auth.In == model.APIKeyInQuery {
		rememberKey(draft.Auth.Key)
		// An inherited key is intentionally hidden; it isn't in the draft.
	}
	base, _, _ := strings.Cut(draft.URL, "#")
	base, _, _ = strings.Cut(base, "?")
	base = vars.Expand(base, masked)
	// Split the authority without url.Parse: {{unknown}} and masked host
	// placeholders are intentionally retained. All userinfo is removed.
	if _, rest, ok := strings.Cut(base, "://"); ok {
		prefix := base[:len(base)-len(rest)]
		authority, path, hasPath := strings.Cut(rest, "/")
		if at := strings.LastIndex(authority, "@"); at >= 0 {
			authority = Mask + "@" + authority[at+1:]
		}
		base = prefix + authority
		if hasPath {
			base += "/" + path
		}
	} else if strings.Contains(base, "@") {
		// Scheme-less or malformed userinfo is ambiguous: hide the route.
		base = Mask
	}
	base = safe(base)
	keys := make([]string, 0, len(q))
	for key := range q {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		name, ok := keyNames[key]
		if !ok {
			name = Mask
		}
		for range q[key] {
			parts = append(parts, url.QueryEscape(safe(name))+"="+Mask)
		}
	}
	if len(parts) > 0 {
		base += "?" + strings.Join(parts, "&")
	}
	if strings.Contains(draft.URL, "#") {
		base += "#" + Mask
	}
	return base, false
}
