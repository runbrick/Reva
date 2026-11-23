package responsejson

import (
	"errors"
	"strings"
	"testing"
)

func TestFilterJSONPathSelectors(t *testing.T) {
	body := []byte(`{"z":null,"orders":[{"id":9007199254740993},{"id":"A-2"},{"id":false},{"id":null},{}],"a":{"店.铺":[1.230e+40],"x\"]":true,"":[2]}}`)
	for _, test := range []struct {
		name, query, json string
		matches           int
	}{
		{"root", " $ ", `[{"a":{"":[2],"x\"]":true,"店.铺":[1.230e+40]},"orders":[{"id":9007199254740993},{"id":"A-2"},{"id":false},{"id":null},{}],"z":null}]`, 1},
		{"order IDs", `$.orders[*].id`, `[9007199254740993,"A-2",false,null]`, 4},
		{"array order with dot wildcard", `$.orders.*.id`, `[9007199254740993,"A-2",false,null]`, 4},
		{"index", `$["orders"][1]`, `[{"id":"A-2"}]`, 1},
		{"missing index", `$.orders[5]`, `[]`, 0},
		{"very large index", `$.orders[999999999999999999999999]`, `[]`, 0},
		{"missing field", `$.orders[*].missing`, `[]`, 0},
		{"wildcard on scalar", `$.z[*]`, `[]`, 0},
		{"field on array", `$.orders["0"]`, `[]`, 0},
		{"index on object", `$.a[0]`, `[]`, 0},
		{"object wildcard order", `$.a.*`, `[[2],true,[1.230e+40]]`, 3},
		{"non-ASCII key", `$.a["店.铺"][0]`, `[1.230e+40]`, 1},
		{"escaped key", `$.a["x\"]"]`, `[true]`, 1},
		{"empty key", `$.a[""]`, `[[2]]`, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := Filter(body, test.query)
			if err != nil || result.JSON != test.json || result.Matches != test.matches {
				t.Fatalf("Filter(%q) = %+v, %v; want %s (%d matches)", test.query, result, err, test.json, test.matches)
			}
		})
	}
	result, err := Filter([]byte(`null`), `$`)
	if err != nil || result != (FilterResult{JSON: `[null]`, Matches: 1}) {
		t.Fatalf("root null: %+v, %v", result, err)
	}
}

func TestFilterJSONPathRejectsUnsupportedSyntax(t *testing.T) {
	for _, query := range []string{
		``, `orders`, `$.`, `$..id`, `$.orders[-1]`, `$.orders[01]`, `$.orders[+1]`, `$.orders[ 0 ]`,
		`$.orders[0:1]`, `$.orders[0,1]`, `$.orders[?(@.id)]`, `$['orders']`, `$["bad\q"]`, `$["no end]`, `$["no bracket"`,
		`$.9name`, `$.店`, `$.orders [0]`, `$.orders[*]invalid`, "$[\"\xff\"]",
	} {
		if result, err := Filter([]byte(`{}`), query); !errors.Is(err, ErrFilterSyntax) || result != (FilterResult{}) {
			t.Errorf("query %q returned %+v, %v; want syntax error without result", query, result, err)
		}
	}
}

func TestFilterJSONPathLimitsAndInvalidBody(t *testing.T) {
	for _, test := range []struct {
		name, body, query string
		want              error
	}{
		{"query bytes", `{}`, "$" + strings.Repeat(" ", FilterMaxQueryBytes), ErrFilterQueryLimit},
		{"query segments", `{}`, "$" + strings.Repeat(".a", FilterMaxSegments+1), ErrFilterQueryLimit},
		{"body bytes", strings.Repeat(" ", FilterMaxBodyBytes) + `0`, `$`, ErrFilterInputLimit},
		{"body depth", strings.Repeat("[", maxDepth+1) + `0` + strings.Repeat("]", maxDepth+1), `$`, ErrFilterInputLimit},
		{"body nodes", "[" + strings.Repeat("0,", maxNodes-1) + "0]", `$`, ErrFilterInputLimit},
		{"body pointer bytes", `{"` + strings.Repeat("x", maxPointer) + `":0}`, `$`, ErrFilterInputLimit},
		{"body number bytes", strings.Repeat("1", maxNumber+1), `$`, ErrFilterInputLimit},
		{"result matches", "[" + strings.Repeat("0,", FilterMaxMatches) + "0]", `$[*]`, ErrFilterResultLimit},
		{"result bytes", `"` + strings.Repeat("x", FilterMaxResultBytes-3) + `"`, `$`, ErrFilterResultLimit},
		{"result escaped bytes", `"` + strings.Repeat("<", FilterMaxResultBytes/6+1) + `"`, `$`, ErrFilterResultLimit},
		{"syntax", `{"id":`, `$`, ErrInvalid},
		{"trailing value", `[] {}`, `$`, ErrInvalid},
		{"duplicate keys", `{"id":1,"\u0069d":2}`, `$`, ErrInvalid},
		{"invalid UTF-8", "\"\xff\"", `$`, ErrInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			if result, err := Filter([]byte(test.body), test.query); !errors.Is(err, test.want) || result != (FilterResult{}) {
				t.Fatalf("Filter returned %d bytes and %d matches with error %v; want %v without result", len(result.JSON), result.Matches, err, test.want)
			}
		})
	}
	result, err := Filter([]byte(`"`+strings.Repeat("x", FilterMaxResultBytes-4)+`"`), `$`)
	if err != nil || len(result.JSON) != FilterMaxResultBytes || result.Matches != 1 {
		t.Fatalf("result at byte limit: %d bytes, %d matches, %v", len(result.JSON), result.Matches, err)
	}
	result, err = Filter([]byte("["+strings.Repeat("0,", FilterMaxMatches-1)+"0]"), `$[*]`)
	if err != nil || result.Matches != FilterMaxMatches {
		t.Fatalf("result at match limit: %d matches, %v", result.Matches, err)
	}
	result, err = Filter([]byte(strings.Repeat(" ", FilterMaxBodyBytes-1)+`0`), strings.Repeat(" ", FilterMaxQueryBytes-1)+`$`)
	if err != nil || result != (FilterResult{JSON: `[0]`, Matches: 1}) {
		t.Fatalf("input and query at byte limits: %+v, %v", result, err)
	}
}
