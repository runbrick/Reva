package gui

import (
	"context"
	"errors"
	"fmt"
	"maps"
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
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	requestservice "github.com/runbrick/reva/internal/service/request"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func collectionRunItem(id, url string) model.Item {
	req := model.NewRequest(id)
	req.Name, req.URL = id, url
	return model.Item{Type: model.ItemRequest, ID: id, Name: id, Request: &req}
}

func collectionRunOverview(h *harness, collectionID, folderID string) {
	h.t.Helper()
	collection, _ := h.Svc.Collections.Get(collectionID)
	name := collection.Name
	if folderID != "" {
		name = model.FindItem(collection.Items, folderID).Name
	}
	h.click(fmt.Sprintf(h.App.S.Collections.SettingsFor, name))
}

func collectionRunReport(h *harness) runner.Report {
	h.t.Helper()
	h.waitFor("collection report", func() bool { return h.App.collections.runner.report.Version == runner.ReportVersion })
	return h.App.collections.runner.report
}

func TestCollectionRunnerSavedInputsAndReportParity(t *testing.T) {
	wire := make(chan string, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wire <- req.URL.Path
		if strings.HasSuffix(req.URL.Path, "/failure") {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	collection := model.Collection{ID: "runner", Name: "Runner", Variables: []model.KV{{Key: "scope", Value: "collection"}}}
	first := collectionRunItem("first", server.URL+"/{{scope}}/first")
	first.Request.TestScript = `pm.test("first assertion", function(){ pm.response.to.have.status(202); });`
	collection.Items = []model.Item{first, {Type: model.ItemFolder, ID: "folder", Name: "Folder", Items: []model.Item{
		collectionRunItem("failure", server.URL+"/{{scope}}/failure"),
		collectionRunItem("last", server.URL+"/{{scope}}/last"),
	}}}
	active := model.Environment{ID: "active", Name: "Global environment", Variables: []model.KV{{Key: "scope", Value: "global"}}}
	selected := model.Environment{ID: "chosen", Name: "Selected staging", Variables: []model.KV{{Key: "scope", Value: "staging"}}}
	st := researchStore(t, dir, collection, active)
	if err := st.Environments().Save(selected); err != nil {
		t.Fatal(err)
	}
	svc := service.Services{Collections: st.Collections(), Environments: st.Environments(), Settings: st.Settings(), History: st.History(), Requests: requestservice.New()}
	h := newHarness(t, svc)
	h.App.OpenRequest(*first.Request, collection.ID)
	dirty := h.App.Active()
	dirty.Request.URL, dirty.Dirty = server.URL+"/dirty", true
	collectionRunOverview(h, collection.ID, "folder")
	text := h.App.S.Collections
	for _, hit := range h.hitLabels() {
		if hit == text.RunCollection {
			t.Fatal("folder Overview offers root collection run")
		}
	}
	collectionRunOverview(h, collection.ID, "")
	settingsDraft(h).variables[0].Value = "dirty-collection-scope"
	h.click(text.RunCollection)
	h.waitFor("default environment", func() bool { return h.shows(text.RunNoEnvironment) })
	if len(wire) != 0 || h.App.collections.runner.job != nil {
		t.Fatal("opening the dialog started a run")
	}
	collection.Items[1].Items[0].Request.TestScript = `pm.test("intentional failure", function(){ pm.response.to.have.status(202); });`
	if err := svc.Collections.Save(collection); err != nil {
		t.Fatal(err)
	}
	before := researchDisk(t, dir)
	h.click(text.RunStart)
	defaultReport := collectionRunReport(h)
	assertCollectionRunPaths(t, wire, "/collection/first", "/collection/failure", "/collection/last")
	if defaultReport.Counts != (runner.Counts{Total: 3, Passed: 2, Failed: 1}) {
		t.Fatalf("default run counts = %+v", defaultReport.Counts)
	}
	if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	h.waitFor("dismissed report", func() bool { return !h.App.collections.runner.open })
	h.click(text.RunCollection)
	h.click(text.RunEnvironment)
	h.click(selected.Name)
	h.click(text.RunStopOnFailure)
	h.click(text.RunStart)
	report := collectionRunReport(h)
	assertCollectionRunPaths(t, wire, "/staging/first", "/staging/failure")
	if report.Counts != (runner.Counts{Total: 3, Passed: 1, Failed: 1, Skipped: 1}) || report.Assertions != (runner.Assertions{Passed: 1, Failed: 1}) || report.Requests[1].StatusCode != http.StatusTeapot {
		t.Fatalf("desktop report = %+v", report)
	}
	stored, _ := svc.Collections.Get(collection.ID)
	core := runner.Run(context.Background(), requestservice.New(), stored, svc.Settings.Get(), selected.Variables, runner.Options{StopOnFailure: true}).Report()
	assertCollectionRunPaths(t, wire, "/staging/first", "/staging/failure")
	if !reflect.DeepEqual(collectionRunWithoutTiming(report), collectionRunWithoutTiming(core)) {
		t.Fatalf("desktop report = %+v, CLI core report = %+v", report, core)
	}
	if !maps.Equal(before, researchDisk(t, dir)) || dirty.Request.URL != server.URL+"/dirty" || !dirty.Dirty {
		t.Fatal("collection runner changed stored files or the request draft")
	}
}

func collectionRunWithoutTiming(report runner.Report) runner.Report {
	report.DurationNS = 0
	report.Requests = append([]runner.RequestReport{}, report.Requests...)
	for index := range report.Requests {
		report.Requests[index].DurationNS = 0
	}
	return report
}

func assertCollectionRunPaths(t *testing.T, wire chan string, want ...string) {
	t.Helper()
	var got []string
	for len(wire) > 0 {
		got = append(got, <-wire)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request paths = %v, want %v", got, want)
	}
}

func TestCollectionRunnerSnapshotSurvivesNavigationAndPreview(t *testing.T) {
	svc := fake.New()
	seen := make(chan model.Request, 3)
	release := make(chan struct{})
	svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, req model.Request) (model.Response, error) {
		seen <- req
		if req.ID == "first" {
			select {
			case <-release:
			case <-ctx.Done():
				return model.Response{}, ctx.Err()
			}
		}
		return model.Response{StatusCode: 200}, nil
	}
	collection := model.Collection{ID: "snapshot", Name: "Snapshot", Variables: []model.KV{{Key: "header", Value: "saved"}}, Items: []model.Item{
		collectionRunItem("first", "http://test.invalid/{{environment}}"),
		collectionRunItem("second", "http://test.invalid/{{environment}}"),
	}}
	collection.Items[1].Request.Headers = []model.KV{{Key: "X-Snapshot", Value: "{{header}}"}}
	env := model.Environment{ID: "env", Name: "Snapshot environment", Variables: []model.KV{{Key: "environment", Value: "original-env"}}}
	settings := svc.Settings.Get()
	settings.RequestTimeoutSec = 13
	authMust(t, svc.Collections.Save(collection))
	authMust(t, svc.Environments.Save(env))
	authMust(t, svc.Settings.Save(settings))
	h := newHarness(t, svc)
	background := h.App.Active()
	collectionRunOverview(h, collection.ID, "")
	text := h.App.S.Collections
	h.click(text.RunCollection)
	h.click(text.RunEnvironment)
	h.click(env.Name)
	h.click(text.RunStart)
	h.waitFor("first saved request", func() bool { return len(seen) == 1 })
	<-seen
	collection.Items[1].Request.URL = "http://test.invalid/changed-request"
	collection.Items[1].Request.Headers[0].Value = "changed-header"
	collection.Variables[0].Value = "changed-collection"
	env.Variables[0].Value = "changed-environment"
	settings.RequestTimeoutSec = 42
	authMust(t, svc.Collections.Save(collection))
	authMust(t, svc.Environments.Save(env))
	authMust(t, svc.Settings.Save(settings))
	h.App.Main = PageHistory
	requestRepaint(h)
	h.App.Main = PageWorkbench
	h.App.active = h.App.requestTabIndex(background.ID)
	h.App.requestTab(background).previewOpen = true
	requestRepaint(h)
	close(release)
	report := collectionRunReport(h)
	if report.Counts != (runner.Counts{Total: 2, Passed: 2}) || len(seen) != 1 {
		t.Fatalf("run restarted or was interrupted across pages: %+v, sends %d", report, len(seen))
	}
	second := <-seen
	if second.ID != "second" || second.URL != "http://test.invalid/original-env" || second.Headers[0].Value != "saved" || second.Options.TimeoutMs != 13000 {
		t.Fatalf("second request used current settings/data: %+v", second)
	}
}

func TestCollectionRunnerCancellation(t *testing.T) {
	for _, operation := range []string{"button", "escape", "window"} {
		t.Run(operation, func(t *testing.T) {
			var started, cancelled, later atomic.Int32
			cleanup := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/hold" {
					later.Add(1)
					return
				}
				started.Add(1)
				select {
				case <-req.Context().Done():
					cancelled.Add(1)
				case <-cleanup:
				}
			}))
			defer server.Close()
			defer close(cleanup)
			svc := fake.New()
			svc.Requests = requestservice.New()
			collection := model.Collection{ID: "cancel", Name: "Cancel", Items: []model.Item{
				collectionRunItem("first", server.URL+"/hold"), collectionRunItem("later", server.URL+"/later"),
			}}
			authMust(t, svc.Collections.Save(collection))
			h := newHarness(t, svc)
			collectionRunOverview(h, collection.ID, "")
			text := h.App.S.Collections
			h.click(text.RunCollection)
			h.click(text.RunStart)
			h.waitFor("in-flight request", func() bool { return started.Load() == 1 })
			switch operation {
			case "button":
				h.click(text.RunCancel)
			case "escape":
				authMust(t, h.D.Key(gfx.KeyEscape, 0))
			case "window":
				h.D.Close()
			}
			if operation != "window" {
				report := collectionRunReport(h)
				if !h.App.collections.runner.open || report.Outcome != runner.Cancelled || report.Counts != (runner.Counts{Total: 2, Cancelled: 1, Skipped: 1}) {
					t.Fatalf("cancel did not leave a partial report: %+v", report)
				}
				h.click(text.RunClose)
				h.pump()
			}
			h.waitFor("HTTP cancellation", func() bool { return cancelled.Load() == 1 })
			if started.Load() != 1 || later.Load() != 0 || len(svc.History.List()) != 0 {
				t.Fatal("cancel allowed a new request or wrote history")
			}
		})
	}
}

func TestCollectionRunnerAllowsPreviouslyAuthorizedDownload(t *testing.T) {
	for _, phase := range []string{"response pending", "write pending"} {
		t.Run(phase, func(t *testing.T) {
			svc := fake.New()
			collection := model.Collection{ID: "downloads", Name: "Downloads"}
			authMust(t, svc.Collections.Save(collection))
			responseRelease, writeRelease := make(chan struct{}), make(chan struct{})
			var sends, writes atomic.Int32
			body := []byte{0, 1, 2, 255}
			svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, _ model.Request) (model.Response, error) {
				sends.Add(1)
				select {
				case <-responseRelease:
					return model.Response{StatusCode: 200, Body: body}, nil
				case <-ctx.Done():
					return model.Response{}, ctx.Err()
				}
			}
			h := newHarness(t, svc)
			tab := h.App.Active()
			tab.Request.URL = "http://test.invalid/download"
			requestRepaint(h)
			requestState := h.App.requestTab(tab)
			requestState.write = ui.NewEffectKeyed(func(ctx context.Context, job *requestDownload) (struct{}, error) {
				writes.Add(1)
				select {
				case <-writeRelease:
					return struct{}{}, os.WriteFile(job.path, job.body, 0600)
				case <-ctx.Done():
					return struct{}{}, ctx.Err()
				}
			})
			path := filepath.Join(t.TempDir(), "response.bin")
			authMust(t, h.D.QueueSaveFile(path, nil))
			h.click(h.App.S.Request.SendMenu)
			h.click(h.App.S.Request.SendDownload)
			h.waitFor("authorized send to start", func() bool { return sends.Load() == 1 })
			if phase == "write pending" {
				close(responseRelease)
				h.waitFor("authorized write to start", func() bool { return writes.Load() == 1 })
			}
			collectionRunOverview(h, collection.ID, "")
			text := h.App.S.Collections
			h.click(text.RunCollection)
			h.click(text.RunStart)
			report := collectionRunReport(h)
			if report.Outcome != runner.Passed || report.Counts.Total != 0 || !h.shows(text.RunEmpty) {
				t.Fatal("empty collection did not leave an empty report")
			}
			if phase == "response pending" {
				close(responseRelease)
				h.waitFor("authorized write through the runner sheet", func() bool { return writes.Load() == 1 })
			}
			close(writeRelease)
			h.waitFor("download completion through the runner sheet", func() bool {
				data, err := os.ReadFile(path)
				return err == nil && string(data) == string(body) && requestState.writeJob == nil
			})
			if !h.App.collections.runner.open || sends.Load() != 1 || writes.Load() != 1 || len(svc.History.List()) != 1 {
				t.Fatal("runner sheet lost, duplicated or added authorized request work")
			}
		})
	}
}

func TestCollectionRunnerVirtualizedAndRedactedAtMinimumViewport(t *testing.T) {
	const secret = "misplaced-sensitive-value"
	for _, language := range []string{"en", "zh"} {
		t.Run(language, func(t *testing.T) {
			svc := fake.New()
			settings := svc.Settings.Get()
			settings.Language = language
			if language == "zh" {
				settings.Theme = "dark"
			}
			authMust(t, svc.Settings.Save(settings))
			svc.Requests.(*fake.Requests).Fn = func(_ context.Context, req model.Request) (model.Response, error) {
				if strings.HasSuffix(req.ID, "-0") {
					return model.Response{}, errors.New(secret)
				}
				return model.Response{StatusCode: 202, Body: []byte(secret)}, nil
			}
			collection := model.Collection{ID: secret, Name: secret, Items: make([]model.Item, 10000)}
			for index := range collection.Items {
				collection.Items[index] = collectionRunItem(fmt.Sprintf("%s-%d", secret, index), "http://test.invalid/"+secret)
			}
			collection.Items[1].Request.TestScript = `console.log("` + secret + `"); pm.test("` + secret + `", function(){ pm.response.to.have.status(202); });`
			authMust(t, svc.Collections.Save(collection))
			h := newHarness(t, svc)
			h.App.openCollectionSettings(collectionNode{collectionID: collection.ID}, collectionSettingsGeneral)
			authMust(t, h.D.Resize(960, 600))
			authMust(t, h.D.Resize(960, 600))
			text := h.App.S.Collections
			h.click(text.RunCollection)
			h.click(text.RunStart)
			report := collectionRunReport(h)
			if len(report.Requests) != 10000 || report.Counts.Failed != 1 || report.Assertions.Passed != 1 || !h.shows(text.RunOrdinal) || !h.shows("202") {
				t.Fatalf("large report is incomplete: counts %+v, assertions %+v", report.Counts, report.Assertions)
			}
			assertCollectionRunPaint(t, h, secret)
			h.click("1 ·")
			authMust(t, h.D.Key(gfx.KeyEnd, 0))
			h.waitFor("last report ordinal", func() bool {
				for _, label := range h.D.Snapshot().Labels {
					if label.Value == "10000" {
						return true
					}
				}
				return false
			})
			assertCollectionRunPaint(t, h, secret)
		})
	}
}

func assertCollectionRunPaint(t *testing.T, h *harness, secret string) {
	t.Helper()
	snapshot := h.D.Snapshot()
	if len(snapshot.Texts) > 100 || len(snapshot.Hits) > 40 {
		t.Fatalf("summary paints too many rows: %d text elements, %d targets", len(snapshot.Texts), len(snapshot.Hits))
	}
	for _, text := range snapshot.Texts {
		if strings.Contains(text.Value, secret) {
			t.Fatal("summary exposes a sensitive value in its paint")
		}
	}
	for _, hit := range snapshot.Hits {
		if strings.Contains(hit.Label, secret) {
			t.Fatal("summary exposes a sensitive value in its controls")
		}
	}
}
