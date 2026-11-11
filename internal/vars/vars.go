// Package vars substitutes {{name}} placeholders.
package vars

import (
	"strings"

	"github.com/runbrick/reqo/internal/model"
)

// Lookup returns the value of the last enabled variable named name, so that
// later scopes (e.g. environment after collection) override earlier ones.
func Lookup(vars []model.KV, name string) (string, bool) {
	for i := len(vars) - 1; i >= 0; i-- {
		if !vars[i].Disabled && vars[i].Key == name {
			return vars[i].Value, true
		}
	}
	return "", false
}

// Expand replaces every {{name}} in s (whitespace inside the braces is
// ignored) with its value. Unknown names are left as written.
func Expand(s string, vars []model.KV) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	var b strings.Builder
	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			break
		}
		end += start + 2
		name := strings.TrimSpace(s[start+2 : end])
		b.WriteString(s[:start])
		if v, ok := Lookup(vars, name); ok && name != "" {
			b.WriteString(v)
		} else {
			b.WriteString(s[start : end+2])
		}
		s = s[end+2:]
	}
	b.WriteString(s)
	return b.String()
}

// Unresolved returns the distinct names in s that vars does not define.
func Unresolved(s string, vars []model.KV) []string {
	var out []string
	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			return out
		}
		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			return out
		}
		name := strings.TrimSpace(s[start+2 : start+2+end])
		if _, ok := Lookup(vars, name); !ok && name != "" && !contains(out, name) {
			out = append(out, name)
		}
		s = s[start+2+end+2:]
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ExpandKVs returns a copy of kvs with keys and values expanded.
func ExpandKVs(kvs []model.KV, vars []model.KV) []model.KV {
	out := make([]model.KV, len(kvs))
	for i, kv := range kvs {
		out[i] = model.KV{Key: Expand(kv.Key, vars), Value: Expand(kv.Value, vars), Disabled: kv.Disabled}
	}
	return out
}

// Apply returns a copy of req with every user-editable string expanded:
// URL, params, headers, body text/form/urlencoded/file path and auth fields.
func Apply(req model.Request, vars []model.KV) model.Request {
	r := req.Clone()
	r.URL = Expand(r.URL, vars)
	r.Params = ExpandKVs(r.Params, vars)
	r.Headers = ExpandKVs(r.Headers, vars)
	r.Body.Text = Expand(r.Body.Text, vars)
	r.Body.ContentType = Expand(r.Body.ContentType, vars)
	r.Body.FilePath = Expand(r.Body.FilePath, vars)
	r.Body.URLEncoded = ExpandKVs(r.Body.URLEncoded, vars)
	for i := range r.Body.Form {
		r.Body.Form[i].Key = Expand(r.Body.Form[i].Key, vars)
		r.Body.Form[i].Value = Expand(r.Body.Form[i].Value, vars)
	}
	a := &r.Auth
	a.Token = Expand(a.Token, vars)
	a.Username = Expand(a.Username, vars)
	a.Password = Expand(a.Password, vars)
	a.Key = Expand(a.Key, vars)
	a.Value = Expand(a.Value, vars)
	return r
}
