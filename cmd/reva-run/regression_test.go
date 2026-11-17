package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
)

func researchAssertSafeOutput(t *testing.T, stdout, stderr string, canaries ...string) {
	t.Helper()
	for _, canary := range canaries {
		if strings.Contains(stdout+stderr, canary) {
			t.Fatalf("CLI disclosed caller-controlled text %q", canary)
		}
	}
}

func TestResearchCLIBinaryNestedFailuresKeepSafePartialReport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "reva-run")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("cannot build CLI: %s", output)
	}
	const payload = "qa-cli-body-private"
	const assertionName = "qa-cli-assertion-private"
	const scriptError = "qa-cli-exception-private"
	const urlSecret = "qa-cli-url-private"
	got := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- r.URL.Path + "|" + r.Header.Get("Authorization") + "|" + string(body)
		w.Header().Set("X-Private", secret)
		w.WriteHeader(503)
		_, _ = io.WriteString(w, payload)
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	first := testItem("z-first", "{{host}}/"+urlSecret)
	first.Request.Method = model.MethodPost
	first.Request.Auth.Type = model.AuthInherit
	first.Request.Body = model.Body{Mode: model.BodyRaw, Text: payload}
	first.Request.PreScript = `console.log("` + secret + `");`
	first.Request.TestScript = `pm.test("` + assertionName + `", function(){pm.response.to.have.status(503);}); console.log(pm.response.text());`
	pre := testItem("a-pre", srv.URL+"/must-not-send")
	pre.Request.PreScript = `console.log("` + payload + `"); throw new Error("` + scriptError + `");`
	assertion := testItem("m-assert", srv.URL+"/assert")
	assertion.Request.TestScript = `pm.test("` + assertionName + `", function(){pm.response.to.have.status(200);}); throw new Error("` + scriptError + `");`
	transport := testItem("b-transport", "http://["+urlSecret)
	last := testItem("c-last", srv.URL+"/last")
	col := testCollection(first, model.Item{ID: "folder", Name: secret, Type: model.ItemFolder, Items: []model.Item{pre, {ID: "nested", Name: payload, Type: model.ItemFolder, Items: []model.Item{assertion, transport}}}}, last)
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
	col.Variables = []model.KV{{Key: "host", Value: srv.URL}, {Key: "token", Value: "collection-token"}}
	writeCollection(t, dir, col)
	writeJSON(t, dir, "environments/env.json", model.Environment{Version: 1, ID: "env", Name: scriptError, Variables: []model.KV{{Key: "token", Value: secret}}})
	settings := model.DefaultSettings()
	settings.ActiveEnvID = "env"
	writeJSON(t, dir, "settings.json", settings)
	writeJSON(t, dir, "history.json", []model.HistoryEntry{{ID: "history", Error: secret}})
	before := snapshot(t, dir)
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "continue", true: "stop"}[stop], func(t *testing.T) {
			args := []string{"--data-dir", dir, "--collection", col.ID, "--environment", "env"}
			if stop {
				args = append(args, "--stop-on-failure")
			}
			cmd := exec.CommandContext(ctx, binary, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			var exit *exec.ExitError
			if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatal("failed collection did not produce actual process exit 1", err)
			}
			researchAssertSafeOutput(t, stdout.String(), stderr.String(), secret, payload, assertionName, scriptError, urlSecret, "collection-token", srv.URL)
			report := decodeReport(t, stdout.String())
			if stderr.Len() != 0 || report.Outcome != runner.Failed || len(report.Requests) != 5 || report.Requests[0].StatusCode != 503 || report.Assertions.Passed != 1 {
				t.Fatal("failed process lost successful partial results or emitted diagnostics")
			}
			wantCodes := []runner.Code{runner.CodeOK, runner.CodePreScript, runner.CodeTestScript, runner.CodeTransport, runner.CodeOK}
			wantCounts := runner.Counts{Total: 5, Passed: 2, Failed: 3}
			wantSends := 3
			if stop {
				wantCodes = []runner.Code{runner.CodeOK, runner.CodePreScript, runner.CodeSkippedFailure, runner.CodeSkippedFailure, runner.CodeSkippedFailure}
				wantCounts, wantSends = runner.Counts{Total: 5, Passed: 1, Failed: 1, Skipped: 3}, 1
			} else if report.Assertions.Failed != 1 {
				t.Fatal("test exception discarded the failed assertion count")
			}
			if report.Counts != wantCounts || len(got) != wantSends {
				t.Fatal("nested failure continuation/stop or report counts changed")
			}
			for i, request := range report.Requests {
				if request.Ordinal != i+1 || request.Code != wantCodes[i] {
					t.Fatalf("wrong request position/code at %d", i+1)
				}
			}
			if <-got != "/"+urlSecret+"|Bearer "+secret+"|"+payload {
				t.Fatal("process did not load explicit environment/auth/body")
			}
			if !stop && (<-got != "/assert||" || <-got != "/last||") {
				t.Fatal("process reordered nested requests or sent the failed pre request")
			}
			if !reflect.DeepEqual(before, snapshot(t, dir)) {
				t.Fatal("failed process mutated native data")
			}
		})
	}
}

func TestResearchCLICancellationAfterFailurePreservesSafeNestedPositions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.URL.Path
		if strings.HasPrefix(r.URL.Path, "/cancel") {
			cancel()
			<-r.Context().Done()
			return
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	pre := testItem("z-failure", srv.URL+"/never-"+secret)
	pre.Request.PreScript = `console.log("` + secret + `"); throw new Error("` + secret + `");`
	col := testCollection(testItem("first", srv.URL+"/first-"+secret), model.Item{ID: "folder", Name: secret, Type: model.ItemFolder, Items: []model.Item{
		pre, {ID: "nested", Name: secret, Type: model.ItemFolder, Items: []model.Item{testItem("cancel", srv.URL+"/cancel-"+secret)}},
	}}, testItem("last", srv.URL+"/must-not-send"))
	writeCollection(t, dir, col)
	writeJSON(t, dir, "settings.json", model.DefaultSettings())
	writeJSON(t, dir, "environments/unused.json", model.Environment{Version: 1, ID: "unused", Name: secret})
	writeJSON(t, dir, "history.json", []model.HistoryEntry{{ID: "history", Error: secret}})
	before := snapshot(t, dir)
	code, stdout, stderr := invoke(t, ctx, dir)
	researchAssertSafeOutput(t, stdout, stderr, secret, srv.URL, col.Items[0].Request.URL)
	report := decodeReport(t, stdout)
	if code != 130 || stderr != "" || report.Outcome != runner.Cancelled || report.Counts != (runner.Counts{Total: 4, Passed: 1, Failed: 1, Cancelled: 1, Skipped: 1}) {
		t.Fatal("cancellation hid prior failure or partial successful results")
	}
	want := []runner.Code{runner.CodeOK, runner.CodePreScript, runner.CodeCancelled, runner.CodeSkippedCancellation}
	for i, request := range report.Requests {
		if request.Ordinal != i+1 || request.Code != want[i] || request.Sent != (i == 0 || i == 2) {
			t.Fatal("cancellation lost native nested positions/send flags")
		}
	}
	if len(got) != 2 || <-got != "/first-"+secret || <-got != "/cancel-"+secret || !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("cancelled CLI sent later requests, changed order, or mutated data")
	}
}
