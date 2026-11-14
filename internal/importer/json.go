package importer

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type object = map[string]any

func fail(code, path string) { panic(&ParseError{Code: code, Path: path}) }
func cleanInput(s string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "\ufeff"))
}

// Token decoding rejects duplicate keys before maps can erase them. Paths for
// arbitrary source keys are deliberately hidden to keep diagnostics value-free.
func readJSON(input string) object {
	if !utf8.ValidString(input) {
		fail("invalid_json", "$")
	}
	d := json.NewDecoder(strings.NewReader(input))
	d.UseNumber()
	var walk func(int) any
	walk = func(depth int) any {
		if depth > 1024 {
			fail("limit_exceeded", "$")
		}
		t, err := d.Token()
		if err != nil {
			fail("invalid_json", "$")
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				m := object{}
				for d.More() {
					key, e := d.Token()
					if e != nil {
						fail("invalid_json", "$")
					}
					k, ok := key.(string)
					if !ok {
						fail("invalid_json", "$")
					}
					if _, exists := m[k]; exists {
						fail("invalid_json", "$")
					}
					m[k] = walk(depth + 1)
				}
				if end, e := d.Token(); e != nil || end != json.Delim('}') {
					fail("invalid_json", "$")
				}
				return m
			case '[':
				a := []any{}
				for d.More() {
					a = append(a, walk(depth+1))
				}
				if end, e := d.Token(); e != nil || end != json.Delim(']') {
					fail("invalid_json", "$")
				}
				return a
			default:
				fail("invalid_json", "$")
			}
		}
		return t
	}
	v := walk(0)
	if _, err := d.Token(); err != io.EOF {
		fail("invalid_json", "$")
	}
	return obj(v, "$")
}

func obj(v any, path string) object {
	m, ok := v.(map[string]any)
	if !ok {
		fail("invalid_structure", path)
	}
	return m
}
func arr(v any, path string) []any {
	a, ok := v.([]any)
	if !ok {
		fail("invalid_structure", path)
	}
	return a
}
func text(v any, path string) string {
	s, ok := v.(string)
	if !ok {
		fail("invalid_structure", path)
	}
	return s
}
func str(m object, key, path string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	return text(v, path+"."+key)
}
func boolean(m object, key, path string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		fail("invalid_structure", path+"."+key)
	}
	return b
}
func name(m object, path string) string {
	s := str(m, "name", path)
	if strings.TrimSpace(s) == "" {
		fail("invalid_structure", path+".name")
	}
	return s
}
func index(path string, i int) string { return path + "[" + strconv.Itoa(i) + "]" }
func lexical(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
func nonempty(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

func identify(m object) Kind {
	var candidates []Kind
	_, info := m["info"]
	_, item := m["item"]
	_, version := m["version"]
	_, items := m["items"]
	_, variables := m["variables"]
	_, scope := m["_postman_variable_scope"]
	_, values := m["values"]
	_, log := m["log"]
	_, swagger := m["swagger"]
	_, openapi := m["openapi"]
	if swagger {
		candidates = append(candidates, KindSwagger)
	}
	if openapi {
		candidates = append(candidates, KindOpenAPI)
	}
	if item || info && !swagger && !openapi {
		candidates = append(candidates, KindPostman)
	}
	if version || items {
		if items {
			candidates = append(candidates, KindNativeCollection)
		} else {
			candidates = append(candidates, KindNativeEnvironment)
		}
	} else if variables {
		candidates = append(candidates, KindNativeEnvironment)
	}
	if scope || values {
		candidates = append(candidates, KindPostmanEnvironment)
	}
	if log {
		candidates = append(candidates, KindHAR)
	}
	unsupported := false
	for _, k := range []string{"_type", "__export_format", "resources", "collections", "environments", "format", "snapshot", "encrypted", "ciphertext", "backup"} {
		if _, ok := m[k]; ok {
			unsupported = true
		}
	}
	if len(candidates) > 1 || unsupported && len(candidates) > 0 {
		fail("ambiguous_format", "$")
	}
	if unsupported {
		fail("unsupported_format", "$")
	}
	if len(candidates) == 0 {
		fail("unknown_format", "$")
	}
	return candidates[0]
}
