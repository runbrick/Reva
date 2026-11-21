package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
)

func TestCLIFolderWorkflow(t *testing.T) {
	seen := make(chan string, 12)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		seen <- req.URL.Path + ":" + req.Header.Get("Authorization")
	}))
	defer server.Close()
	item := func(id string) model.Item {
		entry := testItem(id, server.URL+"/"+id)
		entry.Request.Auth.Type = model.AuthInherit
		return entry
	}
	folder := func(id string, cfg model.Auth, children ...model.Item) model.Item {
		return model.Item{ID: id, Name: id, Type: model.ItemFolder, Auth: cfg, Items: children}
	}
	collection := testCollection(item("outside"), folder("parent", model.Auth{Type: model.AuthBearer, Token: "ancestor"},
		item("sibling"), folder("target", model.Auth{}, item("first"),
			folder("barrier", model.Auth{Type: model.AuthNone}, item("second")), item("third"))), folder("empty", model.Auth{}))
	collection.Auth = model.Auth{Type: model.AuthBearer, Token: "root"}
	dir := t.TempDir()
	writeCollection(t, dir, collection)
	code, out, stderr := invoke(t, context.Background(), dir, "--folder", "target")
	if code != exitOK || stderr != "" {
		t.Fatalf("scope run failed: %d %q", code, stderr)
	}
	if report := decodeReport(t, out); report.Counts != (runner.Counts{Total: 3, Passed: 3}) || report.Requests[2].Ordinal != 3 {
		t.Fatalf("unexpected scope report: %+v", report)
	}
	if actual := workflowTraffic(seen); !reflect.DeepEqual(actual, []string{"/first:Bearer ancestor", "/second:", "/third:Bearer ancestor"}) {
		t.Fatalf("scope or inherited auth incorrect: %v", actual)
	}
	code, out, stderr = invoke(t, context.Background(), dir, "--folder", "empty")
	if code != exitOK || stderr != "" || decodeReport(t, out).Counts.Total != 0 {
		t.Fatal("empty folder should be a successful empty run")
	}
	for _, id := range []string{"missing", "first", "Target", "../" + secret, ""} {
		if code, out, stderr := invoke(t, context.Background(), dir, "--folder", id); code != exitInput || out != "" || stderr == "" || len(seen) != 0 {
			t.Fatalf("unexpected invalid scope behavior: %d %q %q", code, out, stderr)
		}
	}
}

func TestCLIDataWorkflow(t *testing.T) {
	seen := make(chan string, 20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen <- fmt.Sprintf("%s:%s:%s:%s:%s", req.URL.Path, req.Header.Get("X-Token"), req.Header.Get("X-Only-Env"), req.Header.Get("X-Only-Collection"), req.Header.Get("X-Local"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"captured"}`)
	}))
	defer server.Close()
	first, second := testItem("capture", server.URL+"/first/{{key}}"), testItem("consume", server.URL+"/second/{{key}}")
	first.Request.PreScript = `pm.variables.set("local", "request-only");`
	first.Request.TestScript = `pm.test("capture", function(){ pm.expect(pm.response.captureJSON("/id", "token")).to.equal(true); });`
	for _, item := range []model.Item{first, second} {
		item.Request.Headers = []model.KV{{Key: "X-Token", Value: "{{token}}"}, {Key: "X-Only-Env", Value: "{{envOnly}}"}, {Key: "X-Only-Collection", Value: "{{colOnly}}"}, {Key: "X-Local", Value: "{{local}}"}}
	}
	col := testCollection(first, second)
	col.Variables = []model.KV{{Key: "token", Value: "collection"}, {Key: "envOnly", Value: "wrong"}, {Key: "colOnly", Value: "collection-fallback"}, {Key: "local", Value: "base"}}
	for _, fixture := range []struct{ name, input string }{
		{"rows.csv", "key,token\r\nfirst,one\r\nsecond,\"two,two\"\r\n"},
		{"rows.json", `[{"key":"first","token":"one"},{"token":"two,two","key":"second"}]`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCollection(t, dir, col)
			writeJSON(t, dir, "environments/env.json", model.Environment{Version: model.SchemaVersion, ID: "env", Name: "Env", Variables: []model.KV{
				{Key: "token", Value: "environment"}, {Key: "envOnly", Value: "environment-fallback"}, {Key: "colOnly", Value: "wrong", Disabled: true},
			}})
			path := workflowFile(t, dir, fixture.name, fixture.input)
			before := snapshot(t, dir)
			code, out, stderr := invoke(t, context.Background(), dir, "--environment", "env", "--data-file", path)
			if code != exitOK || stderr != "" || strings.Contains(out, "captured") || strings.Contains(out, fixture.name) {
				t.Fatalf("data run failed or leaked input: %d %q %q", code, out, stderr)
			}
			report := decodeReport(t, out)
			if report.Iterations != 2 || report.Counts != (runner.Counts{Total: 4, Passed: 4}) || report.Assertions.Passed != 2 {
				t.Fatalf("wrong aggregate: %+v", report)
			}
			for index, entry := range report.Requests {
				if entry.Iteration != index/2+1 || entry.Ordinal != index+1 {
					t.Fatalf("incorrect iteration position: %+v", entry)
				}
			}
			want := []string{
				"/first/first:one:environment-fallback:collection-fallback:request-only",
				"/second/first:captured:environment-fallback:collection-fallback:base",
				"/first/second:two,two:environment-fallback:collection-fallback:request-only",
				"/second/second:captured:environment-fallback:collection-fallback:base",
			}
			if actual := workflowTraffic(seen); !reflect.DeepEqual(actual, want) || !reflect.DeepEqual(before, snapshot(t, dir)) {
				t.Fatalf("iteration order, isolation, precedence, or local files changed: %v", actual)
			}
		})
	}
}

func TestCLIDataRejectsMalformedAndOversizedBeforeSending(t *testing.T) {
	var sends atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { sends.Add(1) }))
	defer server.Close()
	dir := t.TempDir()
	var items []model.Item
	for index := range 11 {
		items = append(items, testItem(fmt.Sprintf("item-%d", index), server.URL))
	}
	writeCollection(t, dir, testCollection(items...))
	for _, fixture := range []struct{ name, input, code string }{
		{secret + ".csv", "valid,valid\n1,2\n", "data_file_invalid_or_unreadable"},
		{secret + ".json", `[{"row":"valid"},{"row":"invalid","\u0072ow":"duplicate"}]`, "data_file_invalid_or_unreadable"},
		{secret + ".json", `[{"row":"valid"},{"row":{"nested":1}}]`, "data_file_invalid_or_unreadable"},
		{secret + ".csv", "valid\nrow\n" + strings.Repeat("x", 1<<20), "data_file_invalid_or_unreadable"},
		{secret + ".csv", "valid\n" + strings.Repeat("row\n", 1001), "data_file_invalid_or_unreadable"},
		{secret + ".csv", "valid\n" + strings.Repeat("row\n", 1000), "iteration_limit_exceeded"},
	} {
		path := workflowFile(t, dir, fixture.name, fixture.input)
		code, out, stderr := invoke(t, context.Background(), dir, "--data-file", path)
		if code != exitInput || out != "" || stderr != "reva-run: "+fixture.code+"\n" || sends.Load() != 0 {
			t.Fatalf("invalid data sent requests or returned unexpected output: %d %q %q", code, out, stderr)
		}
	}
}

func TestCLIJUnitWorkflow(t *testing.T) {
	var sends atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		sends.Add(1)
		w.Header().Set("Set-Cookie", "sid="+secret)
		if strings.HasPrefix(req.URL.Path, "/fail/") {
			w.WriteHeader(http.StatusTeapot)
		}
		_, _ = io.WriteString(w, secret)
	}))
	defer server.Close()
	dir := t.TempDir()
	first := testItem("passing-"+secret, server.URL+"/pass/{{value}}")
	failure := testItem("failing-"+secret, server.URL+"/fail/{{value}}")
	failure.Request.TestScript = `pm.test("credential<>&", function(){ pm.response.to.have.status(200); });`
	writeCollection(t, dir, testCollection(first, failure))
	path := workflowFile(t, dir, secret+".json", `[{"value":"row-private"},{"value":"second"}]`)
	code, out, stderr := invoke(t, context.Background(), dir, "--data-file", path, "--stop-on-failure", "--report-format", "junit")
	if code != exitFailed || stderr != "" || sends.Load() != 2 || strings.Contains(out, "row-private") || strings.Contains(out, server.URL) || !strings.HasPrefix(out, xml.Header) {
		t.Fatalf("unexpected JUnit run: %d %q %q", code, out, stderr)
	}
	var report struct {
		XMLName  xml.Name `xml:"testsuite"`
		Tests    int      `xml:"tests,attr"`
		Failures int      `xml:"failures,attr"`
		Errors   int      `xml:"errors,attr"`
		Skipped  int      `xml:"skipped,attr"`
		Cases    []struct {
			Name      string  `xml:"name,attr"`
			ClassName string  `xml:"classname,attr"`
			Time      float64 `xml:"time,attr"`
			Failure   *struct {
				Message string `xml:"message,attr"`
			} `xml:"failure"`
			Skipped *struct {
				Message string `xml:"message,attr"`
			} `xml:"skipped"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal([]byte(out), &report); err != nil || report.Tests != 4 || report.Failures != 1 || report.Errors != 0 || report.Skipped != 2 || len(report.Cases) != 4 {
		t.Fatalf("invalid JUnit report: %+v, %v", report, err)
	}
	if report.Cases[0].Name != "request-1" || report.Cases[0].Time < 0 || report.Cases[1].Failure == nil || report.Cases[1].Failure.Message != string(runner.CodeAssertion) ||
		report.Cases[2].ClassName != "reva-run.iteration-2" || report.Cases[2].Skipped == nil || report.Cases[2].Skipped.Message != string(runner.CodeSkippedFailure) {
		t.Fatal("JUnit cases lost order, iteration or diagnostics")
	}
	if code, out, _ := invoke(t, context.Background(), dir, "--data-file", path); code != exitFailed || sends.Load() != 6 || decodeReport(t, out).Counts.Failed != 2 {
		t.Fatal("data run without stop-on-failure did not attempt every row")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code, out, stderr := invoke(t, ctx, dir, "--data-file", path, "--report-format", "junit"); code != exitCancelled || stderr != "" || sends.Load() != 6 ||
		!strings.Contains(out, `name="outcome" value="cancelled"`) || strings.Count(out, `message="skipped_cancellation"`) != 4 {
		t.Fatal("pre-cancelled data run did not report all skipped positions")
	}
	if code, out, stderr := invoke(t, context.Background(), dir, "--report-format", secret); code != exitInput || out != "" || stderr != "reva-run: invalid_arguments\n" || sends.Load() != 6 {
		t.Fatal("invalid format ran or produced output")
	}
}

func TestCLITemporaryCookieWorkflow(t *testing.T) {
	seen := make(chan string, 24)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen <- req.URL.Path + ":" + req.Header.Get("Cookie")
		if req.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: secret, Path: "/"})
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	init := testItem("login", server.URL+"/login")
	manual := testItem("manual", server.URL+"/manual")
	manual.Request.Headers = []model.KV{{Key: "Cookie", Value: "sid=explicit"}}
	writeCollection(t, dir, testCollection(testItem("start", server.URL+"/start"), init, manual, testItem("probe", server.URL+"/probe")))
	path := workflowFile(t, dir, "rows.json", `[{"row":1},{"row":2}]`)
	before := snapshot(t, dir)
	withJar := []string{"/start:", "/login:", "/manual:sid=explicit", "/probe:sid=" + secret}
	for _, run := range []struct{ flags, want []string }{
		{nil, []string{"/start:", "/login:", "/manual:sid=explicit", "/probe:"}},
		{[]string{"--data-file", path, "--cookie-session"}, append(append([]string(nil), withJar...), withJar...)},
		{[]string{"--cookie-session"}, withJar},
	} {
		code, out, stderr := invoke(t, context.Background(), dir, run.flags...)
		if code != exitOK || stderr != "" || decodeReport(t, out).Counts.Passed != len(run.want) || !reflect.DeepEqual(workflowTraffic(seen), run.want) {
			t.Fatal("cookie opt-in, isolation or explicit header precedence failed")
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("CLI persisted cookie state")
	}
}

func TestCLIJUnitCancellationAndWriteFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var sends atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		sends.Add(1)
		cancel()
		<-req.Context().Done()
	}))
	defer server.Close()
	dir := t.TempDir()
	writeCollection(t, dir, testCollection(testItem("request", server.URL)))
	path := workflowFile(t, dir, "rows.csv", "item\nfirst\nsecond\nthird\n")
	code, out, stderr := invoke(t, ctx, dir, "--data-file", path, "--report-format", "junit")
	if code != exitCancelled || stderr != "" || sends.Load() != 1 || !strings.Contains(out, `errors="1" skipped="2"`) ||
		!strings.Contains(out, `<error message="cancelled"></error>`) || strings.Count(out, `message="skipped_cancellation"`) != 2 {
		t.Fatalf("in-flight cancellation was not reported: %d %q %q", code, out, stderr)
	}
	var diagnostics bytes.Buffer
	if code := run(ctx, []string{"--data-dir", dir, "--collection", "collection", "--report-format", "junit"}, failingWriter{}, &diagnostics); code != exitFailed || diagnostics.String() != "reva-run: output_failed\n" {
		t.Fatal("JUnit output failure did not return the fixed write diagnostic")
	}
}

func workflowTraffic(seen chan string) []string {
	var traffic []string
	for len(seen) != 0 {
		traffic = append(traffic, <-seen)
	}
	return traffic
}

func workflowFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
