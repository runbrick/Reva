package importer

import (
	"sort"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/model"
)

const (
	KindNativeCollection   Kind = "reva-collection"
	KindNativeEnvironment  Kind = "reva-environment"
	KindPostmanEnvironment Kind = "postman-environment"
	KindHAR                Kind = "har"
	MaxInputBytes               = 32 << 20
)

// Diagnostic contains only a stable code and a structural path, never source values.
type Diagnostic struct {
	Code string
	Path string
}

// Result contains exactly one payload on success. Warnings require review before saving.
type Result struct {
	Kind        Kind
	Collection  *model.Collection
	Environment *model.Environment
	Request     *model.Request
	Warnings    []Diagnostic
}

// ParseError is safe to log, but applications should localize Code for display.
type ParseError struct {
	Code string
	Path string
}

func (e *ParseError) Error() string { return e.Code + ": " + e.Path }

// Parse converts one bounded input without executing scripts, resolving variables,
// reading referenced files, or accessing services or the network.
func Parse(input string) (result Result, err error) {
	defer func() {
		if p := recover(); p != nil {
			if failure, ok := p.(*ParseError); ok {
				result, err = Result{}, failure
				return
			}
			panic(p)
		}
	}()
	if len(input) > MaxInputBytes {
		fail("input_too_large", "$")
	}
	input = cleanInput(input)
	if !utf8.ValidString(input) {
		fail("invalid_json", "$")
	}
	c := &conversion{}
	if curlCandidate(input) {
		r, e := Curl(input)
		if e != nil {
			fail("invalid_curl", "$")
		}
		c.curlWarnings(input)
		c.checkRequest(&r, "$")
		c.nativeDynamics(&r, "$")
		return Result{Kind: KindCurl, Request: &r, Warnings: c.warnings}, nil
	}
	root := readJSON(input)
	kind := identify(root)
	result.Kind = kind
	switch kind {
	case KindPostman:
		result.Collection = c.postman(root)
	case KindNativeCollection:
		result.Collection = c.nativeCollection(root)
	case KindNativeEnvironment:
		result.Environment = c.nativeEnvironment(root)
	case KindPostmanEnvironment:
		result.Environment = c.environment(root)
	case KindHAR:
		result.Collection = c.har(root)
	default:
		fail("unknown_format", "$")
	}
	sort.Slice(c.warnings, func(i, j int) bool {
		if c.warnings[i].Path == c.warnings[j].Path {
			return c.warnings[i].Code < c.warnings[j].Code
		}
		return c.warnings[i].Path < c.warnings[j].Path
	})
	result.Warnings = c.warnings
	return result, nil
}
