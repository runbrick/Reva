package importer

import (
	"strings"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
)

// Postman keeps the legacy API, with the same blocking validation as Parse.
// It cannot expose recoverable omission warnings; new callers should use Parse.
func Postman(data []byte) (model.Collection, error) {
	r, err := Parse(string(data))
	if err != nil {
		return model.Collection{}, err
	}
	if r.Kind != KindPostman {
		return model.Collection{}, &ParseError{Code: "unknown_format", Path: "$"}
	}
	return *r.Collection, nil
}

func (c *conversion) postman(m object) *model.Collection {
	info := obj(m["info"], "$.info")
	n := name(info, "$.info")
	if schema := str(info, "schema", "$.info"); schema != "" {
		valid := false
		for _, host := range []string{"https://schema.getpostman.com", "https://schema.postman.com"} {
			for _, v := range []string{"v2.0.0", "v2.1.0"} {
				if schema == host+"/json/collection/"+v+"/collection.json" {
					valid = true
				}
			}
		}
		if !valid {
			fail("unsupported_version", "$.info.schema")
		}
	}
	col := &model.Collection{Version: model.SchemaVersion, ID: storage.NewID(), Name: n}
	col.Auth = c.postmanAuth(m["auth"], model.AuthNone, "$.auth")
	col.Items = c.postmanItems(arr(m["item"], "$.item"), 0, "$.item")
	if v, ok := m["variable"]; ok {
		col.Variables = c.postmanRows(v, "$.variable")
	}
	c.metadata(info, "$.info", "name", "schema", "version")
	c.metadata(m, "$", "info", "item", "auth", "variable")
	return col
}

func (c *conversion) postmanItems(a []any, depth int, path string) []model.Item {
	items := make([]model.Item, 0, len(a))
	for i, v := range a {
		p := index(path, i)
		c.node(depth, p)
		m := obj(v, p)
		out := model.Item{Name: name(m, p), ID: storage.NewID()}
		children, folder := m["item"]
		request, req := m["request"]
		if folder == req {
			fail("invalid_structure", p)
		}
		if v, ok := m["type"]; ok {
			kind := text(v, p+".type")
			if folder && kind != "folder" || req && kind != "request" {
				fail("invalid_structure", p+".type")
			}
		}
		if folder {
			out.Type = model.ItemFolder
			out.Auth = c.postmanAuth(m["auth"], model.AuthInherit, p+".auth")
			out.Items = c.postmanItems(arr(children, p+".item"), depth+1, p+".item")
		} else {
			if nonempty(m["auth"]) {
				fail("invalid_structure", p+".auth")
			}
			out.Type = model.ItemRequest
			r := c.postmanRequest(request, p+".request")
			r.ID, r.Name = out.ID, out.Name
			out.Request = &r
		}
		c.metadata(m, p, "name", "item", "request", "auth", "type")
		items = append(items, out)
	}
	return items
}

func (c *conversion) postmanRequest(v any, path string) model.Request {
	r := model.NewRequest(storage.NewID())
	r.Auth.Type = model.AuthInherit
	if s, ok := v.(string); ok {
		if strings.TrimSpace(s) == "" {
			fail("invalid_structure", path)
		}
		r.URL, r.Params = splitURL(s)
		c.row(len(r.Params), path+".url")
		c.dynamic(s, path+".url")
		return r
	}
	m := obj(v, path)
	if method, ok := m["method"]; ok {
		r.Method = model.Method(strings.ToUpper(text(method, path+".method")))
		if !validMethod(string(r.Method)) {
			fail("invalid_structure", path+".method")
		}
	}
	r.URL, r.Params = c.postmanURL(m["url"], path+".url")
	if v, ok := m["header"]; ok {
		if s, ok := v.(string); ok {
			for _, line := range strings.Split(s, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				row, ok := headerRow(strings.TrimSpace(line))
				if !ok {
					fail("invalid_structure", path+".header")
				}
				r.Headers = append(r.Headers, row)
				c.dynamic(line, path+".header")
			}
			c.row(len(r.Headers), path+".header")
		} else {
			r.Headers = c.postmanRows(v, path+".header")
		}
	}
	r.Auth = c.postmanAuth(m["auth"], model.AuthInherit, path+".auth")
	if body, ok := m["body"]; ok && body != nil {
		r.Body = c.postmanBody(obj(body, path+".body"), path+".body")
	}
	c.metadata(m, path, "method", "url", "header", "auth", "body")
	return r
}

func validMethod(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch > 127 || !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", ch) {
			return false
		}
	}
	return true
}

func (c *conversion) postmanURL(v any, path string) (string, []model.KV) {
	if s, ok := v.(string); ok {
		if strings.TrimSpace(s) == "" {
			fail("invalid_structure", path)
		}
		c.dynamic(s, path)
		base, rows := splitURL(s)
		c.row(len(rows), path)
		return base, rows
	}
	m := obj(v, path)
	s := str(m, "raw", path)
	if s != "" {
		if hash := str(m, "hash", path); hash != "" {
			hash = strings.TrimPrefix(hash, "#")
			_, existing, has := strings.Cut(s, "#")
			if has && existing != hash {
				fail("invalid_structure", path+".hash")
			}
			if !has {
				s += "#" + hash
			}
		}
	}
	if s == "" {
		s = joined(m["host"], ".", path+".host")
		if protocol := str(m, "protocol", path); protocol != "" {
			s = protocol + "://" + s
		}
		if port := str(m, "port", path); port != "" {
			s += ":" + port
		}
		if p := joined(m["path"], "/", path+".path"); p != "" {
			s += "/" + strings.TrimPrefix(p, "/")
		}
		if h := str(m, "hash", path); h != "" {
			s += "#" + strings.TrimPrefix(h, "#")
		}
	}
	for _, k := range []string{"protocol", "port", "hash"} {
		if _, ok := m[k]; ok {
			_ = str(m, k, path)
		}
	}
	for _, k := range []string{"host", "path"} {
		if v, ok := m[k]; ok {
			if v == nil {
				fail("invalid_structure", path+"."+k)
			}
			_ = joined(v, "/", path+"."+k)
		}
	}
	if strings.TrimSpace(s) == "" {
		fail("invalid_structure", path)
	}
	base, rows := splitURL(s)
	if q, ok := m["query"]; ok {
		rows = c.postmanRows(q, path+".query")
	} else {
		c.row(len(rows), path+".query")
	}
	if v, ok := m["variable"]; ok {
		_ = c.postmanRows(v, path+".variable")
		if nonempty(v) {
			c.warn("path_variables_unresolved", path+".variable")
		}
	}
	c.dynamic(s, path)
	c.metadata(m, path, "raw", "protocol", "host", "port", "path", "query", "hash", "variable")
	return base, rows
}
func joined(v any, sep, path string) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	a := arr(v, path)
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = text(v, index(path, i))
	}
	return strings.Join(parts, sep)
}

func (c *conversion) postmanRows(v any, path string) []model.KV {
	a := arr(v, path)
	c.row(len(a), path)
	rows := make([]model.KV, 0, len(a))
	for i, v := range a {
		p := index(path, i)
		m := obj(v, p)
		key := str(m, "key", p)
		if _, ok := m["key"]; !ok {
			fail("invalid_structure", p+".key")
		}
		r := model.KV{Key: key, Value: lexical(m["value"]), Disabled: boolean(m, "disabled", p), Description: description(m["description"], p+".description")}
		if v, ok := m["enabled"]; ok {
			b, ok := v.(bool)
			if !ok {
				fail("invalid_structure", p+".enabled")
			}
			if _, both := m["disabled"]; both && r.Disabled == b {
				fail("invalid_structure", p)
			}
			r.Disabled = !b
		}
		if v, ok := m["type"]; ok {
			_ = text(v, p+".type")
		}
		c.dynamic(r.Key, p+".key")
		c.dynamic(r.Value, p+".value")
		c.metadata(m, p, "key", "value", "disabled", "enabled", "description", "type")
		rows = append(rows, r)
	}
	return rows
}
func description(v any, path string) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	m := obj(v, path)
	s := str(m, "content", path)
	for k := range m {
		if k != "content" && k != "type" && k != "version" {
			fail("invalid_structure", path)
		}
	}
	if v, ok := m["type"]; ok {
		_ = text(v, path+".type")
	}
	return s
}

func (c *conversion) postmanAuth(v any, inherited model.AuthType, path string) model.Auth {
	if v == nil {
		return model.Auth{Type: inherited}
	}
	m, ok := v.(map[string]any)
	if !ok {
		fail("invalid_auth", path)
	}
	s, ok := m["type"].(string)
	if !ok || s == "" {
		fail("invalid_auth", path)
	}
	if s != "noauth" && s != "bearer" && s != "basic" && s != "apikey" {
		fail("unsupported_auth", path)
	}
	for k, v := range m {
		if k != "type" && k != s && nonempty(v) {
			fail("invalid_auth", path)
		}
	}
	if s == "noauth" {
		if nonempty(m[s]) {
			fail("invalid_auth", path)
		}
		return model.Auth{Type: model.AuthNone}
	}
	fields, exists := m[s]
	if !exists || fields == nil {
		fail("invalid_auth", path)
	}
	values := map[string]string{}
	allowed := map[string]bool{}
	switch s {
	case "bearer":
		allowed["token"] = true
	case "basic":
		allowed["username"], allowed["password"] = true, true
	case "apikey":
		allowed["key"], allowed["value"], allowed["in"] = true, true, true
	}
	put := func(k string, v any) {
		if !allowed[k] {
			fail("invalid_auth", path)
		}
		if _, dup := values[k]; dup {
			fail("invalid_auth", path)
		}
		value, ok := v.(string)
		if !ok {
			fail("invalid_auth", path)
		}
		values[k] = value
		c.dynamic(value, path)
	}
	switch f := fields.(type) {
	case []any:
		c.row(len(f), path)
		for _, v := range f {
			row, ok := v.(map[string]any)
			if !ok {
				fail("invalid_auth", path)
			}
			k, ok := row["key"].(string)
			if !ok {
				fail("invalid_auth", path)
			}
			for field := range row {
				if field != "key" && field != "value" && field != "type" {
					fail("invalid_auth", path)
				}
			}
			if v, ok := row["type"]; ok {
				if _, ok := v.(string); !ok {
					fail("invalid_auth", path)
				}
			}
			put(k, row["value"])
		}
	case map[string]any:
		c.row(len(f), path)
		for k, v := range f {
			put(k, v)
		}
	default:
		fail("invalid_auth", path)
	}
	out := model.Auth{Type: model.AuthType(s)}
	switch s {
	case "bearer":
		if _, ok := values["token"]; !ok {
			fail("invalid_auth", path)
		}
		out.Token = values["token"]
	case "basic":
		if _, ok := values["username"]; !ok {
			fail("invalid_auth", path)
		}
		out.Username, out.Password = values["username"], values["password"]
	case "apikey":
		out.Key, out.Value, out.In = values["key"], values["value"], model.APIKeyIn(values["in"])
		if out.In == "" {
			if _, present := values["in"]; present {
				fail("invalid_auth", path)
			}
			out.In = model.APIKeyInHeader
		}
		if strings.TrimSpace(out.Key) == "" || (out.In != model.APIKeyInHeader && out.In != model.APIKeyInQuery) {
			fail("invalid_auth", path)
		}
		if _, ok := values["value"]; !ok {
			fail("invalid_auth", path)
		}
	}
	return out
}

func (c *conversion) postmanBody(m object, path string) model.Body {
	out := model.Body{Mode: model.BodyNone}
	mode := str(m, "mode", path)
	// Reject wrong types even in inactive, empty fields; exporters may include
	// empty alternate representations, but may not hide malformed data in them.
	if v, ok := m["raw"]; ok {
		_ = text(v, path+".raw")
	}
	for _, k := range []string{"urlencoded", "formdata"} {
		if v, ok := m[k]; ok {
			_ = arr(v, path+"."+k)
		}
	}
	if v, ok := m["file"]; ok {
		_ = obj(v, path+".file")
	}
	for _, k := range []string{"raw", "urlencoded", "formdata", "file", "graphql"} {
		if k != mode && nonempty(m[k]) {
			fail("invalid_structure", path)
		}
	}
	switch mode {
	case "", "none":
		if mode == "" && len(m) > 0 {
			fail("unsupported_body", path)
		}
	case "raw":
		out.Mode, out.Text = model.BodyRaw, str(m, "raw", path)
		c.dynamic(out.Text, path+".raw")
		if options, ok := m["options"]; ok {
			o := obj(options, path+".options")
			if raw, ok := o["raw"]; ok {
				r := obj(raw, path+".options.raw")
				lang := strings.ToLower(str(r, "language", path+".options.raw"))
				switch lang {
				case "json":
					out.Mode = model.BodyJSON
				case "xml":
					out.ContentType = "application/xml"
				case "html":
					out.ContentType = "text/html"
				case "javascript":
					out.ContentType = "application/javascript"
				}
				c.metadata(r, path+".options.raw", "language")
			}
			c.metadata(o, path+".options", "raw")
		}
	case "urlencoded":
		out.Mode = model.BodyURLEncoded
		out.URLEncoded = c.postmanRows(m["urlencoded"], path+".urlencoded")
	case "formdata":
		out.Mode = model.BodyFormData
		a := arr(m["formdata"], path+".formdata")
		c.row(len(a), path+".formdata")
		for i, v := range a {
			p := index(path+".formdata", i)
			r := obj(v, p)
			f := model.FormField{Key: str(r, "key", p), Value: str(r, "value", p), Disabled: boolean(r, "disabled", p)}
			if _, ok := r["key"]; !ok {
				fail("invalid_structure", p)
			}
			t := str(r, "type", p)
			if t != "" && t != "text" && t != "file" {
				fail("unsupported_body", p)
			}
			f.File = t == "file"
			if nonempty(r["description"]) || nonempty(r["contentType"]) {
				c.warn("form_metadata_omitted", p)
			}
			c.metadata(r, p, "key", "value", "disabled", "type", "src", "description", "contentType")
			if f.File {
				if nonempty(r["value"]) {
					fail("invalid_structure", p)
				}
				c.warn("local_file_references", p)
				src := r["src"]
				if a, ok := src.([]any); ok {
					if len(a) == 0 {
						fail("invalid_structure", p+".src")
					}
					c.row(len(a)-1, p)
					for j, v := range a {
						f.Value = text(v, index(p+".src", j))
						if strings.TrimSpace(f.Value) == "" {
							fail("invalid_structure", p+".src")
						}
						c.dynamic(f.Key, p+".key")
						c.dynamic(f.Value, index(p+".src", j))
						out.Form = append(out.Form, f)
					}
					continue
				}
				f.Value = text(src, p+".src")
				if strings.TrimSpace(f.Value) == "" {
					fail("invalid_structure", p+".src")
				}
			} else if nonempty(r["src"]) {
				fail("invalid_structure", p+".src")
			}
			c.dynamic(f.Key, p+".key")
			c.dynamic(f.Value, p+".value")
			out.Form = append(out.Form, f)
		}
	case "file":
		out.Mode = model.BodyBinary
		f := obj(m["file"], path+".file")
		out.FilePath = str(f, "src", path+".file")
		if strings.TrimSpace(out.FilePath) == "" {
			fail("invalid_structure", path+".file.src")
		}
		c.warn("local_file_references", path+".file")
		c.dynamic(out.FilePath, path+".file.src")
		c.metadata(f, path+".file", "src")
	default:
		fail("unsupported_body", path)
	}
	if mode != "raw" {
		if v, ok := m["options"]; ok {
			c.metadata(obj(v, path+".options"), path+".options")
		}
	}
	for _, k := range []string{"encoding", "_encoding", "contentTransferEncoding", "compression"} {
		if nonempty(m[k]) {
			fail("unsupported_body", path)
		}
	}
	c.metadata(m, path, "mode", "raw", "options", "urlencoded", "formdata", "file", "disabled")
	if boolean(m, "disabled", path) {
		if out.Mode != model.BodyNone {
			c.warn("metadata_omitted", path)
		}
		out = model.Body{Mode: model.BodyNone}
	}
	return out
}
