package shareexport

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/runbrick/reva/internal/model"
)

type flowRequest struct {
	Ordinal int
	Method  model.Method
	URL     string
	Headers []model.KV
	Query   []model.KV
	Body    string
}

func (build *builder) step(request model.Request, effective model.Auth) (flowRequest, bool) {
	step := flowRequest{
		Ordinal: build.position, Method: request.Method, URL: request.URL,
		Headers: request.Headers, Query: request.Params, Body: request.Body.Text,
	}
	blocked := false
	seen := make(map[string]bool, len(step.Headers))
	for _, row := range step.Headers {
		key := strings.ToLower(row.Key)
		if managedHeader(key) {
			build.notice(ManagedHeader, true)
			blocked = true
		}
		if build.format == K6 && seen[key] {
			build.notice(DuplicateHeader, true)
			blocked = true
		}
		seen[key] = true
	}
	if effective.Type == invalidAuth {
		build.notice(UnsupportedAuth, true)
		blocked = true
	}
	if blocked {
		return flowRequest{}, false
	}
	if key, value := authHeader(effective); key != "" && !seen[strings.ToLower(key)] {
		if managedHeader(strings.ToLower(key)) {
			build.notice(ManagedHeader, true)
			return flowRequest{}, false
		}
		step.Headers = append(step.Headers, model.KV{Key: key, Value: value})
	}
	if effective.Type == model.AuthAPIKey && effective.In == model.APIKeyInQuery {
		var query []model.KV
		for _, row := range step.Query {
			if row.Key != effective.Key {
				query = append(query, row)
			}
		}
		if len(query) != len(step.Query) {
			build.notice(AuthOverride, false)
		}
		step.Query = append(query, model.KV{Key: effective.Key, Value: effective.Value})
	}
	if build.format == Hurl && !seen["content-type"] {
		contentType := request.Body.ContentType
		switch request.Body.Mode {
		case model.BodyJSON:
			contentType = "application/json"
		case model.BodyRaw:
			if contentType == "" {
				contentType = "text/plain"
			}
		}
		if contentType != "" {
			step.Headers = append(step.Headers, model.KV{Key: "Content-Type", Value: contentType})
		}
	}
	return step, true
}

func managedHeader(key string) bool {
	switch key {
	case "host", "content-length", "transfer-encoding", "connection", "trailer", "upgrade", "te":
		return true
	}
	return false
}

func authHeader(config model.Auth) (key, value string) {
	switch config.Type {
	case model.AuthBearer:
		return "Authorization", "Bearer " + config.Token
	case model.AuthBasic:
		return "Authorization", config.Value
	case model.AuthAPIKey:
		if config.In == model.APIKeyInHeader {
			return config.Key, config.Value
		}
	}
	return "", ""
}

func authCollides(request model.Request, auth model.Auth) bool {
	if auth.Type == model.AuthAPIKey && auth.In == model.APIKeyInQuery {
		for _, row := range request.Params {
			if !row.Disabled && row.Key == auth.Key {
				return true
			}
		}
	}
	if key, _ := authHeader(auth); key != "" {
		for _, row := range request.Headers {
			if !row.Disabled && strings.EqualFold(row.Key, key) {
				return true
			}
		}
	}
	return false
}

func renderHurl(steps []flowRequest) []byte {
	var file strings.Builder
	file.WriteString("# Fill REVA_* variables externally. Each URL is complete, without query or fragment.\n")
	for _, step := range steps {
		fmt.Fprintf(&file, "\n# Request %d\n%s %s\n", step.Ordinal, step.Method, step.URL)
		writeHurlRows(&file, step.Headers)
		if len(step.Query) > 0 {
			file.WriteString("[QueryStringParams]\n")
			writeHurlRows(&file, step.Query)
		}
		if step.Body != "" {
			fmt.Fprintf(&file, "```\n%s\n```\n", step.Body)
		}
	}
	return []byte(strings.TrimSuffix(file.String(), "\n"))
}

func writeHurlRows(file *strings.Builder, rows []model.KV) {
	for _, row := range rows {
		file.WriteString(row.Key)
		file.WriteByte(':')
		if row.Value != "" {
			file.WriteByte(' ')
			file.WriteString(row.Value)
		}
		file.WriteByte('\n')
	}
}

type k6Binding struct {
	Key    string `json:"key"`
	Env    string `json:"env,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type k6Step struct {
	Ordinal int          `json:"ordinal"`
	Method  model.Method `json:"method"`
	URL     string       `json:"url"`
	Headers []k6Binding  `json:"headers,omitempty"`
	Query   []k6Binding  `json:"query,omitempty"`
}

func renderK6(flow []flowRequest) []byte {
	steps := make([]k6Step, 0, len(flow))
	for _, step := range flow {
		steps = append(steps, k6Step{
			Ordinal: step.Ordinal, Method: step.Method, URL: slotKey(step.URL),
			Headers: bindings(step.Headers), Query: bindings(step.Query),
		})
	}
	configuration, _ := json.MarshalIndent(steps, "", "  ")
	return []byte(`import http from 'k6/http';

export const options = { vus: 1, iterations: 1 };

const steps = ` + string(configuration) + `;

function required(name) {
  const value = __ENV[name];
  if (value === undefined) throw new Error('Missing environment variable ' + name);
  return value;
}

function valueOf(binding) {
  return (binding.prefix || '') + (binding.env ? required(binding.env) : '');
}

export default function () {
  const requests = steps.map((step) => {
    let url = required(step.url);
    if (!/^https?:\/\/[^/?#\s]+(?:\/[^?#\s]*)?$/i.test(url) || url.split('/')[2].includes('@')) {
      throw new Error('Expected an HTTP(S) URL without userinfo, query or fragment for request ' + step.ordinal);
    }
    const query = (step.query || []).map((row) => {
      return encodeURIComponent(row.key) + '=' + encodeURIComponent(valueOf(row));
    });
    if (query.length) url += '?' + query.join('&');
    const headers = Object.create(null);
    for (const row of step.headers || []) headers[row.key] = valueOf(row);
    return { method: step.method, url, headers };
  });
  for (const request of requests) http.request(request.method, request.url, null, { headers: request.headers });
}`)
}

func bindings(rows []model.KV) []k6Binding {
	var values []k6Binding
	for _, row := range rows {
		prefix, _, _ := strings.Cut(row.Value, "{{")
		values = append(values, k6Binding{Key: row.Key, Env: slotKey(row.Value), Prefix: prefix})
	}
	return values
}

func slotKey(value string) string {
	_, key, _ := strings.Cut(value, "{{")
	return strings.TrimSuffix(key, "}}")
}
