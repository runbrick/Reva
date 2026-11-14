package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestImportCopyCoversDiagnosticsAndCompatibilityRoutes(t *testing.T) {
	errors := map[string]string{
		"invalid_json": "InvalidJSON", "unknown_format": "Unknown", "unsupported_format": "UnsupportedFormat",
		"unsupported_version": "UnsupportedVersion", "ambiguous_format": "AmbiguousFormat", "input_too_large": "InputTooLarge",
		"limit_exceeded": "LimitExceeded", "invalid_structure": "InvalidStructure", "invalid_auth": "InvalidAuth",
		"unsupported_auth": "UnsupportedAuth", "unsupported_body": "UnsupportedBody", "invalid_curl": "InvalidCurl",
		"invalid_source_url": "InvalidSourceURL", "fetch_failed": "FetchFailed",
		"unsupported_reference": "UnsupportedReference", "unsupported_parameter": "UnsupportedParameter",
	}
	warnings := map[string]string{
		"scripts_omitted": "ScriptsOmitted", "metadata_omitted": "MetadataOmitted", "examples_omitted": "ExamplesOmitted",
		"path_variables_unresolved": "PathVariablesUnresolved", "dynamic_variables_unsupported": "DynamicVariablesUnsupported",
		"local_file_references": "LocalFileReferences", "form_metadata_omitted": "FormMetadataOmitted", "transport_flags_ignored": "TransportFlagsIgnored",
		"har_response_omitted": "HARResponseOmitted", "har_cookies_omitted": "HARCookiesOmitted",
		"openapi_review": "OpenAPIReview", "openapi_selection": "OpenAPISelection",
	}
	// Discover emitted codes as well as checking the frozen contract: a new
	// parser code must never silently use the generic untranslated fallback.
	files, err := filepath.Glob("../importer/*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			name := ""
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				name = fun.Name
			case *ast.SelectorExpr:
				name = fun.Sel.Name
			}
			if name != "fail" && name != "warn" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				t.Errorf("nonliteral diagnostic in %s requires explicit copy audit", filepath.Base(path))
				return true
			}
			code, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			known := errors
			if name == "warn" {
				known = warnings
			}
			if known[code] == "" {
				t.Errorf("parser emits unmapped %s diagnostic %q", name, code)
			}
			return true
		})
	}
	for _, lang := range []Lang{Zh, En} {
		t.Run(string(lang), func(t *testing.T) {
			copy := For(lang).Import
			value := reflect.ValueOf(copy)
			for i := 0; i < value.NumField(); i++ {
				if strings.TrimSpace(value.Field(i).String()) == "" {
					t.Errorf("empty import copy: %s", value.Type().Field(i).Name)
				}
			}
			for code, field := range errors {
				if got := copy.ErrorMessage(code); got != value.FieldByName(field).String() || got == code {
					t.Errorf("error %s has no dedicated localized message", code)
				}
			}
			for code, field := range warnings {
				if got := copy.WarningMessage(code); got != value.FieldByName(field).String() || got == code {
					t.Errorf("warning %s has no dedicated localized message", code)
				}
			}
			for _, marker := range []string{"Reva v1", "Postman", "v2.0/v2.1", "HAR 1.2", "curl", "32 MiB", "Apifox", "ApiPost", "Postman 2.1"} {
				if !strings.Contains(copy.Help, marker) {
					t.Errorf("help omits bounded support/route %q", marker)
				}
			}
			for _, field := range []string{copy.NativeCollection, copy.NativeEnvironment, copy.PostmanCollection, copy.PostmanEnvironment} {
				if strings.Contains(field, "Apifox") || strings.Contains(field, "ApiPost") {
					t.Error("format name implies proprietary native vendor support")
				}
			}
			unsupported := "proprietary project formats are unsupported"
			if lang == Zh {
				unsupported = "不支持这两者的专有项目格式"
			}
			if !strings.Contains(copy.Help, unsupported) {
				t.Error("help does not distinguish native support from the Postman export route")
			}
			if copy.ErrorMessage("source-secret-unknown") != copy.Unknown || strings.Contains(copy.WarningMessage("source-secret-unknown"), "source-secret") {
				t.Error("unknown-code fallback exposes a source value")
			}
		})
	}
	for code := range errors {
		if For(Zh).Import.ErrorMessage(code) == For(En).Import.ErrorMessage(code) {
			t.Errorf("error %s lacks distinct zh/en copy", code)
		}
	}
	for code := range warnings {
		if For(Zh).Import.WarningMessage(code) == For(En).Import.WarningMessage(code) {
			t.Errorf("warning %s lacks distinct zh/en copy", code)
		}
	}
}
