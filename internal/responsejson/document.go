package responsejson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxBody     = 512 * 1024
	maxDepth    = 64
	maxNodes    = 12000
	maxPointer  = 1024
	maxNumber   = 128
	maxScalar   = 4096
	MaxCaptures = 16
)

var ErrInvalid = errors.New("invalid, ambiguous or oversized JSON")

type Kind string

const (
	Object  Kind = "object"
	Array   Kind = "array"
	String  Kind = "string"
	Number  Kind = "number"
	Boolean Kind = "boolean"
	Null    Kind = "null"
)

type Value struct {
	kind   Kind
	text   string
	object map[string]*Value
	array  []*Value
}

func (value *Value) Kind() Kind { return value.kind }

func (value *Value) Scalar() (string, bool) {
	switch value.kind {
	case String, Number, Boolean, Null:
		return value.text, len(value.text) <= maxScalar
	default:
		return "", false
	}
}

func (value *Value) JSONScalar() (string, bool) {
	text, ok := value.Scalar()
	if ok && value.kind == String {
		return Quote(text), true
	}
	return text, ok
}

func (value *Value) Summary() string {
	const maxRunes = 80
	if value.kind == Array || value.kind == Object {
		return ""
	}
	text := value.text
	index := 0
	for offset := range text {
		if index == maxRunes {
			text = text[:offset] + "…"
			break
		}
		index++
	}
	if value.kind == String {
		return strconv.Quote(text)
	}
	return text
}

func Quote(value string) string {
	quoted, _ := json.Marshal(value)
	return string(quoted)
}

type Document struct{ root *Value }

func Parse(body []byte) (*Document, error) {
	if len(body) > maxBody || !utf8.Valid(body) {
		return nil, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	remaining := maxNodes
	root, err := readValue(decoder, 0, 0, &remaining)
	if err != nil {
		return nil, ErrInvalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return &Document{root: root}, nil
}

func readValue(decoder *json.Decoder, depth, pointerBytes int, remaining *int) (*Value, error) {
	if depth > maxDepth || pointerBytes > maxPointer || *remaining == 0 {
		return nil, ErrInvalid
	}
	*remaining--
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch token := token.(type) {
	case json.Delim:
		switch token {
		case '{':
			value := &Value{kind: Object, object: make(map[string]*Value)}
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				if err != nil || !ok || value.object[name] != nil {
					return nil, ErrInvalid
				}
				segmentBytes := len(name) + strings.Count(name, "~") + strings.Count(name, "/")
				child, err := readValue(decoder, depth+1, pointerBytes+1+segmentBytes, remaining)
				if err != nil {
					return nil, err
				}
				value.object[name] = child
			}
			_, err := decoder.Token()
			return value, err
		case '[':
			value := &Value{kind: Array}
			for decoder.More() {
				child, err := readValue(decoder, depth+1, pointerBytes+1+len(strconv.Itoa(len(value.array))), remaining)
				if err != nil {
					return nil, err
				}
				value.array = append(value.array, child)
			}
			_, err := decoder.Token()
			return value, err
		}
	case string:
		return &Value{kind: String, text: token}, nil
	case json.Number:
		if len(token) <= maxNumber {
			return &Value{kind: Number, text: string(token)}, nil
		}
	case bool:
		return &Value{kind: Boolean, text: strconv.FormatBool(token)}, nil
	case nil:
		return &Value{kind: Null, text: "null"}, nil
	}
	return nil, ErrInvalid
}

var encodeSegment = strings.NewReplacer("~", "~0", "/", "~1")
var decodeSegment = strings.NewReplacer("~1", "/", "~0", "~")

func pointerSegments(pointer string) ([]string, bool) {
	if pointer == "" {
		return nil, true
	}
	if len(pointer) > maxPointer || pointer[0] != '/' {
		return nil, false
	}
	parts := strings.Split(pointer[1:], "/")
	for index, part := range parts {
		for offset := 0; offset < len(part); offset++ {
			if part[offset] == '~' {
				offset++
				if offset == len(part) || part[offset] != '0' && part[offset] != '1' {
					return nil, false
				}
			}
		}
		parts[index] = decodeSegment.Replace(part)
	}
	return parts, true
}

func (doc *Document) Lookup(pointer string) *Value {
	if doc == nil {
		return nil
	}
	parts, ok := pointerSegments(pointer)
	if !ok {
		return nil
	}
	value := doc.root
	for _, part := range parts {
		switch value.kind {
		case Object:
			value = value.object[part]
		case Array:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(value.array) || strconv.Itoa(index) != part {
				return nil
			}
			value = value.array[index]
		default:
			return nil
		}
		if value == nil {
			return nil
		}
	}
	return value
}

type Field struct {
	Pointer string
	Value   *Value
}

func (doc *Document) Fields(query string, limit int) (fields []Field, more bool) {
	query = strings.ToLower(query)
	var visit func(string, *Value) bool
	visit = func(path string, value *Value) bool {
		if strings.Contains(strings.ToLower(path), query) {
			if len(fields) == limit {
				return false
			}
			fields = append(fields, Field{Pointer: path, Value: value})
		}
		switch value.kind {
		case Object:
			for _, key := range keys(value.object, nil) {
				if !visit(path+"/"+encodeSegment.Replace(key), value.object[key]) {
					return false
				}
			}
		case Array:
			for index, item := range value.array {
				if !visit(path+"/"+strconv.Itoa(index), item) {
					return false
				}
			}
		}
		return true
	}
	more = !visit("", doc.root)
	return fields, more
}

func keys(before, after map[string]*Value) []string {
	out := make([]string, 0, len(before)+len(after))
	for key := range before {
		out = append(out, key)
	}
	for key := range after {
		if before[key] == nil {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}

func (value *Value) EqualsJSONScalar(expected string) bool {
	if len(expected) > maxScalar*6+2 {
		return false
	}
	if _, ok := value.Scalar(); !ok {
		return false
	}
	other, err := Parse([]byte(expected))
	return err == nil && value.kind == other.root.kind && equalScalar(value, other.root)
}

func equalScalar(before, after *Value) bool {
	if before.text == after.text {
		return true
	}
	return before.kind == Number && canonicalNumber(before.text) == canonicalNumber(after.text)
}

func canonicalNumber(number string) string {
	mantissa, exponentText, hasExponent := strings.Cut(strings.ToLower(number), "e")
	var exponent big.Int
	if hasExponent {
		exponent.SetString(exponentText, 10)
	}
	negative := strings.HasPrefix(mantissa, "-")
	integer, fraction, _ := strings.Cut(strings.TrimPrefix(mantissa, "-"), ".")
	digits := strings.TrimLeft(integer+fraction, "0")
	if digits == "" {
		return "0"
	}
	significant := strings.TrimRight(digits, "0")
	exponent.Add(&exponent, big.NewInt(int64(len(digits)-len(significant)-len(fraction))))
	if negative {
		significant = "-" + significant
	}
	return significant + "e" + exponent.String()
}

func ValidVariable(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for index, char := range name {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char == '_' ||
			index > 0 && (char >= '0' && char <= '9' || char == '-' || char == '.') {
			continue
		}
		return false
	}
	return true
}
