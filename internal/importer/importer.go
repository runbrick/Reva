// Package importer converts native Reqo, Postman-compatible, environment, HAR,
// and curl inputs into Reqo models without executing commands or reading files.
package importer

import (
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

// Detect identifies a single supported format, returning Unknown for ambiguous input.
// Parsing is still required to validate the complete input.
func Detect(input string) (kind Kind) {
	if len(input) > MaxInputBytes {
		return KindUnknown
	}
	input = cleanInput(input)
	if curlCandidate(input) {
		return KindCurl
	}
	defer func() {
		if p := recover(); p != nil {
			if _, ok := p.(*ParseError); ok {
				kind = KindUnknown
			} else {
				panic(p)
			}
		}
	}()
	return identify(readJSON(input))
}

func curlCandidate(input string) bool {
	if !strings.HasPrefix(strings.ToLower(input), "curl") && !strings.HasPrefix(input, "\"curl") && !strings.HasPrefix(input, "'curl") {
		return false
	}
	words, err := shellWords(input)
	if err == nil && len(words) > 0 && isCurl(words[0]) {
		return true
	}
	// Recognize incomplete commands too, so the parser can explain the error.
	first := strings.Fields(input)
	return len(first) > 0 && isCurl(first[0])
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
