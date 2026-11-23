package responsejson

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	FilterMaxBodyBytes   = maxBody
	FilterMaxQueryBytes  = 1024
	FilterMaxSegments    = 32
	FilterMaxMatches     = 256
	FilterMaxResultBytes = 64 * 1024
)

var (
	ErrFilterSyntax      = errors.New("unsupported JSONPath syntax")
	ErrFilterQueryLimit  = errors.New("JSONPath query limit exceeded")
	ErrFilterInputLimit  = errors.New("JSONPath input limit exceeded")
	ErrFilterResultLimit = errors.New("JSONPath result limit exceeded")
)

type FilterResult struct {
	JSON    string
	Matches int
}

func Filter(body []byte, query string) (FilterResult, error) {
	segments, err := filterSegments(query)
	if err != nil {
		return FilterResult{}, err
	}
	doc, err := parseDocument(body)
	if err == errDocumentLimit {
		return FilterResult{}, ErrFilterInputLimit
	}
	if err != nil {
		return FilterResult{}, err
	}
	var output filterOutput
	output.write("[")
	if !output.selectValues(doc.root, segments) || !output.write("]") {
		return FilterResult{}, ErrFilterResultLimit
	}
	return FilterResult{JSON: output.json.String(), Matches: output.matches}, nil
}

type filterSelector uint8

const (
	filterField filterSelector = iota
	filterIndex
	filterWildcard
)

type filterSegment struct {
	selector filterSelector
	field    string
	index    int
}

func filterSegments(query string) ([]filterSegment, error) {
	if len(query) > FilterMaxQueryBytes {
		return nil, ErrFilterQueryLimit
	}
	if !utf8.ValidString(query) {
		return nil, ErrFilterSyntax
	}
	path, ok := strings.CutPrefix(strings.TrimSpace(query), "$")
	if !ok {
		return nil, ErrFilterSyntax
	}
	var segments []filterSegment
	for path != "" {
		if len(segments) == FilterMaxSegments {
			return nil, ErrFilterQueryLimit
		}
		var segment filterSegment
		var consumed int
		switch path[0] {
		case '.':
			segment, consumed = filterDotSegment(path[1:])
		case '[':
			segment, consumed = filterBracketSegment(path[1:])
		}
		if consumed == 0 {
			return nil, ErrFilterSyntax
		}
		segments = append(segments, segment)
		path = path[1+consumed:]
	}
	return segments, nil
}

func filterDotSegment(path string) (filterSegment, int) {
	if strings.HasPrefix(path, "*") {
		return filterSegment{selector: filterWildcard}, 1
	}
	if path == "" || !filterNameStart(path[0]) {
		return filterSegment{}, 0
	}
	end := 1
	for end < len(path) && (filterNameStart(path[end]) || filterDigit(path[end])) {
		end++
	}
	return filterSegment{selector: filterField, field: path[:end]}, end
}

func filterNameStart(char byte) bool {
	return char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

func filterDigit(char byte) bool { return char >= '0' && char <= '9' }

func filterBracketSegment(path string) (filterSegment, int) {
	if strings.HasPrefix(path, "*]") {
		return filterSegment{selector: filterWildcard}, 2
	}
	if strings.HasPrefix(path, `"`) {
		var field string
		decoder := json.NewDecoder(strings.NewReader(path))
		if decoder.Decode(&field) == nil {
			end := int(decoder.InputOffset())
			if end < len(path) && path[end] == ']' {
				return filterSegment{selector: filterField, field: field}, end + 1
			}
		}
		return filterSegment{}, 0
	}
	end := 0
	for end < len(path) && filterDigit(path[end]) {
		end++
	}
	if end == 0 || end == len(path) || path[end] != ']' || end > 1 && path[0] == '0' {
		return filterSegment{}, 0
	}
	index, err := strconv.Atoi(path[:end])
	if err != nil {
		index = maxNodes
	}
	return filterSegment{selector: filterIndex, index: index}, end + 1
}

type filterOutput struct {
	json    strings.Builder
	matches int
}

func (out *filterOutput) selectValues(value *Value, segments []filterSegment) bool {
	if len(segments) == 0 {
		if out.matches == FilterMaxMatches || out.matches > 0 && !out.write(",") || !out.writeValue(value) {
			return false
		}
		out.matches++
		return true
	}
	segment, rest := segments[0], segments[1:]
	switch segment.selector {
	case filterField:
		if child := value.object[segment.field]; child != nil {
			return out.selectValues(child, rest)
		}
	case filterIndex:
		if segment.index < len(value.array) {
			return out.selectValues(value.array[segment.index], rest)
		}
	case filterWildcard:
		for _, child := range value.array {
			if !out.selectValues(child, rest) {
				return false
			}
		}
		for _, key := range keys(value.object, nil) {
			if !out.selectValues(value.object[key], rest) {
				return false
			}
		}
	}
	return true
}

func (out *filterOutput) write(value string) bool {
	if len(value) > FilterMaxResultBytes-out.json.Len() {
		return false
	}
	out.json.WriteString(value)
	return true
}

func (out *filterOutput) writeString(value string) bool {
	return len(value)+2 <= FilterMaxResultBytes-out.json.Len() && out.write(Quote(value))
}

func (out *filterOutput) writeValue(value *Value) bool {
	switch value.kind {
	case String:
		return out.writeString(value.text)
	case Object:
		if !out.write("{") {
			return false
		}
		for index, key := range keys(value.object, nil) {
			if (index > 0 && !out.write(",")) || !out.writeString(key) || !out.write(":") || !out.writeValue(value.object[key]) {
				return false
			}
		}
		return out.write("}")
	case Array:
		if !out.write("[") {
			return false
		}
		for index, child := range value.array {
			if (index > 0 && !out.write(",")) || !out.writeValue(child) {
				return false
			}
		}
		return out.write("]")
	default:
		return out.write(value.text)
	}
}
