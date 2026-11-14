package importer

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
)

// Validate exact JSON field names and types before decoding: encoding/json alone
// also accepts case-insensitive names and null scalar/struct fields.
func nativeShape(v any, t reflect.Type, path string) {
	if t.Kind() == reflect.Pointer {
		if v == nil {
			return
		}
		nativeShape(v, t.Elem(), path)
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		m := obj(v, path)
		var required []string
		switch t {
		case reflect.TypeOf(model.Request{}):
			required = []string{"name", "method", "url", "body"}
		case reflect.TypeOf(model.Body{}):
			required = []string{"mode"}
		case reflect.TypeOf(model.KV{}), reflect.TypeOf(model.FormField{}):
			required = []string{"key", "value"}
		}
		for _, key := range required {
			if _, ok := m[key]; !ok {
				fail("invalid_structure", path+"."+key)
			}
		}
		fields := map[string]reflect.Type{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			key := strings.Split(f.Tag.Get("json"), ",")[0]
			fields[key] = f.Type
		}
		for k, v := range m {
			ft, ok := fields[k]
			if !ok {
				fail("invalid_structure", path)
			}
			nativeShape(v, ft, path+"."+k)
		}
	case reflect.Slice:
		if v == nil {
			return
		}
		for i, v := range arr(v, path) {
			nativeShape(v, t.Elem(), index(path, i))
		}
	case reflect.String:
		_ = text(v, path)
	case reflect.Bool:
		if _, ok := v.(bool); !ok {
			fail("invalid_structure", path)
		}
	case reflect.Int:
		n, ok := v.(json.Number)
		if !ok {
			fail("invalid_structure", path)
		}
		if _, err := n.Int64(); err != nil {
			fail("invalid_structure", path)
		}
	default:
		fail("invalid_structure", path)
	}
}
func nativeDecode(m object, dest any) {
	nativeShape(m, reflect.TypeOf(dest).Elem(), "$")
	b, _ := json.Marshal(m)
	if json.Unmarshal(b, dest) != nil {
		fail("invalid_structure", "$")
	}
}
func nativeVersion(m object) {
	n, ok := m["version"].(json.Number)
	if !ok {
		fail("invalid_structure", "$.version")
	}
	v, e := n.Int64()
	if e != nil {
		fail("invalid_structure", "$.version")
	}
	if v != model.SchemaVersion {
		fail("unsupported_version", "$.version")
	}
}

func (c *conversion) nativeCollection(m object) *model.Collection {
	nativeVersion(m)
	name(m, "$")
	if _, ok := m["items"]; !ok {
		fail("invalid_structure", "$.items")
	}
	var col model.Collection
	nativeDecode(m, &col)
	validateNativeAuth(col.Auth, "$.auth")
	c.authDynamics(col.Auth, "$.auth")
	c.nativeRows(col.Variables, "$.variables")
	var walk func([]model.Item, int, string)
	walk = func(items []model.Item, depth int, path string) {
		for i := range items {
			p := index(path, i)
			c.node(depth, p)
			item := &items[i]
			if strings.TrimSpace(item.Name) == "" {
				fail("invalid_structure", p+".name")
			}
			switch item.Type {
			case model.ItemFolder:
				if item.Request != nil {
					fail("invalid_structure", p)
				}
				validateNativeAuth(item.Auth, p+".auth")
				c.authDynamics(item.Auth, p+".auth")
				walk(item.Items, depth+1, p+".items")
			case model.ItemRequest:
				if item.Request == nil || item.Items != nil || item.Auth != (model.Auth{}) {
					fail("invalid_structure", p)
				}
				r := item.Request
				if strings.TrimSpace(r.Name) == "" || r.Name != item.Name || !validMethod(string(r.Method)) {
					fail("invalid_structure", p+".request")
				}
				validateNativeAuth(r.Auth, p+".request.auth")
				validateNativeBody(r.Body, p+".request.body")
				if r.Options.TimeoutMs < 0 || int64(r.Options.TimeoutMs) > 9223372036854 {
					fail("invalid_structure", p+".request.options.timeoutMs")
				}
				for _, v := range []model.Toggle{r.Options.FollowRedirects, r.Options.VerifyTLS} {
					if v != model.ToggleInherit && v != model.ToggleOn && v != model.ToggleOff {
						fail("invalid_structure", p+".request.options")
					}
				}
				c.checkRequest(r, p+".request")
				c.nativeDynamics(r, p+".request")
			default:
				fail("invalid_structure", p+".type")
			}
			item.ID = storage.NewID()
			if item.Request != nil {
				item.Request.ID = item.ID
			}
		}
	}
	walk(col.Items, 0, "$.items")
	col.ID = storage.NewID()
	return &col
}
func (c *conversion) nativeEnvironment(m object) *model.Environment {
	nativeVersion(m)
	name(m, "$")
	if _, ok := m["variables"]; !ok {
		fail("invalid_structure", "$.variables")
	}
	var e model.Environment
	nativeDecode(m, &e)
	c.nativeRows(e.Variables, "$.variables")
	e.ID = storage.NewID()
	return &e
}
func validateNativeAuth(a model.Auth, path string) {
	switch a.Type {
	case "", model.AuthNone, model.AuthInherit, model.AuthBearer, model.AuthBasic:
	case model.AuthAPIKey:
		if a.In != "" && a.In != model.APIKeyInHeader && a.In != model.APIKeyInQuery {
			fail("invalid_auth", path)
		}
	default:
		fail("unsupported_auth", path)
	}
	// Inactive credential fields are intentionally retained: Reva's auth editor
	// changes Type without clearing them. They do not affect the selected scheme.
	if a.In != "" && a.In != model.APIKeyInHeader && a.In != model.APIKeyInQuery {
		fail("invalid_auth", path)
	}
}
func validateNativeBody(b model.Body, path string) {
	switch b.Mode {
	case "", model.BodyNone, model.BodyRaw, model.BodyJSON, model.BodyURLEncoded, model.BodyFormData, model.BodyBinary:
	default:
		fail("unsupported_body", path)
	}
}
func (c *conversion) nativeRows(rows []model.KV, path string) {
	c.row(len(rows), path)
	for i, r := range rows {
		c.dynamic(r.Key, index(path, i)+".key")
		c.dynamic(r.Value, index(path, i)+".value")
	}
}
func (c *conversion) nativeDynamics(r *model.Request, path string) {
	c.dynamic(r.URL, path+".url")
	c.dynamic(r.Body.Text, path+".body.text")
	for _, rows := range [][]model.KV{r.Params, r.Headers, r.Body.URLEncoded} {
		for _, row := range rows {
			c.dynamic(row.Key, path)
			c.dynamic(row.Value, path)
		}
	}
	for _, s := range []string{r.Auth.Token, r.Auth.Username, r.Auth.Password, r.Auth.Key, r.Auth.Value, r.Body.FilePath} {
		c.dynamic(s, path)
	}
	for _, f := range r.Body.Form {
		c.dynamic(f.Key, path+".body.form")
		c.dynamic(f.Value, path+".body.form")
	}
}

func (c *conversion) authDynamics(a model.Auth, path string) {
	for _, s := range []string{a.Token, a.Username, a.Password, a.Key, a.Value} {
		c.dynamic(s, path)
	}
}
