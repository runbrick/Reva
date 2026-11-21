package responsejson

import (
	"slices"
	"strings"
	"testing"
)

func parseFixture(test *testing.T, body string) *Document {
	test.Helper()
	doc, err := Parse([]byte(body))
	if err != nil {
		test.Fatal(err)
	}
	return doc
}

func TestCompareResponseJSON(t *testing.T) {
	before := parseFixture(t, `{"amount":1,"id":9007199254740992,"nested":{"timestamp":"before"},"items":[{"price":5,"state":"ready","valid":true}]}`)
	after := parseFixture(t, `{"items":[{"valid":"true","state":"ready","count":0}],"nested":{"timestamp":"after"},"id":9007199254740993,"amount":100e-2}`)
	ignored, err := IgnoredPaths("/nested/timestamp\r\n")
	if err != nil {
		t.Fatal(err)
	}
	type difference struct {
		path string
		kind ChangeKind
	}
	changes, more := Diff(before, after, ignored, 100)
	var actual []difference
	for _, change := range changes {
		actual = append(actual, difference{change.Pointer, change.Kind})
	}
	want := []difference{{"/id", ValueChanged}, {"/items/0/count", Added}, {"/items/0/price", Removed}, {"/items/0/valid", TypeChanged}}
	if more || !slices.Equal(actual, want) {
		t.Fatalf("got %v (more %v), want %v", actual, more, want)
	}
	if changes, more := Diff(before, after, ignored, 2); len(changes) != 2 || !more {
		t.Fatal("difference limit was not applied")
	}
	if _, err := IgnoredPaths("/invalid~2escape"); err == nil {
		t.Fatal("invalid ignore path accepted")
	}
}

func TestResponseJSONPointersAndBounds(t *testing.T) {
	doc := parseFixture(t, `{"a/~key":[null,false,9.007199254740993e15],"empty":"","__proto__":8}`)
	if null := doc.Lookup("/a~1~0key/0"); null == nil || null.Kind() != Null || doc.Lookup("/a~1~0key/00") != nil || doc.Lookup("/missing") != nil {
		t.Fatal("pointers confused missing values, null, or array index syntax")
	}
	if value := doc.Lookup("/a~1~0key/2"); !value.EqualsJSONScalar("9007199254740993") || value.EqualsJSONScalar("9007199254740992") {
		t.Fatal("numeric comparison lost precision")
	}
	if value, ok := doc.Lookup("/empty").Scalar(); value != "" || !ok {
		t.Fatal("empty strings must be capturable")
	}
	if fields, more := doc.Fields("a~1~0key", 2); len(fields) != 2 || fields[1].Pointer != "/a~1~0key/0" || !more {
		t.Fatalf("fields = %+v, more = %v", fields, more)
	}
	for _, body := range []string{
		`{"id":1,"\u0069d":2}`, `[1] true`, `{"x":`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66),
		Quote(strings.Repeat("x", maxBody)), `{"` + strings.Repeat("x", maxPointer) + `":0}`,
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Fatal("invalid, ambiguous or oversized JSON was accepted")
		}
	}
}
