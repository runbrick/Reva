package importer

import (
	"strings"

	"github.com/runbrick/reqo/internal/model"
)

const (
	maxDepth = 256
	maxNodes = 10000
	maxRows  = 100000
)

type conversion struct {
	warnings    []Diagnostic
	seen        map[Diagnostic]bool
	nodes, rows int
}

func (c *conversion) warn(code, path string) {
	w := Diagnostic{code, path}
	if c.seen == nil {
		c.seen = make(map[Diagnostic]bool)
	}
	if c.seen[w] {
		return
	}
	c.seen[w] = true
	c.warnings = append(c.warnings, w)
}
func (c *conversion) node(depth int, path string) {
	c.nodes++
	if depth > maxDepth || c.nodes > maxNodes {
		fail("limit_exceeded", path)
	}
}
func (c *conversion) row(n int, path string) {
	c.rows += n
	if c.rows > maxRows {
		fail("limit_exceeded", path)
	}
}
func (c *conversion) dynamic(v any, path string) { // paths use only schema keys and array indexes
	if s, ok := v.(string); ok {
		for {
			start := strings.Index(s, "{{")
			if start < 0 {
				return
			}
			s = s[start+2:]
			end := strings.Index(s, "}}")
			if end < 0 {
				return
			}
			key := strings.TrimSpace(s[:end])
			if strings.HasPrefix(key, "$") || strings.HasPrefix(key, "vault:") {
				c.warn("dynamic_variables_unsupported", path)
				return
			}
			s = s[end+2:]
		}
	}
}
func (c *conversion) metadata(m object, path string, known ...string) {
	allowed := map[string]bool{}
	for _, k := range known {
		allowed[k] = true
	}
	for k, v := range m {
		if allowed[k] {
			continue
		}
		if k == "event" || k == "response" {
			_ = arr(v, path+"."+k)
		}
		if !nonempty(v) {
			continue
		}
		switch k {
		case "id", "_postman_id", "_postman_exported_at", "_postman_exported_using", "_exporter_id", "_collection_link":
			continue
		case "event":
			c.warn("scripts_omitted", path+".event")
		case "response":
			c.warn("examples_omitted", path+".response")
		default:
			lower := strings.ToLower(k)
			authKey := strings.TrimLeft(lower, "_")
			if strings.HasSuffix(authKey, "auth") || authKey == "authtype" || authKey == "authconfig" || authKey == "security" || strings.Contains(authKey, "oauth") || strings.Contains(authKey, "authentication") || strings.Contains(authKey, "authorization") {
				fail("unsupported_auth", path)
			}
			if strings.Contains(lower, "encoding") || strings.Contains(lower, "compression") || strings.Contains(lower, "body") || strings.Contains(lower, "postdata") {
				fail("unsupported_body", path)
			}
			c.warn("metadata_omitted", path)
		}
	}
}
func (c *conversion) curlWarnings(input string) {
	words, _ := shellWords(input)
	for i := 1; i < len(words); i++ {
		a := words[i]
		if a == "--" {
			break
		}
		if !strings.HasPrefix(a, "-") {
			continue
		}
		option, _, attached := strings.Cut(a, "=")
		if option == "--insecure" || option == "--location" || option == "--compressed" {
			c.warn("transport_flags_ignored", "$")
			continue
		}
		if strings.HasPrefix(a, "--") {
			switch option {
			case "--request", "--url", "--header", "--data", "--data-raw", "--data-binary", "--data-ascii", "--data-urlencode", "--form", "--user", "--user-agent", "--referer", "--cookie":
				if !attached {
					i++
				}
			}
			continue
		}
		if len(a) > 1 && strings.ContainsAny(a[1:2], "XHdFuAeb") {
			if len(a) == 2 {
				i++
			}
			continue
		}
		if strings.ContainsAny(a[1:], "kL") {
			c.warn("transport_flags_ignored", "$")
		}
	}
}
func (c *conversion) checkRequest(r *model.Request, path string) {
	c.row(len(r.Params)+len(r.Headers)+len(r.Body.Form)+len(r.Body.URLEncoded), path)
	if r.Body.Mode == model.BodyBinary {
		c.warn("local_file_references", path+".body")
	}
	for _, f := range r.Body.Form {
		if f.File {
			c.warn("local_file_references", path+".body")
			break
		}
	}
}
