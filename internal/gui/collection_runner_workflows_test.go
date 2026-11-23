package gui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service/fake"
	requestservice "github.com/runbrick/reva/internal/service/request"
)

func TestCollectionRunnerFolderAndDataWorkflow(t *testing.T) {
	seen := make(chan string, 12)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		seen <- req.URL.Path + ":" + req.Header.Get("Authorization")
	}))
	defer server.Close()
	svc := fake.New()
	svc.Requests = requestservice.New()
	first := collectionRunItem("first", server.URL+"/{{row}}/first")
	second := collectionRunItem("second", server.URL+"/{{row}}/second")
	first.Request.Auth.Type, second.Request.Auth.Type = model.AuthInherit, model.AuthInherit
	collection := model.Collection{ID: "runner", Name: "Runner", Items: []model.Item{
		collectionRunItem("outside", server.URL+"/outside"),
		{Type: model.ItemFolder, ID: "outer", Name: "Outer", Auth: model.Auth{Type: model.AuthBearer, Token: "ancestor"}, Items: []model.Item{
			collectionRunItem("sibling", server.URL+"/sibling"),
			{Type: model.ItemFolder, ID: "inner", Name: "Inner", Items: []model.Item{first, second}},
		}},
	}}
	if err := svc.Collections.Save(collection); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bad, good := filepath.Join(dir, "private-path.json"), filepath.Join(dir, "rows.csv")
	if err := os.WriteFile(bad, []byte(`[{"row":"private-row"},{"row":null}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, []byte("row\nalpha\nbeta\n"), 0600); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	collectionRunOverview(h, collection.ID, "")
	text, labels := h.App.S.Collections, i18n.RunnerFor(h.App.S.Lang)
	h.click(text.RunCollection)
	if err := h.D.Resize(960, 600); err != nil {
		t.Fatal(err)
	}
	if !h.shows(labels.WholeCollection) {
		t.Fatal("runner default scope should be the whole collection")
	}
	h.click(labels.Scope)
	h.click(fmt.Sprintf(labels.FolderPath, "Outer", "Inner"))
	if err := h.D.QueueOpenFiles([]string{bad}, nil); err != nil {
		t.Fatal(err)
	}
	h.click(labels.BrowseData)
	if len(seen) != 0 || h.App.collections.runner.job != nil || !h.shows(labels.DataSelected) {
		t.Fatal("choosing a data file started a run or lost the selection")
	}
	h.click(text.RunStart)
	h.waitFor("invalid iteration file", func() bool { return h.shows(labels.InvalidData) })
	if len(seen) != 0 || h.shows("private-row") || h.shows("private-path") {
		t.Fatal("invalid file sent requests or leaked data")
	}
	h.click(labels.ClearData)
	if h.shows(labels.InvalidData) || h.shows(labels.DataSelected) {
		t.Fatal("clearing a data file did not reset its status")
	}
	if err := h.D.QueueOpenFiles([]string{good}, nil); err != nil {
		t.Fatal(err)
	}
	h.click(labels.BrowseData)
	h.click(text.RunStart)
	report := collectionRunReport(h)
	assertCollectionRunPaths(t, seen, "/alpha/first:Bearer ancestor", "/alpha/second:Bearer ancestor", "/beta/first:Bearer ancestor", "/beta/second:Bearer ancestor")
	if report.Iterations != 2 || report.Counts != (runner.Counts{Total: 4, Passed: 4}) || report.Requests[2].Iteration != 2 || !h.shows(fmt.Sprintf(labels.Iterations, 2)) || h.shows("alpha") || h.shows("ancestor") {
		t.Fatalf("unexpected desktop data report: %+v", report)
	}
}

func TestCollectionRunnerStaleFolderRequiresReselection(t *testing.T) {
	var sends atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { sends.Add(1) }))
	defer server.Close()
	svc := fake.New()
	svc.Requests = requestservice.New()
	collection := model.Collection{ID: "runner", Name: "Runner", Items: []model.Item{
		collectionRunItem("outside", server.URL),
		{Type: model.ItemFolder, ID: "folder", Name: "Saved folder", Items: []model.Item{collectionRunItem("inside", server.URL)}},
	}}
	if err := svc.Collections.Save(collection); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	collectionRunOverview(h, collection.ID, "")
	text, labels := h.App.S.Collections, i18n.RunnerFor(h.App.S.Lang)
	h.click(text.RunCollection)
	h.click(labels.Scope)
	h.click("Saved folder")
	collection.Items = collection.Items[:1]
	if err := svc.Collections.Save(collection); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		h.click(text.RunStart)
		if sends.Load() != 0 || h.App.collections.runner.job != nil || !h.shows(labels.MissingFolder) {
			t.Fatal("a stale folder allowed a send or silent fallback")
		}
	}
	h.click(labels.Scope)
	h.click(labels.WholeCollection)
	h.click(text.RunStart)
	if report := collectionRunReport(h); sends.Load() != 1 || report.Counts.Passed != 1 {
		t.Fatal("explicit scope reselection did not recover")
	}
}

func BenchmarkCollectionRunnerSetupPaint(b *testing.B) {
	application, collection := authBenchmarkApp(b)
	application.openCollectionRunner(collection.ID)
	authBenchmarkPaint(b, application, application.Page, 960, 600)
}
