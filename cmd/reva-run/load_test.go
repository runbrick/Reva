package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
)

func TestCLIStrictJSONRejectsAmbiguousMalformedAndFutureData(t *testing.T) {
	for _, tc := range []struct{ name, settings string }{
		{"syntax", `{"version":1, "` + secret + `":`},
		{"unknown-field", `{"version":1,"` + secret + `":"credential"}`},
		{"duplicate-field", `{"version":1,"insecureTls":false,"insecureTls":true}`},
		{"escaped-duplicate", `{"version":1,"insecureTls":false,"insecure\u0054ls":true}`},
		{"case-alias", `{"version":1,"InsecureTLS":true}`},
		{"null-scalar", `{"version":1,"insecureTls":null}`},
		{"null-root", `null`},
		{"multiple-objects", `{"version":1} {"version":1}`},
		{"array-root", `[]`},
		{"negative-timeout", `{"version":1,"requestTimeoutSec":-1}`},
		{"overflow-timeout", `{"version":1,"requestTimeoutSec":9223372036854775}`},
		{"wrong-type", `{"version":"` + secret + `"}`},
		{"invalid-utf8", "{\"version\":1,\"language\":\"\xff\"}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCollection(t, dir, testCollection())
			if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(tc.settings), 0600); err != nil {
				t.Fatal("write invalid fixture failed")
			}
			before := snapshot(t, dir)
			code, out, stderr := invoke(t, context.Background(), dir)
			if code != exitInput || out != "" || stderr != "reva-run: settings_invalid_or_unreadable\n" {
				t.Fatal("ambiguous settings were accepted or diagnosed unsafely")
			}
			if !reflect.DeepEqual(before, snapshot(t, dir)) {
				t.Fatal("invalid JSON handling mutated data")
			}
		})
	}
}

func TestCLIBoundedNativeInput(t *testing.T) {
	t.Run("file-size", func(t *testing.T) {
		dir := t.TempDir()
		writeCollection(t, dir, testCollection())
		path := filepath.Join(dir, "settings.json")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = file.Truncate((32 << 20) + 1)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		code, out, _ := invoke(t, context.Background(), dir)
		if code != exitInput || out != "" {
			t.Fatal("oversized native file accepted")
		}
	})
	for _, tc := range []struct {
		name string
		col  func() model.Collection
	}{
		{"tree-depth", func() model.Collection {
			col := testCollection()
			for range 257 {
				col.Items = []model.Item{{Type: model.ItemFolder, ID: strings.Repeat("f", len(col.Name)), Name: "folder", Items: col.Items}}
				col.Name += "x"
			}
			return col
		}},
		{"json-depth", func() model.Collection {
			col := testCollection()
			for range 600 {
				col.Items = []model.Item{{Type: model.ItemFolder, ID: col.Name, Name: "folder", Items: col.Items}}
				col.Name += "x"
			}
			return col
		}},
		{"item-count", func() model.Collection {
			col := testCollection()
			for i := range 10001 {
				col.Items = append(col.Items, model.Item{ID: string(rune(0x1000 + i)), Name: "folder", Type: model.ItemFolder})
			}
			return col
		}},
		{"row-count", func() model.Collection {
			col := testCollection()
			col.Variables = make([]model.KV, 100001)
			return col
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCollection(t, dir, tc.col())
			code, out, _ := invoke(t, context.Background(), dir)
			if code != exitInput || out != "" {
				t.Fatal("native data exceeded documented limits")
			}
		})
	}
}

func TestCLIStrictNestedSchemaAndNonregularFiles(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement string }{
		{"request-missing-body", `"body":{"mode":"none"},`, ``},
		{"body-missing-mode", `"body":{"mode":"none"}`, `"body":{}`},
		{"missing-items", `"items":`, `"unused":`},
		{"request-unknown", `"url":`, `"` + secret + `":"credential","url":`},
		{"body-null", `"body":{"mode":"none"}`, `"body":null`},
		{"auth-null", `"auth":{"type":"none"}`, `"auth":null`},
		{"headers-null-entry", `"body":`, `"headers":[null],"body":`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			col := testCollection(testItem("one", "http://127.0.0.1:1"))
			data, err := json.Marshal(col)
			if err != nil {
				t.Fatal(err)
			}
			changed := strings.Replace(string(data), tc.old, tc.replacement, 1)
			if changed == string(data) {
				t.Fatal("invalid fixture was not changed")
			}
			writeCollection(t, dir, col)
			if err := os.WriteFile(filepath.Join(dir, "collections/collection.json"), []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
			code, out, _ := invoke(t, context.Background(), dir)
			if code != exitInput || out != "" {
				t.Fatal("invalid nested native data accepted")
			}
		})
	}
	t.Run("directory-instead-of-settings", func(t *testing.T) {
		dir := t.TempDir()
		writeCollection(t, dir, testCollection())
		if err := os.Mkdir(filepath.Join(dir, "settings.json"), 0700); err != nil {
			t.Fatal(err)
		}
		code, out, _ := invoke(t, context.Background(), dir)
		if code != exitInput || out != "" {
			t.Fatal("nonregular input file accepted")
		}
	})
}
