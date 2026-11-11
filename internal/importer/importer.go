// Package importer converts pasted Postman collections and curl commands into
// Reqo models. It never executes commands or reads files named in a command.
package importer

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/runbrick/reqo/internal/model"
)

// Kind identifies a supported import format.
type Kind string

const (
	KindUnknown Kind = "unknown"
	KindPostman Kind = "postman"
	KindCurl    Kind = "curl"
)

// Detect identifies collection-shaped JSON or a command starting with curl.
// Parsing is still required to validate the complete input.
func Detect(input string) Kind {
	input = strings.TrimSpace(strings.TrimPrefix(input, "\ufeff"))
	var col struct {
		Info json.RawMessage `json:"info"`
		Item json.RawMessage `json:"item"`
	}
	if json.Unmarshal([]byte(input), &col) == nil && len(col.Info) > 0 && len(col.Item) > 0 {
		return KindPostman
	}
	words, err := shellWords(input)
	if err == nil && len(words) > 0 && isCurl(words[0]) {
		return KindCurl
	}
	// Recognize incomplete commands too, so the parser can explain the error.
	first, _, _ := strings.Cut(input, " ")
	if isCurl(first) {
		return KindCurl
	}
	return KindUnknown
}

func isCurl(s string) bool {
	return strings.EqualFold(s, "curl") || strings.EqualFold(s, "curl.exe")
}

// splitURL deliberately avoids parsing the whole URL: templates such as
// {{baseUrl}} and {{port}} must survive without validation or escaping.
func splitURL(raw string) (string, []model.KV) {
	base, fragment, hasFragment := strings.Cut(raw, "#")
	base, query, hasQuery := strings.Cut(base, "?")
	if hasFragment {
		base += "#" + fragment
	}
	if !hasQuery {
		return base, nil
	}
	return base, queryRows(query)
}

func queryRows(query string) []model.KV {
	var rows []model.KV
	for _, part := range strings.Split(query, "&") {
		if part == "" {
			continue
		}
		key, value, _ := strings.Cut(part, "=")
		rows = append(rows, model.KV{Key: unescape(key), Value: unescape(value)})
	}
	return rows
}

func unescape(s string) string {
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}
