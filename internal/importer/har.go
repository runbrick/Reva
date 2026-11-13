package importer

import (
	"mime"
	"strings"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
)

func (c *conversion) har(m object) *model.Collection {
	l := obj(m["log"], "$.log")
	if str(l, "version", "$.log") != "1.2" {
		fail("unsupported_version", "$.log.version")
	}
	a := arr(l["entries"], "$.log.entries")
	col := &model.Collection{Version: model.SchemaVersion, ID: storage.NewID(), Name: "HAR", Auth: model.Auth{Type: model.AuthNone}, Items: make([]model.Item, 0, len(a))}
	// HAR is request extraction; response/page/timing state never becomes history.
	c.warn("har_response_omitted", "$.log")
	for i, v := range a {
		p := index("$.log.entries", i)
		c.node(0, p)
		entry := obj(v, p)
		q := p + ".request"
		r := obj(entry["request"], q)
		out := model.NewRequest(storage.NewID())
		out.Method = model.Method(text(r["method"], q+".method"))
		if !validMethod(string(out.Method)) {
			fail("invalid_structure", q+".method")
		}
		raw := text(r["url"], q+".url")
		if strings.TrimSpace(raw) == "" {
			fail("invalid_structure", q+".url")
		}
		out.Name = string(out.Method) + " " + raw
		out.URL, out.Params = splitURL(raw)
		c.dynamic(raw, q+".url")
		out.Headers = c.harRows(r["headers"], q+".headers")
		if v, ok := r["queryString"]; ok {
			out.Params = c.harRows(v, q+".queryString")
		} else {
			c.row(len(out.Params), q+".queryString")
		}
		if v, ok := r["cookies"]; ok {
			cookies := arr(v, q+".cookies")
			c.row(len(cookies), q+".cookies")
			for j, v := range cookies {
				cookie := obj(v, index(q+".cookies", j))
				_ = text(cookie["name"], q+".cookies")
				_ = text(cookie["value"], q+".cookies")
			}
			hasCookie := false
			for _, h := range out.Headers {
				if strings.EqualFold(h.Key, "Cookie") {
					hasCookie = true
				}
			}
			if len(cookies) > 0 && !hasCookie {
				c.warn("har_cookies_omitted", q+".cookies")
			}
		}
		if v, ok := r["postData"]; ok {
			out.Body = c.harBody(obj(v, q+".postData"), q+".postData")
		}
		for _, k := range []string{"_encoding", "encoding", "_body", "_postData", "body"} {
			if nonempty(r[k]) {
				fail("unsupported_body", q)
			}
		}
		c.metadata(r, q, "method", "url", "headers", "queryString", "cookies", "postData", "httpVersion", "headersSize", "bodySize")
		c.metadata(entry, p, "request", "response", "cache", "timings", "time", "startedDateTime", "pageref", "serverIPAddress", "connection")
		col.Items = append(col.Items, model.Item{Type: model.ItemRequest, ID: out.ID, Name: out.Name, Request: &out})
	}
	c.metadata(l, "$.log", "version", "entries", "pages", "creator", "browser")
	c.metadata(m, "$", "log")
	return col
}
func (c *conversion) harRows(v any, path string) []model.KV {
	a := arr(v, path)
	c.row(len(a), path)
	out := make([]model.KV, 0, len(a))
	for i, v := range a {
		p := index(path, i)
		m := obj(v, p)
		r := model.KV{Key: text(m["name"], p+".name"), Value: text(m["value"], p+".value"), Description: str(m, "comment", p)}
		c.dynamic(r.Key, p+".name")
		c.dynamic(r.Value, p+".value")
		c.metadata(m, p, "name", "value", "comment")
		out = append(out, r)
	}
	return out
}
func (c *conversion) harBody(m object, path string) model.Body {
	if nonempty(m["comment"]) {
		c.warn("metadata_omitted", path)
	}
	for k, v := range m {
		if nonempty(v) && k != "mimeType" && k != "params" && k != "text" && k != "comment" {
			fail("unsupported_body", path)
		}
	}
	contentType := text(m["mimeType"], path+".mimeType")
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		fail("unsupported_body", path+".mimeType")
	}
	var params []any
	if v, ok := m["params"]; ok {
		params = arr(v, path+".params")
	}
	s, hasText := "", false
	if v, ok := m["text"]; ok {
		s = text(v, path+".text")
		hasText = true
		c.dynamic(s, path+".text")
	}
	if params != nil && hasText && s != "" {
		// URL-encoded text can be compared without losing repeated-row order.
		if media != "application/x-www-form-urlencoded" {
			fail("unsupported_body", path)
		}
		rows := queryRows(s)
		if len(rows) != len(params) {
			fail("unsupported_body", path)
		}
		for i, v := range params {
			p := obj(v, index(path+".params", i))
			if rows[i].Key != str(p, "name", path) || rows[i].Value != str(p, "value", path) {
				fail("unsupported_body", path)
			}
		}
	}
	if len(params) > 0 || !hasText && params != nil {
		out := model.Body{}
		switch media {
		case "application/x-www-form-urlencoded":
			out.Mode = model.BodyURLEncoded
		case "multipart/form-data":
			out.Mode = model.BodyFormData
		default:
			fail("unsupported_body", path)
		}
		c.row(len(params), path+".params")
		for i, v := range params {
			p := index(path+".params", i)
			m := obj(v, p)
			for k, v := range m {
				if nonempty(v) && k != "name" && k != "value" && k != "comment" && k != "contentType" {
					fail("unsupported_body", p)
				}
			}
			if _, ok := m["fileName"]; ok {
				fail("unsupported_body", p)
			}
			key, value := text(m["name"], p+".name"), text(m["value"], p+".value")
			c.dynamic(key, p+".name")
			c.dynamic(value, p+".value")
			if out.Mode == model.BodyFormData {
				if nonempty(m["contentType"]) || nonempty(m["comment"]) {
					c.warn("form_metadata_omitted", p)
				}
				out.Form = append(out.Form, model.FormField{Key: key, Value: value})
			} else {
				if nonempty(m["contentType"]) {
					fail("unsupported_body", p)
				}
				out.URLEncoded = append(out.URLEncoded, model.KV{Key: key, Value: value, Description: str(m, "comment", p)})
			}
		}
		return out
	}
	if !hasText {
		fail("unsupported_body", path)
	}
	textual := strings.HasPrefix(media, "text/") || strings.HasSuffix(media, "+json") || strings.HasSuffix(media, "+xml")
	switch media {
	case "application/json", "application/xml", "application/javascript", "application/ecmascript", "application/graphql", "application/x-www-form-urlencoded":
		textual = true
	}
	if !textual || strings.ContainsRune(s, '\x00') {
		fail("unsupported_body", path)
	}
	return model.Body{Mode: model.BodyRaw, Text: s, ContentType: contentType}
}
