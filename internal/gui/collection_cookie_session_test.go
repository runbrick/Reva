package gui

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/request"
)

func assertRunStatuses(t *testing.T, report runner.Report, want []int) {
	t.Helper()
	if len(report.Requests) != len(want) {
		t.Fatalf("report has %d requests, want %d: %+v", len(report.Requests), len(want), report)
	}
	for index, status := range want {
		if report.Requests[index].StatusCode != status {
			t.Fatalf("request %d status = %d, want %d: %+v", index, report.Requests[index].StatusCode, status, report)
		}
	}
}

func TestCollectionCookieSessionOptInAndIsolation(t *testing.T) {
	var mu sync.Mutex
	var firstProbeCookieHeaders []string
	var manualProbeCookieHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/probe-first":
			mu.Lock()
			firstProbeCookieHeaders = append(firstProbeCookieHeaders, r.Header.Get("Cookie"))
			mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
		case "/manual-probe":
			mu.Lock()
			manualProbeCookieHeader = r.Header.Get("Cookie")
			mu.Unlock()
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "runner-cookie-secret", Path: "/"})
		case "/manual-login":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "manual-secret", Path: "/"})
		case "/me":
			if cookie, err := r.Cookie("sid"); err != nil || cookie.Value != "runner-cookie-secret" {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	collection := model.Collection{ID: "runner-cookie", Name: "Runner cookie", Items: []model.Item{
		collectionRunItem("probe-first", server.URL+"/probe-first"),
		collectionRunItem("login", server.URL+"/login"),
		collectionRunItem("me", server.URL+"/me"),
	}}
	environment := model.Environment{ID: "runner-env", Name: "Runner environment"}
	st := researchStore(t, dir, collection, environment)
	svc := service.Services{Requests: request.New(), Collections: st.Collections(), Environments: st.Environments(), Settings: st.Settings(), History: st.History()}
	h := newHarness(t, svc)
	requestEnglish(h)
	tab := h.App.Active()
	tab.CollectionID = collection.ID
	h.App.selectEnvironment(collection.ID, environment.ID)
	manualSettings(t, h)
	h.click(h.App.S.Request.CookieSessionEnabled)
	manualSend(t, h, tab, server.URL+"/manual-login", http.StatusOK)
	before := researchDisk(t, dir)
	h.App.openCollectionSettings(collectionNode{collectionID: collection.ID}, collectionSettingsGeneral)
	requestRepaint(h)
	text := h.App.S.Collections
	var reports []runner.Report
	for index, enabled := range []bool{false, true, true, false} {
		h.click(text.RunCollection)
		h.click(text.RunEnvironment)
		h.click(environment.Name)
		if enabled {
			h.click(text.RunUseCookieSession)
		}
		h.click(text.RunStart)
		report := collectionRunReport(h)
		reports = append(reports, report)
		if report.Outcome != runner.Passed {
			t.Fatalf("run %d outcome = %q: %+v", index, report.Outcome, report)
		}
		h.click(text.RunClose)
		h.pump()
	}
	assertRunStatuses(t, reports[0], []int{401, 200, 401})
	assertRunStatuses(t, reports[1], []int{401, 200, 200})
	assertRunStatuses(t, reports[2], []int{401, 200, 200})
	assertRunStatuses(t, reports[3], []int{401, 200, 401})
	mu.Lock()
	probes := slices.Clone(firstProbeCookieHeaders)
	mu.Unlock()
	if !slices.Equal(probes, []string{"", "", "", ""}) {
		t.Fatalf("first request inherited cookie: %q", probes)
	}
	for _, report := range reports {
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(encoded, []byte("runner-cookie-secret")) {
			t.Fatal("runner report exposed an automatic cookie")
		}
	}
	if !maps.Equal(before, researchDisk(t, dir)) {
		t.Fatal("runner stored an automatic cookie")
	}
	manualSend(t, h, tab, server.URL+"/manual-probe", http.StatusOK)
	mu.Lock()
	manualProbe := manualProbeCookieHeader
	mu.Unlock()
	if manualProbe != "sid=manual-secret" || len(svc.History.List()) == 0 {
		t.Fatalf("manual session changed or manual send omitted history: cookie %q, history %+v", manualProbe, svc.History.List())
	}
}

func TestCollectionCookieSessionCancellation(t *testing.T) {
	var slowCalls, thirdCalls atomic.Int32
	var releaseSlow atomic.Bool
	slowEntered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/probe-first":
			if r.Header.Get("Cookie") != "" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "runner-cookie-secret", Path: "/"})
			w.WriteHeader(http.StatusUnauthorized)
		case "/slow":
			slowCalls.Add(1)
			if cookie, err := r.Cookie("sid"); err != nil || cookie.Value != "runner-cookie-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if !releaseSlow.Load() {
				slowEntered <- struct{}{}
				<-r.Context().Done()
			}
		case "/me":
			thirdCalls.Add(1)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	collection := model.Collection{ID: "cancel-cookie", Name: "Cancel cookie", Items: []model.Item{
		collectionRunItem("probe-first", server.URL+"/probe-first"),
		collectionRunItem("slow", server.URL+"/slow"),
		collectionRunItem("me", server.URL+"/me"),
	}}
	st := researchStore(t, dir, collection, model.Environment{ID: "cancel-env", Name: "Cancel environment"})
	svc := service.Services{Requests: request.New(), Collections: st.Collections(), Environments: st.Environments(), Settings: st.Settings(), History: st.History()}
	h := newHarness(t, svc)
	collectionRunOverview(h, collection.ID, "")
	text := h.App.S.Collections
	h.click(text.RunCollection)
	h.click(text.RunUseCookieSession)
	h.click(text.RunStart)
	h.waitFor("authenticated slow request", func() bool { return len(slowEntered) == 1 })
	<-slowEntered
	h.click(text.RunCancel)
	report := collectionRunReport(h)
	if report.Outcome != runner.Cancelled || report.Counts.Cancelled != 1 || report.Counts.Skipped != 1 || thirdCalls.Load() != 0 {
		t.Fatalf("cancellation did not stop the authenticated run: %+v", report)
	}
	assertRunStatuses(t, report, []int{401, 0, 0})
	h.click(text.RunClose)
	h.pump()
	releaseSlow.Store(true)
	h.click(text.RunCollection)
	h.click(text.RunUseCookieSession)
	h.click(text.RunStart)
	freshRun := collectionRunReport(h)
	if freshRun.Requests[0].StatusCode != http.StatusUnauthorized || freshRun.Requests[1].StatusCode != http.StatusOK || slowCalls.Load() != 2 {
		t.Fatalf("cancelled run left a cookie behind: %+v", freshRun)
	}
}

func TestCollectionCookieSessionControlsAtMinimumViewport(t *testing.T) {
	for _, language := range []i18n.Lang{i18n.Zh, i18n.En} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(string(language)+"/"+theme, func(t *testing.T) {
				svc := fake.New()
				entered := make(chan struct{}, 1)
				svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, _ model.Request) (model.Response, error) {
					entered <- struct{}{}
					<-ctx.Done()
					return model.Response{}, ctx.Err()
				}
				settings := svc.Settings.Get()
				settings.Language, settings.Theme = string(language), theme
				if err := svc.Settings.Save(settings); err != nil {
					t.Fatal(err)
				}
				collection := model.Collection{ID: "viewport", Name: "Viewport", Items: []model.Item{
					collectionRunItem("request", "http://test.invalid"),
				}}
				if err := svc.Collections.Save(collection); err != nil {
					t.Fatal(err)
				}
				h := newHarness(t, svc)
				collectionRunOverview(h, collection.ID, "")
				if err := h.D.Resize(960, 600); err != nil {
					t.Fatal(err)
				}
				h.click(h.App.S.Collections.RunCollection)
				text := h.App.S.Collections
				for _, label := range []string{text.RunUseCookieSession, text.RunStart} {
					visible := false
					for _, hit := range h.D.Snapshot().Hits {
						b := hit.Bounds
						if hit.Label == label && b.X >= 0 && b.Y >= 0 && b.X+b.Width <= 960 && b.Y+b.Height <= 600 {
							visible = true
						}
					}
					if !visible {
						t.Fatalf("control %q outside 960x600: %v", label, h.hitLabels())
					}
				}
				if h.App.collections.runner.useCookieSession {
					t.Fatal("new collection run opted into cookies")
				}
				h.click(text.RunUseCookieSession)
				h.click(text.RunStart)
				h.waitFor("in-flight request", func() bool { return len(entered) == 1 })
				visible := false
				for _, hit := range h.D.Snapshot().Hits {
					b := hit.Bounds
					if hit.Label == text.RunCancel && b.X >= 0 && b.Y >= 0 && b.X+b.Width <= 960 && b.Y+b.Height <= 600 {
						visible = true
					}
				}
				if !visible {
					t.Fatalf("cancel control outside 960x600: %v", h.hitLabels())
				}
				h.click(text.RunCancel)
				collectionRunReport(h)
			})
		}
	}
}
