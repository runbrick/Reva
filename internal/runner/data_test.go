package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
)

func TestReadIterationDataScalarSemantics(t *testing.T) {
	for _, fixture := range []struct {
		name, contents string
		want           [][]model.KV
	}{
		{"rows.CSV", "\xef\xbb\xbfactive,detail\r\ntrue,\"two\r\nlines\"\r\nfalse,\r\n", [][]model.KV{
			{{Key: "active", Value: "true"}, {Key: "detail", Value: "two\nlines"}},
			{{Key: "active", Value: "false"}, {Key: "detail", Value: ""}},
		}},
		{"rows.JSON", "\xef\xbb\xbf" + `[{"amount":9007199254740993,"active":false},{"text":"","amount":1.20e+9,"active":true}]`, [][]model.KV{
			{{Key: "amount", Value: "9007199254740993"}, {Key: "active", Value: "false"}},
			{{Key: "text", Value: ""}, {Key: "amount", Value: "1.20e+9"}, {Key: "active", Value: "true"}},
		}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			data, err := ReadDataFile(iterationFixture(t, fixture.name, fixture.contents))
			if err != nil || !reflect.DeepEqual(data.rows, fixture.want) {
				t.Fatalf("incorrect parsed rows: %+v, %v", data.rows, err)
			}
		})
	}
}

func TestReadIterationDataRejectsInvalidRecords(t *testing.T) {
	for _, fixture := range []struct{ name, extension, contents string }{
		{"extension", ".txt", `[{"field":1}]`},
		{"empty-json", ".json", `[]`},
		{"empty-json-row", ".json", `[{}]`},
		{"json-null", ".json", `[{"field":null}]`},
		{"json-trailing", ".json", `[{"field":1}] []`},
		{"json-nesting", ".json", `[{"field":[1]}]`},
		{"csv-utf8", ".csv", "field\n\xff\n"},
		{"csv-row-width", ".csv", "first,second\n1,2\n3\n"},
		{"csv-empty", ".csv", "field\n"},
		{"csv-key-whitespace", ".csv", " field\nrow\n"},
		{"csv-key-size", ".csv", strings.Repeat("k", maxIterationKeyBytes+1) + "\nrow\n"},
		{"csv-value-size", ".csv", "field\n" + strings.Repeat("v", maxIterationValBytes+1) + "\n"},
		{"too-many-fields", ".csv", strings.Repeat("field,", maxIterationFields) + "last\nrow\n"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if data, err := ReadDataFile(iterationFixture(t, fixture.name+fixture.extension, fixture.contents)); err != ErrInvalidDataFile || len(data.rows) != 0 {
				t.Fatalf("invalid rows accepted or diagnostic changed: %+v, %v", data, err)
			}
		})
	}
	if _, err := ReadDataFile(t.TempDir() + ".csv"); err != ErrInvalidDataFile {
		t.Fatal("missing file did not return the fixed diagnostic")
	}
}

func iterationFixture(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
