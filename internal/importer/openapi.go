package importer

import (
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
)

type apiConversion struct {
	*conversion
	root       object
	v2         bool
	options    Options
	collection *model.Collection
	// Count reference expansion too: a small schema must not expand without bounds.
	work        int
	bytes       int
	sampleBytes int
	schemas     map[string]bool
}

func sortedKeys(m object) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (c *conversion) openapi(root object, kind Kind, options Options) *model.Collection {
	v2 := kind == KindSwagger
	if v2 {
		if str(root, "swagger", "$") != "2.0" {
			fail("unsupported_version", "$.swagger")
		}
	} else {
		version := str(root, "openapi", "$")
		if !strings.HasPrefix(version, "3.0.") && !strings.HasPrefix(version, "3.1.") {
			fail("unsupported_version", "$.openapi")
		}
		parts := strings.Split(version, ".")
		if len(parts) != 3 {
			fail("unsupported_version", "$.openapi")
		}
		if _, err := strconv.ParseUint(parts[2], 10, 32); err != nil {
			fail("unsupported_version", "$.openapi")
		}
	}
	info := obj(root["info"], "$.info")
	col := &model.Collection{ID: storage.NewID(), Version: model.SchemaVersion, Name: str(info, "title", "$.info"), Auth: model.Auth{Type: model.AuthNone}}
	if strings.TrimSpace(col.Name) == "" {
		fail("invalid_structure", "$.info.title")
	}
	a := &apiConversion{conversion: c, root: root, v2: v2, options: options, collection: col}
	// Specifications are request templates, not saved runnable requests. Schema
	// constraints, documentation, responses and callbacks have no model equivalent.
	c.warn("openapi_review", "$")
	paths := obj(root["paths"], "$.paths")
	if security, exists := root["security"]; exists {
		_ = arr(security, "$.security")
	}
	folders := map[string]int{}
	for i, path := range sortedKeys(paths) {
		p := index("$.paths", i)
		if strings.HasPrefix(path, "x-") {
			continue
		}
		if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "?#\r\n") {
			fail("invalid_structure", p)
		}
		entry := a.resolve(paths[path], p)
		for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"} {
			value, ok := entry[method]
			if !ok {
				continue
			}
			opPath := p + "." + method
			op := obj(value, opPath)
			c.node(1, opPath)
			r := model.NewRequest(storage.NewID())
			r.Method = model.Method(strings.ToUpper(method))
			r.Name = str(op, "summary", opPath)
			if r.Name == "" {
				r.Name = str(op, "operationId", opPath)
			}
			if r.Name == "" {
				r.Name = string(r.Method) + " " + path
			}
			r.URL = a.requestURL(path, entry, op, opPath)
			security := root["security"]
			if v, exists := op["security"]; exists {
				_ = arr(v, opPath+".security")
				security = v
			}
			r.Auth = a.auth(security, opPath+".security")
			a.parameters(&r, entry, op, opPath)
			if !v2 {
				if body, exists := op["requestBody"]; exists {
					r.Body = a.requestBody(body, opPath+".requestBody")
				}
			}
			if r.Body.Mode == model.BodyJSON && r.Body.ContentType != "" && r.Body.ContentType != "application/json" {
				found := false
				for _, h := range r.Headers {
					if strings.EqualFold(h.Key, "Content-Type") {
						found = true
					}
				}
				if !found {
					r.Headers = append(r.Headers, model.KV{Key: "Content-Type", Value: r.Body.ContentType})
				}
			}
			a.bytes += len(r.URL) + len(r.Name) + len(r.Body.Text)
			for _, rows := range [][]model.KV{r.Params, r.Headers, r.Body.URLEncoded} {
				for _, row := range rows {
					a.bytes += len(row.Key) + len(row.Value) + len(row.Description)
				}
			}
			for _, row := range r.Body.Form {
				a.bytes += len(row.Key) + len(row.Value)
			}
			if a.bytes > MaxInputBytes {
				fail("limit_exceeded", opPath)
			}
			c.checkRequest(&r, opPath)
			item := model.Item{ID: r.ID, Type: model.ItemRequest, Name: r.Name, Request: &r}
			tag := ""
			if v, ok := op["tags"]; ok {
				tags := arr(v, opPath+".tags")
				for j, value := range tags {
					s := text(value, index(opPath+".tags", j))
					if j == 0 {
						tag = s
					}
				}
			}
			if tag == "" {
				col.Items = append(col.Items, item)
				continue
			}
			at, found := folders[tag]
			if !found {
				c.node(0, opPath+".tags")
				at = len(col.Items)
				folders[tag] = at
				col.Items = append(col.Items, model.Item{ID: storage.NewID(), Type: model.ItemFolder, Name: tag, Auth: model.Auth{Type: model.AuthNone}})
			}
			col.Items[at].Items = append(col.Items[at].Items, item)
		}
	}
	return col
}

// Only local JSON pointers are resolved. Paths in diagnostics never contain
// user-supplied pointer names, URLs, credentials or arbitrary property names.
func (a *apiConversion) resolve(v any, path string) object {
	seen := map[string]bool{}
	for depth := 0; ; depth++ {
		a.work++
		if depth > maxDepth || a.work > maxRows {
			fail("limit_exceeded", path)
		}
		m := obj(v, path)
		value, ok := m["$ref"]
		if !ok {
			return m
		}
		ref := text(value, path+".$ref")
		if !strings.HasPrefix(ref, "#/") {
			fail("unsupported_reference", path+".$ref")
		}
		if seen[ref] {
			fail("unsupported_reference", path+".$ref")
		}
		seen[ref] = true
		fragment, err := url.PathUnescape(ref[2:])
		if err != nil {
			fail("unsupported_reference", path+".$ref")
		}
		v = a.root
		for _, token := range strings.Split(fragment, "/") {
			for i := 0; i < len(token); i++ {
				if token[i] == '~' {
					if i+1 >= len(token) || token[i+1] != '0' && token[i+1] != '1' {
						fail("unsupported_reference", path+".$ref")
					}
					i++
				}
			}
			token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
			switch parent := v.(type) {
			case map[string]any:
				v = parent[token]
			case []any:
				i, err := strconv.Atoi(token)
				if err != nil || i < 0 || i >= len(parent) {
					fail("unsupported_reference", path+".$ref")
				}
				v = parent[i]
			default:
				fail("unsupported_reference", path+".$ref")
			}
			if v == nil {
				fail("unsupported_reference", path+".$ref")
			}
		}
		if len(m) > 1 {
			a.warn("metadata_omitted", path)
		}
	}
}

func (a *apiConversion) requestURL(path string, entry, op object, p string) string {
	host, base := "", ""
	if a.v2 {
		base = str(a.root, "basePath", "$")
		if base != "" && !strings.HasPrefix(base, "/") || strings.ContainsAny(base, "?#\r\n") {
			fail("invalid_structure", "$.basePath")
		}
		h := str(a.root, "host", "$")
		scheme := "https"
		if document, err := url.Parse(a.options.DocumentURL); err == nil && (document.Scheme == "http" || document.Scheme == "https") {
			scheme = document.Scheme
		}
		schemes := a.root["schemes"]
		if v, ok := op["schemes"]; ok {
			schemes = v
		}
		if schemes != nil {
			values := arr(schemes, p+".schemes")
			if len(values) > 0 {
				scheme = text(values[0], p+".schemes[0]")
			}
			if len(values) > 1 {
				a.warn("openapi_selection", p+".schemes")
			}
		}
		if scheme != "http" && scheme != "https" {
			fail("invalid_structure", p+".schemes")
		}
		if h != "" {
			host = scheme + "://" + h
		} else if a.options.DocumentURL != "" {
			u, _ := url.Parse(a.options.DocumentURL)
			if u != nil {
				host = u.Scheme + "://" + u.Host
			}
		}
	} else {
		servers := a.root["servers"]
		if v, ok := entry["servers"]; ok {
			servers = v
		}
		if v, ok := op["servers"]; ok {
			servers = v
		}
		address := "/"
		if servers != nil {
			values := arr(servers, p+".servers")
			if len(values) > 0 {
				server := obj(values[0], p+".servers[0]")
				address = str(server, "url", p+".servers[0]")
				if address == "" {
					fail("invalid_structure", p+".servers[0].url")
				}
				if variables, ok := server["variables"]; ok {
					m := obj(variables, p+".servers[0].variables")
					for _, key := range sortedKeys(m) {
						variable := obj(m[key], p+".servers[0].variables")
						value := text(variable["default"], p+".servers[0].variables.default")
						address = strings.ReplaceAll(address, "{"+key+"}", value)
					}
				}
			}
			if len(values) > 1 {
				a.warn("openapi_selection", p+".servers")
			}
		}
		u, err := url.Parse(address)
		if err != nil || strings.ContainsAny(address, "{}\r\n") {
			fail("invalid_structure", p+".servers")
		}
		if a.options.DocumentURL != "" {
			document, err := url.Parse(a.options.DocumentURL)
			if err != nil {
				fail("invalid_source_url", "$")
			}
			u = document.ResolveReference(u)
		}
		if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" {
			fail("invalid_structure", p+".servers")
		}
		base = u.EscapedPath()
		if u.Host != "" {
			scheme := u.Scheme
			if scheme == "" {
				scheme = "https"
			}
			host = scheme + "://" + u.Host
		}
	}
	if host != "" {
		u, err := url.Parse(host)
		if err != nil || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			fail("invalid_structure", p+".servers")
		}
	}
	result := path
	if a.options.AddBasePath {
		result = strings.TrimRight(base, "/") + result
	}
	if a.options.AddHost {
		result = strings.TrimRight(host, "/") + result
	}
	return result
}

func (a *apiConversion) variable(key, value, path string) {
	if key == "" || strings.ContainsAny(key, "{}\r\n") {
		fail("invalid_structure", path)
	}
	for _, row := range a.collection.Variables {
		if row.Key == key {
			if row.Value != value {
				a.warn("path_variables_unresolved", path)
			}
			return
		}
	}
	a.row(1, path)
	a.collection.Variables = append(a.collection.Variables, model.KV{Key: key, Value: value})
}

func (a *apiConversion) auth(v any, path string) model.Auth {
	none := model.Auth{Type: model.AuthNone}
	if v == nil {
		return none
	}
	choices := arr(v, path)
	if len(choices) == 0 {
		return none
	}
	if len(choices) > 1 {
		a.warn("openapi_selection", path)
	}
	requirement := obj(choices[0], index(path, 0))
	if len(requirement) == 0 {
		return none
	}
	if len(requirement) != 1 {
		fail("unsupported_auth", path)
	}
	definitions := a.root["securityDefinitions"]
	if !a.v2 {
		components := obj(a.root["components"], "$.components")
		definitions = components["securitySchemes"]
	}
	defs := obj(definitions, path)
	key := sortedKeys(requirement)[0]
	_ = arr(requirement[key], path)
	scheme := a.resolve(defs[key], path)
	switch str(scheme, "type", path) {
	case "basic":
		if !a.v2 {
			fail("unsupported_auth", path)
		}
		return model.Auth{Type: model.AuthBasic, Username: "{{username}}", Password: "{{password}}"}
	case "http":
		switch strings.ToLower(str(scheme, "scheme", path)) {
		case "basic":
			return model.Auth{Type: model.AuthBasic, Username: "{{username}}", Password: "{{password}}"}
		case "bearer":
			return model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
		}
	case "apiKey":
		in := model.APIKeyIn(str(scheme, "in", path))
		if in != model.APIKeyInHeader && in != model.APIKeyInQuery {
			fail("unsupported_auth", path)
		}
		return model.Auth{Type: model.AuthAPIKey, Key: name(scheme, path), Value: "{{apiKey}}", In: in}
	}
	fail("unsupported_auth", path)
	return none
}

func (a *apiConversion) parameters(r *model.Request, entry, op object, path string) {
	var parameters []object
	positions := map[string]int{}
	for _, m := range []object{entry, op} {
		if v, ok := m["parameters"]; ok {
			for i, value := range arr(v, path+".parameters") {
				p := index(path+".parameters", i)
				param := a.resolve(value, p)
				key := str(param, "in", p) + "\x00" + name(param, p)
				if at, exists := positions[key]; exists {
					parameters[at] = param
				} else {
					positions[key] = len(parameters)
					parameters = append(parameters, param)
				}
			}
		}
	}
	for i, param := range parameters {
		p := index(path+".parameters", i)
		key, in := name(param, p), str(param, "in", p)
		if a.v2 && in == "body" {
			if r.Body.Mode != model.BodyNone {
				fail("unsupported_body", p)
			}
			media := a.consumes(op, path)
			if !jsonMedia(media) && media != "" {
				fail("unsupported_body", p)
			}
			r.Body = model.Body{Mode: model.BodyJSON, ContentType: media, Text: jsonText(a.sample(obj(param["schema"], p+".schema"), p+".schema", 0))}
			continue
		}
		schema := param
		if !a.v2 {
			schema = a.resolve(param["schema"], p+".schema")
		}
		value, present := param["example"]
		if !present {
			value, present = schema["example"]
		}
		if !present {
			value, present = schema["default"]
		}
		if !present {
			value = a.sample(schema, p+".schema", 0)
		}
		disabled := !boolean(param, "required", p) && !present
		description := str(param, "description", p)
		typeName := str(schema, "type", p+".schema")
		if typeName == "object" {
			fail("unsupported_parameter", p)
		}
		if _, objectValue := value.(map[string]any); objectValue {
			fail("unsupported_parameter", p)
		}
		values := []string{lexical(value)}
		if array, ok := value.([]any); ok {
			values = nil
			for _, v := range array {
				switch v.(type) {
				case map[string]any, []any:
					fail("unsupported_parameter", p)
				}
				values = append(values, lexical(v))
			}
			format := str(param, "collectionFormat", p)
			repeated := a.v2 && format == "multi" || !a.v2 && in == "query" && (!hasKey(param, "explode") || boolean(param, "explode", p))
			if !repeated {
				separator := ","
				switch format {
				case "", "csv":
				case "ssv":
					separator = " "
				case "tsv":
					separator = "\t"
				case "pipes":
					separator = "|"
				default:
					fail("unsupported_parameter", p)
				}
				values = []string{strings.Join(values, separator)}
			}
		}
		if style := str(param, "style", p); style != "" && !(in == "query" && style == "form" || (in == "path" || in == "header") && style == "simple") {
			fail("unsupported_parameter", p)
		}
		if boolean(param, "allowReserved", p) {
			fail("unsupported_parameter", p)
		}
		switch in {
		case "path":
			if len(values) != 1 || !boolean(param, "required", p) {
				fail("unsupported_parameter", p)
			}
			if strings.Contains(r.URL, "{"+key+"}") {
				r.URL = strings.ReplaceAll(r.URL, "{"+key+"}", "{{"+key+"}}")
				a.variable(key, values[0], p)
			}
		case "query", "header":
			for _, value := range values {
				row := model.KV{Key: key, Value: value, Description: description, Disabled: disabled}
				if in == "query" {
					r.Params = append(r.Params, row)
				} else {
					r.Headers = append(r.Headers, row)
				}
			}
		case "formData":
			if !a.v2 || typeName == "file" {
				fail("unsupported_body", p)
			}
			media := a.consumes(op, path)
			if media == "multipart/form-data" {
				if r.Body.Mode != model.BodyNone && r.Body.Mode != model.BodyFormData {
					fail("unsupported_body", p)
				}
				r.Body.Mode = model.BodyFormData
				for _, value := range values {
					r.Body.Form = append(r.Body.Form, model.FormField{Key: key, Value: value, Disabled: disabled})
				}
			} else if media == "application/x-www-form-urlencoded" {
				if r.Body.Mode != model.BodyNone && r.Body.Mode != model.BodyURLEncoded {
					fail("unsupported_body", p)
				}
				r.Body.Mode = model.BodyURLEncoded
				for _, value := range values {
					r.Body.URLEncoded = append(r.Body.URLEncoded, model.KV{Key: key, Value: value, Description: description, Disabled: disabled})
				}
			} else {
				fail("unsupported_body", p)
			}
		default:
			fail("unsupported_parameter", p)
		}
	}
	// A malformed/undeclared template remains visible and needs manual repair.
	if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(r.URL, "{{", ""), "}}", ""), "{") {
		a.warn("path_variables_unresolved", path)
	}
}

func hasKey(m object, key string) bool { _, ok := m[key]; return ok }
func jsonMedia(media string) bool {
	return media == "application/json" || strings.HasSuffix(media, "+json")
}
func jsonText(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fail("unsupported_body", "$")
	}
	return string(b)
}

func (a *apiConversion) consumes(op object, path string) string {
	v := a.root["consumes"]
	if value, ok := op["consumes"]; ok {
		v = value
	}
	if v == nil {
		return "application/json"
	}
	values := arr(v, path+".consumes")
	if len(values) == 0 {
		return "application/json"
	}
	if len(values) > 1 {
		a.warn("openapi_selection", path+".consumes")
	}
	return text(values[0], path+".consumes[0]")
}

func (a *apiConversion) requestBody(v any, path string) model.Body {
	body := a.resolve(v, path)
	content := obj(body["content"], path+".content")
	keys := sortedKeys(content)
	if len(keys) == 0 {
		fail("unsupported_body", path)
	}
	media := keys[0]
	for _, k := range keys {
		if k == "application/json" {
			media = k
			break
		}
	}
	if len(keys) > 1 {
		a.warn("openapi_selection", path+".content")
	}
	m := obj(content[media], path+".content")
	if nonempty(m["encoding"]) {
		fail("unsupported_body", path+".content.encoding")
	}
	value, exists := m["example"]
	if !exists && hasKey(m, "examples") {
		examples := obj(m["examples"], path+".content.examples")
		names := sortedKeys(examples)
		if len(names) > 0 {
			example := a.resolve(examples[names[0]], path+".content.examples[0]")
			if hasKey(example, "externalValue") {
				fail("unsupported_reference", path+".content.examples[0]")
			}
			value, exists = example["value"]
			if len(names) > 1 {
				a.warn("openapi_selection", path+".content.examples")
			}
		}
	}
	if !exists {
		value = a.sample(obj(m["schema"], path+".content.schema"), path+".content.schema", 0)
	}
	if jsonMedia(media) {
		return model.Body{Mode: model.BodyJSON, Text: jsonText(value), ContentType: media}
	}
	if strings.HasPrefix(media, "text/") {
		return model.Body{Mode: model.BodyRaw, Text: lexical(value), ContentType: media}
	}
	if media == "application/x-www-form-urlencoded" || media == "multipart/form-data" {
		fields := obj(value, path+".content")
		out := model.Body{Mode: model.BodyURLEncoded}
		if media == "multipart/form-data" {
			out.Mode = model.BodyFormData
		}
		for _, key := range sortedKeys(fields) {
			switch fields[key].(type) {
			case []any, map[string]any:
				fail("unsupported_body", path)
			}
			if out.Mode == model.BodyFormData {
				out.Form = append(out.Form, model.FormField{Key: key, Value: lexical(fields[key])})
			} else {
				out.URLEncoded = append(out.URLEncoded, model.KV{Key: key, Value: lexical(fields[key])})
			}
		}
		return out
	}
	fail("unsupported_body", path)
	return model.Body{}
}

// sample generates a bounded request template. Composed/recursive schemas need
// manual values rather than pretending to satisfy constraints or picking a branch.
func (a *apiConversion) sample(schema object, path string, depth int) any {
	if depth > maxDepth {
		fail("limit_exceeded", path)
	}
	if ref, ok := schema["$ref"]; ok {
		key := text(ref, path+".$ref")
		if a.schemas == nil {
			a.schemas = map[string]bool{}
		}
		if a.schemas[key] {
			fail("unsupported_reference", path+".$ref")
		}
		a.schemas[key] = true
		defer delete(a.schemas, key)
	}
	schema = a.resolve(schema, path)
	for _, key := range []string{"example", "default", "const"} {
		if v, ok := schema[key]; ok {
			a.sampleSize(v, path)
			return v
		}
	}
	if v, ok := schema["enum"]; ok {
		values := arr(v, path+".enum")
		if len(values) > 0 {
			a.sampleSize(values[0], path)
			return values[0]
		}
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if nonempty(schema[key]) {
			fail("unsupported_body", path)
		}
	}
	typeName := ""
	if v, ok := schema["type"]; ok {
		typeName = text(v, path+".type")
	}
	if typeName == "" && hasKey(schema, "properties") {
		typeName = "object"
	}
	switch typeName {
	case "object":
		out := object{}
		if properties, ok := schema["properties"]; ok {
			m := obj(properties, path+".properties")
			for i, key := range sortedKeys(m) {
				p := index(path+".properties", i)
				child := a.resolve(m[key], p)
				if boolean(child, "readOnly", p) {
					continue
				}
				a.row(1, p)
				a.sampleSize(key, p)
				out[key] = a.sample(obj(m[key], p), p, depth+1)
			}
		}
		return out
	case "array":
		return []any{a.sample(obj(schema["items"], path+".items"), path+".items", depth+1)}
	case "integer", "number":
		a.sampleSize(0, path)
		return json.Number("0")
	case "boolean":
		a.sampleSize(false, path)
		return false
	case "null":
		a.sampleSize(nil, path)
		return nil
	case "", "string":
		format := str(schema, "format", path)
		if format == "binary" || format == "byte" {
			fail("unsupported_body", path)
		}
		a.sampleSize("", path)
		return ""
	default:
		fail("unsupported_body", path)
	}
	return nil
}

func (a *apiConversion) sampleSize(value any, path string) {
	encoded, err := json.Marshal(value)
	if err != nil {
		fail("unsupported_body", path)
	}
	a.sampleBytes += len(encoded)
	if a.sampleBytes > MaxInputBytes {
		fail("limit_exceeded", path)
	}
}
