package runner_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	requestservice "github.com/runbrick/reva/internal/service/request"
	"github.com/runbrick/reva/internal/service/store"
)

func TestResearchStoredNestedRunFailureThenCancellationIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got := make(chan string, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.URL.Path + "|" + r.Header.Get("Authorization") + "|" + r.Header.Get("X-Local")
		if r.URL.Path == "/cancel" {
			cancel()
			<-r.Context().Done()
			return
		}
		w.WriteHeader(201)
	}))
	defer srv.Close()
	first, failed, blocked, later := item("z-first", "{{host}}/first"), item("a-pre-failure", "{{host}}/never-pre"), item("m-cancel", "{{host}}/cancel"), item("b-later", "{{host}}/never-later")
	first.Request.PreScript = `pm.environment.set("token", "request-local-token"); pm.variables.set("local", "first-only");`
	first.Request.TestScript = `pm.test("created", function(){pm.response.to.have.status(201);});`
	failed.Request.PreScript = `pm.environment.set("token", "failed-local-token"); throw new Error("pre failure");`
	blocked.Request.TestScript = `throw new Error("cancelled response tests must not run");`
	for _, it := range []*model.Item{&first, &failed, &blocked, &later} {
		it.Request.Auth.Type = model.AuthInherit
		it.Request.Headers = []model.KV{{Key: "X-Local", Value: "{{local}}"}}
	}
	col := collection(first, model.Item{ID: "folder", Name: "Same name", Type: model.ItemFolder, Auth: model.Auth{Type: model.AuthBearer, Token: "{{token}}"}, Items: []model.Item{
		{ID: "empty", Name: "Empty folder", Type: model.ItemFolder},
		failed,
		{ID: "nested", Name: "Same name", Type: model.ItemFolder, Items: []model.Item{blocked}},
	}}, later)
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{token}}"}
	col.Variables = []model.KV{{Key: "host", Value: srv.URL}, {Key: "token", Value: "collection-token"}, {Key: "local", Value: "baseline"}}
	env := model.Environment{ID: "environment", Name: "Stored environment", Variables: []model.KV{{Key: "token", Value: "environment-token"}}}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	settings := model.DefaultSettings()
	settings.ActiveEnvID = env.ID
	for _, err := range []error{st.Collections().Save(col), st.Environments().Save(env), st.Settings().Save(settings), st.History().Add(model.HistoryEntry{Request: *first.Request, StatusCode: 204})} {
		if err != nil {
			t.Fatal(err)
		}
	}
	st, err = store.Open(dir)
	if err != nil || len(st.LoadErrors()) != 0 {
		t.Fatal("cannot reload native fixture", err)
	}
	col, _ = st.Collections().Get(col.ID)
	env, _ = st.Environments().Get(env.ID)
	before := directoryBytes(t, dir)
	result := runner.Run(ctx, requestservice.New(), col, st.Settings().Get(), env.Variables, runner.Options{})
	if result.Outcome != runner.Cancelled || !errors.Is(result.Err, context.Canceled) || result.Report().Counts != (runner.Counts{Total: 4, Passed: 1, Failed: 1, Cancelled: 1, Skipped: 1}) {
		t.Fatalf("failure followed by cancellation lost its partial results: %+v", result.Report())
	}
	ids := []string{"z-first", "a-pre-failure", "m-cancel", "b-later"}
	codes := []runner.Code{runner.CodeOK, runner.CodePreScript, runner.CodeCancelled, runner.CodeSkippedCancellation}
	for i, request := range result.Requests {
		if request.Ordinal != i+1 || request.ItemID != ids[i] || request.Code != codes[i] || request.Sent != (i == 0 || i == 2) {
			t.Fatalf("native order or skipped-send semantics changed at position %d", i+1)
		}
	}
	if len(got) != 2 || <-got != "/first|Bearer request-local-token|first-only" || <-got != "/cancel|Bearer environment-token|baseline" {
		t.Fatal("nested requests were reordered, sent after cancellation, or shared script variables")
	}
	if result.Requests[2].Execution.Tests.Err != nil || result.Requests[3].Execution != nil {
		t.Fatal("cancellation executed later stages")
	}
	if !reflect.DeepEqual(before, directoryBytes(t, dir)) || len(st.History().List()) != 1 {
		t.Fatal("partial failed/cancelled run changed native persistence")
	}
	// A separate run must start at the persisted baseline, even after failed
	// pre scripts and cancellation. Stop-on-failure must stop before /cancel.
	retryCtx, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer retryCancel()
	result = runner.Run(retryCtx, requestservice.New(), col, st.Settings().Get(), env.Variables, runner.Options{StopOnFailure: true})
	if result.Outcome != runner.Failed || result.Report().Counts != (runner.Counts{Total: 4, Passed: 1, Failed: 1, Skipped: 2}) || len(got) != 1 || <-got != "/first|Bearer request-local-token|first-only" {
		t.Fatal("rerun or nested stop-on-failure changed")
	}
	if result.Requests[2].Code != runner.CodeSkippedFailure || result.Requests[3].Sent || !reflect.DeepEqual(before, directoryBytes(t, dir)) {
		t.Fatal("stopped rerun sent a later request or mutated persistence")
	}
}
