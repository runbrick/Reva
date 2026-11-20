package gui

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/request"
	"github.com/runbrick/reva/internal/syncing"
)

func manualHarness(t *testing.T, handler http.HandlerFunc) (*harness, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	svc := fake.New()
	svc.Requests = request.New()
	h := newHarness(t, svc)
	requestEnglish(h)
	return h, server.URL
}

func manualSend(t *testing.T, h *harness, tab *Tab, url string, want int) {
	t.Helper()
	tab.Request.URL = url
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("manual cookie HTTP response", func() bool { return !tab.Sending })
	if tab.Response == nil || tab.Response.StatusCode != want {
		t.Fatalf("response for %s: %+v; error: %s; want %d", url, tab.Response, tab.Err, want)
	}
}

func manualSettings(t *testing.T, h *harness) {
	t.Helper()
	selected := false
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label != h.App.S.Request.Settings {
			continue
		}
		b := hit.Bounds
		if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
			t.Fatal(err)
		}
		selected = true
		break
	}
	if !selected {
		t.Fatalf("request settings tab absent: %v", h.hitLabels())
	}
	if !h.shows(h.App.S.Request.CookieSessionEnabled) || !h.shows(h.App.S.Request.CookieSessionClear) {
		t.Fatalf("cookie controls absent (section %d): %v", h.App.requestTab(h.App.Active()).section, h.hitLabels())
	}
}

func TestManualCookieSessionLifecycleAndScope(t *testing.T) {
	h, base := manualHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "manual-secret", Path: "/"})
			return
		}
		if cookie, err := r.Cookie("sid"); err == nil && cookie.Value == "manual-secret" {
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})
	first := h.App.Active()
	manualSettings(t, h)
	manualSend(t, h, first, base+"/login", http.StatusOK)
	manualSend(t, h, first, base+"/me", http.StatusUnauthorized)
	h.click(h.App.S.Request.CookieSessionEnabled)
	manualSend(t, h, first, base+"/login", http.StatusOK)
	otherTab := h.App.NewTab()
	manualSend(t, h, otherTab, base+"/me", http.StatusOK)

	firstCollection, err := h.Svc.Collections.Create("First")
	if err != nil {
		t.Fatal(err)
	}
	secondCollection, err := h.Svc.Collections.Create("Second")
	if err != nil {
		t.Fatal(err)
	}
	otherTab.CollectionID = secondCollection.ID
	manualSend(t, h, otherTab, base+"/me", http.StatusUnauthorized)
	first.CollectionID = firstCollection.ID
	manualSend(t, h, first, base+"/me", http.StatusUnauthorized)
	manualSend(t, h, first, base+"/login", http.StatusOK)
	otherTab.CollectionID = firstCollection.ID
	manualSend(t, h, otherTab, base+"/me", http.StatusOK)

	envA, err := h.Svc.Environments.Create("A")
	if err != nil {
		t.Fatal(err)
	}
	envB, err := h.Svc.Environments.Create("B")
	if err != nil {
		t.Fatal(err)
	}
	h.App.selectEnvironment(firstCollection.ID, envB.ID)
	if col, _ := h.Svc.Collections.Get(firstCollection.ID); col.EnvironmentScope == nil || col.EnvironmentScope.ActiveID != envB.ID {
		t.Fatalf("picker did not select collection environment B: %+v; active collection %q", col.EnvironmentScope, h.App.Active().CollectionID)
	}
	manualSend(t, h, otherTab, base+"/me", http.StatusUnauthorized)
	h.App.selectEnvironment(firstCollection.ID, envA.ID)
	manualSend(t, h, otherTab, base+"/me", http.StatusUnauthorized)
	h.App.selectEnvironment(firstCollection.ID, "")
	manualSend(t, h, otherTab, base+"/me", http.StatusOK)
	h.App.active = 0
	requestRepaint(h)
	manualSettings(t, h)
	h.click(h.App.S.Request.CookieSessionClear)
	manualSend(t, h, first, base+"/me", http.StatusUnauthorized)
	manualSend(t, h, first, base+"/login", http.StatusOK)
	h.click(h.App.S.Request.CookieSessionEnabled)
	h.click(h.App.S.Request.CookieSessionEnabled)
	manualSend(t, h, first, base+"/me", http.StatusUnauthorized)
	restarted := newHarness(t, h.Svc)
	manualSend(t, restarted, restarted.App.Active(), base+"/me", http.StatusUnauthorized)
	if first.Dirty || otherTab.Dirty {
		t.Fatal("session controls dirtied authored requests")
	}
}

func TestManualCookieSessionClearDuringSend(t *testing.T) {
	for _, action := range []string{"clear", "disable"} {
		t.Run(action, func(t *testing.T) {
			entered, release := make(chan struct{}, 1), make(chan struct{})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			h, base := manualHarness(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/slow-login" {
					entered <- struct{}{}
					<-release
					http.SetCookie(w, &http.Cookie{Name: "sid", Value: "manual-secret", Path: "/"})
					return
				}
				if cookie, err := r.Cookie("sid"); err == nil && cookie.Value == "manual-secret" {
					return
				}
				w.WriteHeader(http.StatusUnauthorized)
			})
			tab := h.App.Active()
			manualSettings(t, h)
			h.click(h.App.S.Request.CookieSessionEnabled)
			tab.Request.URL = base + "/slow-login"
			h.App.Send(tab)
			requestRepaint(h)
			<-entered
			if action == "clear" {
				h.click(h.App.S.Request.CookieSessionClear)
			} else {
				h.click(h.App.S.Request.CookieSessionEnabled)
				h.click(h.App.S.Request.CookieSessionEnabled)
			}
			close(release)
			h.waitFor("slow login completes", func() bool { return !tab.Sending })
			manualSend(t, h, tab, base+"/me", http.StatusUnauthorized)
		})
	}
}

func TestManualCookieSessionSSE(t *testing.T) {
	closed := make(chan struct{})
	h, base := manualHarness(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "manual-secret", Path: "/"})
		case "/events":
			if cookie, err := r.Cookie("sid"); err != nil || cookie.Value != "manual-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			flushSSE(w, "data: authenticated\n\n")
			<-r.Context().Done()
			close(closed)
		case "/me":
			if cookie, err := r.Cookie("sid"); err != nil || cookie.Value != "manual-secret" {
				w.WriteHeader(http.StatusUnauthorized)
			}
		}
	})
	tab := h.App.Active()
	manualSettings(t, h)
	h.click(h.App.S.Request.CookieSessionEnabled)
	manualSend(t, h, tab, base+"/login", http.StatusOK)
	tab.Request.URL = base + "/events"
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("live authenticated SSE event", func() bool { return h.shows("authenticated") })
	if !tab.Sending || tab.Response != nil {
		t.Fatal("SSE event did not arrive during the send")
	}
	h.click(h.App.S.Response.SSEDisconnect)
	h.waitFor("SSE disconnect", func() bool { return !tab.Sending })
	assertSSEClosed(t, closed)
	manualSend(t, h, tab, base+"/me", http.StatusOK)
}

func TestManualCookieSessionDownload(t *testing.T) {
	h, base := manualHarness(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "manual-secret", Path: "/"})
			return
		}
		if cookie, err := r.Cookie("sid"); err != nil || cookie.Value != "manual-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("download authenticated"))
	})
	tab := h.App.Active()
	manualSettings(t, h)
	h.click(h.App.S.Request.CookieSessionEnabled)
	manualSend(t, h, tab, base+"/login", http.StatusOK)
	tab.Request.URL = base + "/download"
	path := filepath.Join(t.TempDir(), "response.bin")
	if err := h.D.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Request.SendMenu)
	h.click(h.App.S.Request.SendDownload)
	h.waitFor("authenticated download", func() bool {
		body, err := os.ReadFile(path)
		return err == nil && string(body) == "download authenticated"
	})
	if tab.Response == nil || tab.Response.StatusCode != http.StatusOK {
		t.Fatalf("download status: %+v; error: %s", tab.Response, tab.Err)
	}
}

func TestManualCookieSessionNeverPersists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "manual-secret", Path: "/"})
		case "/broken-redirect":
			if cookie, err := r.Cookie("sid"); err == nil && cookie.Value == "manual-secret" {
				w.Header().Set("Location", "/bad%2?echo=sid=manual-secret")
				w.WriteHeader(http.StatusFound)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	t.Setenv("REVA_HOME", dir)
	col := model.Collection{ID: "collection", Name: "Cookies"}
	env := model.Environment{ID: "environment", Name: "Local"}
	st := researchStore(t, dir, col, env)
	svc := service.Services{Requests: request.New(), Collections: st.Collections(), Environments: st.Environments(), Settings: st.Settings(), History: st.History()}
	h := newHarness(t, svc)
	requestEnglish(h)
	tab := h.App.Active()
	tab.CollectionID = col.ID
	manualSettings(t, h)
	before := researchDisk(t, dir)
	h.click(h.App.S.Request.CookieSessionEnabled)
	h.click(h.App.S.Request.CookieSessionClear)
	h.click(h.App.S.Request.CookieSessionEnabled)
	h.click(h.App.S.Request.CookieSessionEnabled)
	if tab.Dirty || !maps.Equal(before, researchDisk(t, dir)) {
		t.Fatal("session controls modified authored data")
	}
	manualSend(t, h, tab, server.URL+"/login", http.StatusOK)
	tab.Request.URL = server.URL + "/broken-redirect"
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("redirect transport error", func() bool { return !tab.Sending })
	entries := svc.History.List()
	if len(entries) == 0 || !strings.Contains(tab.Err, "manual-secret") || entries[0].Error != h.App.S.Request.CookieSessionHistoryError {
		t.Fatalf("live error = %q; history = %+v", tab.Err, entries)
	}
	command, err := h.App.requestCURLFor(tab)
	if err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Request.SendMenu)
	h.click(h.App.S.Request.CopyCURL)
	if strings.Contains(h.D.Clipboard(), "manual-secret") {
		t.Fatal("copied cURL leaked the session")
	}
	h.click(h.App.S.Request.Preview)
	if strings.Contains(previewLabels(h), "manual-secret") {
		t.Fatal("preview leaked the session")
	}
	preview := h.App.requestPreviewSnapshot(tab)
	previewJSON, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	snapshotJSON, err := json.Marshal(syncing.Capture(svc))
	if err != nil {
		t.Fatal(err)
	}
	historyJSON, err := os.ReadFile(filepath.Join(dir, "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{historyJSON, snapshotJSON, []byte(command), previewJSON} {
		if bytes.Contains(data, []byte("manual-secret")) {
			t.Fatal("automatic cookie escaped into persistent or preview data")
		}
	}
	if tab.Dirty {
		t.Fatal("session controls marked the request dirty")
	}
}

func TestManualCookieSessionControlsAtMinimumWindowSize(t *testing.T) {
	for _, language := range []i18n.Lang{i18n.Zh, i18n.En} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(string(language)+"/"+theme, func(t *testing.T) {
				svc := fake.New()
				settings := svc.Settings.Get()
				settings.Language, settings.Theme = string(language), theme
				if err := svc.Settings.Save(settings); err != nil {
					t.Fatal(err)
				}
				h := newHarness(t, svc)
				if err := h.D.Resize(960, 600); err != nil {
					t.Fatal(err)
				}
				h.App.requestTab(h.App.Active()).section = requestSettings
				if err := h.D.Resize(960, 600); err != nil {
					t.Fatal(err)
				}
				if !h.shows(h.App.S.Request.CookieSessionEnabled) || !h.shows(h.App.S.Request.CookieSessionClear) {
					t.Fatal("cookie controls not visible at minimum window size")
				}
				for _, label := range []string{h.App.S.Request.CookieSessionEnabled, h.App.S.Request.CookieSessionClear} {
					found := false
					for _, hit := range h.D.Snapshot().Hits {
						if hit.Label == label && hit.Bounds.X >= 0 && hit.Bounds.Y >= 0 && hit.Bounds.X+hit.Bounds.Width <= 960 && hit.Bounds.Y+hit.Bounds.Height <= 600 {
							found = true
						}
					}
					if !found {
						t.Fatalf("control %q outside 960x600: %v", label, h.hitLabels())
					}
				}
				h.click(h.App.S.Request.CookieSessionEnabled)
				h.click(h.App.S.Request.CookieSessionClear)
				if !h.App.request.cookies.enabled {
					t.Fatal("clear disabled session")
				}
			})
		}
	}
}
