package script

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/vars"
)

func TestRunPreRequestAndVariables(t *testing.T) {
	req := model.NewRequest("request")
	req.URL = "https://example.test/old"
	req.Headers = []model.KV{
		{Key: "X-Token", Value: "old", Disabled: true},
		{Key: "x-token", Value: "duplicate"},
		{Key: "X-Remove", Value: "old"},
		{Key: "x-remove", Value: "duplicate"},
	}
	values := []model.KV{{Key: "host", Value: "base"}, {Key: "host", Value: "active"}, {Key: "disabled", Value: "hidden", Disabled: true}}
	original := append([]model.KV(nil), values...)
	updated, logs, err := RunPre(`
        if (pm.request.method !== "GET" || pm.request.url.indexOf("old") === -1) throw new Error("request getters");
        if (pm.environment.get("host") !== "active") throw new Error("scope precedence");
        if (pm.variables.get("disabled") !== undefined || pm.variables.get("missing") !== undefined) throw new Error("missing vars");
        pm.environment.set("host", "example.test");
        pm.variables.set("path", "new");
        pm.variables.set("disabled", 42);
        if (pm.environment.get("path") !== "new") throw new Error("shared vars");
        pm.request.url = "https://{{host}}/{{path}}";
        pm.request.method = "POST";
        pm.request.headers.add({key: "X-Added", value: "yes"});
        pm.request.headers.upsert({key: "X-TOKEN", value: "fresh"});
        pm.request.headers.upsert({key: "X-New", value: "new"});
        pm.request.headers.remove("X-REMOVE");
        console.log("url", pm.request.url, {enabled: true}, null);
    `, &req, values, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, original) {
		t.Fatal("modified caller's variables")
	}
	if req.Method != model.MethodPost || vars.Apply(req, updated).URL != "https://example.test/new" {
		t.Fatalf("request = %+v, vars = %+v", req, updated)
	}
	wantHeaders := []model.KV{{Key: "X-TOKEN", Value: "fresh"}, {Key: "X-Added", Value: "yes"}, {Key: "X-New", Value: "new"}}
	if !reflect.DeepEqual(req.Headers, wantHeaders) {
		t.Fatalf("headers = %+v", req.Headers)
	}
	if v, ok := vars.Lookup(updated, "disabled"); !ok || v != "42" {
		t.Fatalf("re-enabled variable = %q, %v", v, ok)
	}
	if len(logs) != 1 || logs[0] != `url https://{{host}}/{{path}} {"enabled":true} null` {
		t.Fatalf("logs = %q", logs)
	}
}

func testResponse() model.Response {
	return model.Response{StatusCode: 200, Status: "200 OK", Duration: 125500 * time.Microsecond,
		Headers: []model.KV{{Key: "Content-Type", Value: "application/json"}}, Body: []byte(`{"token":"secret","items":[1,2]}`)}
}

func TestRunTestsResponseAndAssertions(t *testing.T) {
	results, logs, err := RunTests(`
        pm.test("response status", function() {
            pm.response.to.have.status(200);
            pm.expect(pm.response).to.have.status(200);
            pm.expect(pm.response.code).to.equal(200);
            pm.expect(pm.response.status).to.equal("200 OK");
        });
        pm.test("time", function() {
            pm.expect(pm.response.responseTime).to.equal(125.5);
            pm.expect(pm.response.responseTime).to.be.above(100);
            pm.expect(pm.response.responseTime).to.be.below(500);
        });
        pm.test("headers and body", function() {
            pm.expect(pm.response.headers.get("content-type")).to.equal("application/json");
            pm.expect(pm.response.headers.get("absent")).to.equal(undefined);
            pm.expect(pm.response.text()).to.include("secret");
            pm.expect(pm.response.json()).to.eql({items: [1, 2], token: "secret"});
            pm.expect(pm.response.json()).to.include({items: [1, 2]});
            pm.expect(pm.response.json().items).to.include(2);
            pm.expect(pm.response.json().token).to.be.ok;
        });
        pm.test("variables", function() {
            pm.variables.set("token", pm.response.json().token);
            pm.expect(pm.environment.get("token")).to.equal("secret");
        });
        console.log("done", pm.response.code);
    `, model.NewRequest("req"), testResponse(), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("results = %+v", results)
	}
	for _, result := range results {
		if !result.Passed || result.Error != "" {
			t.Errorf("failed: %+v", result)
		}
	}
	if !reflect.DeepEqual(logs, []string{"done 200"}) {
		t.Fatalf("logs = %q", logs)
	}
}

func TestRunTestsAssertionFailuresContinue(t *testing.T) {
	assertions := []string{
		`pm.expect(1).to.equal("1")`, `pm.expect({a:1}).to.eql({a:2})`,
		`pm.expect([1]).to.eql([1,2])`, `pm.expect(5).to.be.above(5)`,
		`pm.expect(5).to.be.below(5)`, `pm.expect("abc").to.include("z")`,
		`pm.expect([1,2]).to.include(3)`, `pm.expect({a:1}).to.include({a:2})`,
		`pm.expect(false).to.be.ok`, `pm.response.to.have.status(404)`,
		`throw new Error("callback error")`, `throw "string exception"`,
	}
	for _, assertion := range assertions {
		t.Run(assertion, func(t *testing.T) {
			results, _, err := RunTests(`pm.test("fails", function() {`+assertion+`}); pm.test("next", function() { pm.expect(true).to.be.ok; });`, model.Request{}, testResponse(), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 || results[0].Passed || results[0].Error == "" || !results[1].Passed {
				t.Fatalf("results = %+v", results)
			}
		})
	}
}

func TestScriptExceptionsAndSyntaxErrors(t *testing.T) {
	for _, source := range []string{`throw new Error("boom")`, `throw "boom"`, `var = ;`} {
		t.Run(source, func(t *testing.T) {
			req := model.Request{}
			_, _, preErr := RunPre(source, &req, nil, 0)
			_, _, testErr := RunTests(source, req, testResponse(), nil, 0)
			for _, err := range []error{preErr, testErr} {
				if err == nil || (!strings.Contains(err.Error(), "boom") && !strings.Contains(err.Error(), "Unexpected token")) {
					t.Fatalf("error = %v", err)
				}
			}
		})
	}
	resp := testResponse()
	resp.Body = []byte("invalid JSON")
	results, _, err := RunTests(`pm.test("json", function() { pm.response.json(); });`, model.Request{}, resp, nil, 0)
	if err != nil || len(results) != 1 || results[0].Passed || results[0].Error == "" {
		t.Fatalf("JSON result = %+v, error = %v", results, err)
	}
}

func TestScriptHardTimeout(t *testing.T) {
	for _, source := range []string{`while(true){}`, `try { while(true){} } catch(e) {}`, `pm.test("loop", function() { while(true){} });`} {
		t.Run(source, func(t *testing.T) {
			start := time.Now()
			req := model.Request{}
			_, _, preErr := RunPre(source, &req, nil, 20*time.Millisecond)
			_, _, testErr := RunTests(source, req, testResponse(), nil, 20*time.Millisecond)
			if !errors.Is(preErr, ErrTimeout) || !errors.Is(testErr, ErrTimeout) {
				t.Fatalf("timeouts: %v, %v", preErr, testErr)
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("timeout took %s", elapsed)
			}
		})
	}
}

func TestRunPrePreservesMutationsAndLogsBeforeException(t *testing.T) {
	req := model.Request{}
	values, logs, err := RunPre(`pm.request.url="changed"; pm.variables.set("x", "y"); console.log("before"); throw new Error("stop");`, &req, nil, 0)
	if err == nil || req.URL != "changed" || len(values) != 1 || len(logs) != 1 {
		t.Fatalf("req=%+v values=%+v logs=%q err=%v", req, values, logs, err)
	}
}

func TestRunTestsIsolatesRequestAndVariables(t *testing.T) {
	req := model.NewRequest("req")
	req.Headers = []model.KV{{Key: "X-Keep", Value: "original"}}
	values := []model.KV{{Key: "keep", Value: "original"}}
	_, _, err := RunTests(`pm.request.headers.upsert({key:"X-Keep", value:"changed"}); pm.variables.set("keep", "changed");`, req, testResponse(), values, 0)
	if err != nil {
		t.Fatal(err)
	}
	if req.Headers[0].Value != "original" || values[0].Value != "original" {
		t.Fatal("test script changed caller state")
	}
	_, _, err = RunPre(`if (pm.variables.get("keep") !== undefined) throw new Error("leaked");`, &req, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunPreNilRequest(t *testing.T) {
	if _, _, err := RunPre("", nil, nil, 0); err == nil {
		t.Fatal("nil request must return error")
	}
}

func TestRunTestsRejectsAsyncCallbacks(t *testing.T) {
	results, _, err := RunTests(`pm.test("async", function() { return {then: function() {}}; });`, model.Request{}, testResponse(), nil, 0)
	if err != nil || len(results) != 1 || results[0].Passed || !strings.Contains(results[0].Error, "Async") {
		t.Fatalf("results=%+v err=%v", results, err)
	}
}

func BenchmarkRunPre(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		req := model.NewRequest("req")
		if _, _, err := RunPre(`pm.request.headers.upsert({key:"X-Token",value:"token"});`, &req, nil, 0); err != nil {
			b.Fatal(err)
		}
	}
}
