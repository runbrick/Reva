package runner_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	requestservice "github.com/runbrick/reva/internal/service/request"
)

func TestResponseCapturesFlowThroughCollectionAndStayInRun(t *testing.T) {
	var created atomic.Int32
	paths := make(chan string, 6)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/create" {
			fmt.Fprintf(writer, `{"order":{"id":"sample-%d"}}`, created.Add(1))
			return
		}
		paths <- req.URL.Path + ":" + req.Header.Get("X-Scoped")
		fmt.Fprint(writer, `{"ok":true}`)
	}))
	defer server.Close()
	before := item("before", server.URL+"/orders/{{orderId}}")
	create := item("create", server.URL+"/create")
	create.Request.TestScript = `pm.test("capture", function () {
  pm.expect(pm.response.captureJSON("/order/id", "orderId")).to.equal(true);
  pm.variables.set("scoped", "request-only");
});`
	after := item("after", server.URL+"/orders/{{orderId}}")
	after.Request.Headers = []model.KV{{Key: "X-Scoped", Value: "{{scoped}}"}}
	col := collection(before, create, model.Item{ID: "folder", Type: model.ItemFolder, Items: []model.Item{after}})
	col.Variables = []model.KV{{Key: "orderId", Value: "collection"}, {Key: "scoped", Value: "saved"}}
	env := []model.KV{{Key: "orderId", Value: "environment"}}
	beforeCol, beforeEnv := string(jsonBytes(t, col)), string(jsonBytes(t, env))
	snapshot := runner.NewSnapshot(col, model.DefaultSettings(), env)
	for _, expected := range []string{"sample-1", "sample-2"} {
		report := snapshot.RunReport(context.Background(), requestservice.New(), runner.Options{})
		if report.Outcome != runner.Passed || report.Assertions.Passed != 1 || report.Counts.Passed != 3 {
			t.Fatalf("unexpected run: %+v", report)
		}
		if reportJSON := string(jsonBytes(t, report)); strings.Contains(reportJSON, expected) || strings.Contains(reportJSON, "orderId") {
			t.Fatalf("report exposed captures: %s", reportJSON)
		}
		if got := <-paths; got != "/orders/environment:" {
			t.Fatalf("initial request unexpectedly used an earlier capture: %q", got)
		}
		if got := <-paths; got != "/orders/"+expected+":saved" {
			t.Fatalf("later request had the wrong run variables: %q", got)
		}
	}
	if beforeCol != string(jsonBytes(t, col)) || beforeEnv != string(jsonBytes(t, env)) {
		t.Fatal("collection or environment values changed")
	}
}

func TestFailedResponseStepDoesNotPublishCaptures(t *testing.T) {
	create := item("create", "http://example.test/create")
	next := item("next", "http://example.test/{{orderId}}")
	col := collection(create, next)
	col.Variables = []model.KV{{Key: "orderId", Value: "fallback"}}
	for _, failingPath := range []string{"/absent", "/items"} {
		create.Request.TestScript = `pm.test("capture existing", function () {
  pm.expect(pm.response.captureJSON("/id", "orderId")).to.equal(true);
});
pm.test("capture failure", function () {
  pm.expect(pm.response.captureJSON("` + failingPath + `", "other")).to.equal(true);
});`
		for _, stop := range []bool{false, true} {
			var urls []string
			sender := requestFunc(func(_ context.Context, req model.Request, _ []model.KV) (model.Response, error) {
				urls = append(urls, req.URL)
				return model.Response{StatusCode: 200, Body: []byte(`{"id":"captured","items":[]}`)}, nil
			})
			result := runner.Run(context.Background(), sender, col, model.DefaultSettings(), nil, runner.Options{StopOnFailure: stop})
			if result.Outcome != runner.Failed || result.Requests[0].Code != runner.CodeAssertion || len(result.Requests[0].Execution.Captures) != 0 {
				t.Fatalf("failed capture was published or not recorded as a failure: %+v", result.Report())
			}
			if stop && (len(urls) != 1 || result.Requests[1].Outcome != runner.Skipped) ||
				!stop && (len(urls) != 2 || urls[1] != "http://example.test/fallback") {
				t.Fatalf("unexpected continuation (stop=%v): %v", stop, urls)
			}
		}
	}
}
