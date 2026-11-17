package gui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type previewReadOnlyCollections struct{ service.CollectionService }

func (s previewReadOnlyCollections) Save(model.Collection) error { panic("preview saved collection") }
func (s previewReadOnlyCollections) SaveRequest(string, string, model.Request) (model.Request, error) {
	panic("preview saved request")
}

type previewReadOnlyEnvironments struct{ service.EnvironmentService }

func (s previewReadOnlyEnvironments) Save(model.Environment) error {
	panic("preview saved environment")
}

type previewReadOnlySettings struct{ service.SettingsService }

func (s previewReadOnlySettings) Save(model.Settings) error { panic("preview saved settings") }

type previewReadOnlyHistory struct{ service.HistoryService }

func (s previewReadOnlyHistory) Add(model.HistoryEntry) error { panic("preview wrote history") }

func previewFailOnSend(t *testing.T, svc service.Services) {
	t.Helper()
	svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		t.Error("opening preview sent a network request")
		return model.Response{}, nil
	}
}

func previewLabels(h *harness) string {
	var labels []string
	for _, label := range h.D.Snapshot().Labels {
		labels = append(labels, label.Value)
	}
	for _, hit := range h.D.Snapshot().Hits {
		labels = append(labels, hit.Label)
	}
	return strings.Join(labels, "\n")
}

func previewScroll(h *harness, amount float32) {
	h.t.Helper()
	// The preview modal is centered in the whole window; scroll its body.
	if err := h.D.Scroll(gfx.ScrollEvent{X: 640, Y: 380, DeltaY: amount, Precise: true}); err != nil {
		h.t.Fatal(err)
	}
}

func TestPreviewReadOnlyAndMaskedInPaintedSnapshot(t *testing.T) {
	t.Setenv("REVA_HOME", t.TempDir())
	sentinel := filepath.Join(os.Getenv("REVA_HOME"), "sentinel")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := fake.New()
	previewFailOnSend(t, svc)
	r := model.NewRequest("preview-request")
	r.URL = "https://literal-user:literal-pass@example.test/{{route}}?token=literal-query&normal={{value}}"
	r.Auth = model.Auth{Type: model.AuthInherit}
	r.Headers = []model.KV{{Key: "Authorization", Value: "literal-auth"}, {Key: "Proxy-Authorization", Value: "literal-proxy"}, {Key: "Cookie", Value: "literal-cookie"}, {Key: "X-Ordinary", Value: "literal-header"}}
	r.Body = model.Body{Mode: model.BodyBinary, FilePath: filepath.Join(t.TempDir(), "must-not-open-file")}
	r.PreScript = `throw new Error("must-not-execute-script")`
	col := model.Collection{ID: "collection", Name: "Workspace", Variables: []model.KV{{Key: "route", Value: "private-path"}, {Key: "value", Value: "ordinary-value"}}, Auth: model.Auth{Type: model.AuthBearer, Token: "inherited-token"}, Items: []model.Item{{ID: r.ID, Type: model.ItemRequest, Request: &r}}}
	if err := svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	env := model.Environment{ID: "env", Name: "Staging", Variables: []model.KV{{Key: "value", Value: "environment-value"}, {Key: "disabled", Value: "disabled-value", Disabled: true}}}
	if err := svc.Environments.Save(env); err != nil {
		t.Fatal(err)
	}
	settings := svc.Settings.Get()
	settings.ActiveEnvID = env.ID
	if err := svc.Settings.Save(settings); err != nil {
		t.Fatal(err)
	}
	svc.Collections = previewReadOnlyCollections{svc.Collections}
	svc.Environments = previewReadOnlyEnvironments{svc.Environments}
	svc.Settings = previewReadOnlySettings{svc.Settings}
	svc.History = previewReadOnlyHistory{svc.History}
	h := newHarness(t, svc)
	h.App.S = i18n.For(i18n.En)
	h.App.OpenRequest(r, col.ID)
	requestRepaint(h)
	draftBefore, _ := json.Marshal(h.App.Active().Request)
	dataBefore, _ := json.Marshal([]any{svc.Collections.List(), svc.Environments.List(), svc.Settings.Get(), svc.History.List()})
	dirty := h.App.Active().Dirty
	h.click(h.App.S.Request.Preview)
	requestRepaint(h)
	if !h.shows(h.App.S.Request.PreviewTitle) || !h.shows("BEFORE pre-request scripts") {
		t.Fatalf("preview did not open: %s", previewLabels(h))
	}
	for i := 0; i < 5; i++ {
		painted := previewLabels(h)
		for _, secret := range []string{"literal-user", "literal-pass", "literal-query", "literal-auth", "literal-proxy", "literal-cookie", "literal-header", "private-path", "ordinary-value", "inherited-token", "environment-value", "disabled-value", "must-not-open-file", "must-not-execute-script"} {
			if strings.Contains(painted, secret) {
				t.Errorf("painted preview leaked %q", secret)
			}
		}
		previewScroll(h, 220)
	}
	h.click("Close preview")
	requestRepaint(h)
	if h.App.Active().Err != "" || h.App.Active().Response != nil || h.App.Active().Sending || h.App.Active().Tests != nil {
		t.Fatal("preview entered the send/script pipeline")
	}
	draftAfter, _ := json.Marshal(h.App.Active().Request)
	dataAfter, _ := json.Marshal([]any{svc.Collections.List(), svc.Environments.List(), svc.Settings.Get(), svc.History.List()})
	if string(draftBefore) != string(draftAfter) || string(dataBefore) != string(dataAfter) || h.App.Active().Dirty != dirty {
		t.Fatal("preview mutated editor or persistence")
	}
	files, err := os.ReadDir(os.Getenv("REVA_HOME"))
	if err != nil || len(files) != 1 || files[0].Name() != "sentinel" {
		t.Fatalf("preview changed data directory: %v, %v", files, err)
	}
	if contents, err := os.ReadFile(sentinel); err != nil || string(contents) != "unchanged" {
		t.Fatal("preview modified an existing file")
	}
}

func TestPreviewRefreshesEnvironmentDraftAndAuthContext(t *testing.T) {
	svc := fake.New()
	previewFailOnSend(t, svc)
	r := model.NewRequest("request")
	r.URL = "https://example.test/{{host}}?one=first"
	r.Params = []model.KV{{Key: "one", Value: "first"}}
	r.Auth.Type = model.AuthInherit
	col := model.Collection{ID: "collection", Variables: []model.KV{{Key: "host", Value: "collection-host"}}, Auth: model.Auth{Type: model.AuthBearer, Token: "parent-token"}, Items: []model.Item{{ID: r.ID, Type: model.ItemRequest, Request: &r}}}
	if err := svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	envs := []model.Environment{
		{ID: "disabled", Name: "Fallback environment", Variables: []model.KV{{Key: "host", Value: "ignored-host", Disabled: true}}},
		{ID: "enabled", Name: "Override environment", Variables: []model.KV{{Key: "host", Value: "old-host"}, {Key: "host", Value: "new-host"}}},
	}
	for _, env := range envs {
		if err := svc.Environments.Save(env); err != nil {
			t.Fatal(err)
		}
	}
	h := newHarness(t, svc)
	h.App.S = i18n.For(i18n.En)
	h.App.OpenRequest(r, col.ID)
	h.App.setActiveEnvironment("disabled")
	requestRepaint(h)
	h.click(h.App.S.Request.Preview)
	requestRepaint(h)
	if !h.shows("Fallback environment") || !h.shows("Source: Collection") {
		t.Fatalf("missing environment/auth provenance: %s", previewLabels(h))
	}
	previewScroll(h, 400)
	if !h.shows("Collection row 1 · Winner") || !h.shows("Disabled (does not override)") {
		t.Fatalf("disabled fallback not explained: %s", previewLabels(h))
	}
	// Simulate an external settings change while open: the preview must re-read
	// the active environment, rather than retaining a resolved draft snapshot.
	h.App.setActiveEnvironment("enabled")
	requestRepaint(h)
	if !h.shows("Environment row 2 · Winner") || !h.shows("Shadowed by a later enabled row") {
		t.Fatalf("last enabled winner not refreshed: %s", previewLabels(h))
	}
	h.App.Active().Request.URL = "https://changed.test/{{host}}?two=second"
	col.Items = nil
	if err := svc.Collections.Save(col); err != nil {
		t.Fatal(err)
	}
	previewScroll(h, -1000)
	requestRepaint(h)
	// The URL and Params can diverge in an externally changed draft; preview
	// must merge both as the sender does, without normalizing the editor.
	if !h.shows("Override environment") || !h.shows("https://changed.test/[masked]?one=[masked]&two=[masked]") || !h.shows("Cannot inherit authentication:") {
		t.Fatalf("current draft/context not refreshed: %s", previewLabels(h))
	}
}

func TestPreviewMasksAllTabNamesAndRestoresTitlesOnClose(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	first := h.App.Active()
	first.Request.URL = "https://example.test?token=first-tab-credential"
	first.Dirty = true
	second := h.App.NewTab()
	second.Request.URL = "https://example.test/second"
	second.Request.Name = "second-tab-credential"
	requestRepaint(h)
	h.click(h.App.S.Request.Preview)
	requestRepaint(h)
	for _, tab := range h.App.Tabs() {
		if got := h.App.requestTabTitle(tab); got != h.App.S.Request.PreviewTitle {
			t.Fatalf("tab title was not masked: %q", got)
		}
	}
	if painted := previewLabels(h); strings.Contains(painted, "first-tab-credential") || strings.Contains(painted, "second-tab-credential") {
		t.Fatal("background tab hit label or title leaked a credential")
	}
	if !first.Dirty || first.ID == second.ID {
		t.Fatal("preview changed tab state or identity")
	}
	h.click("Close preview")
	requestRepaint(h)
	if h.App.requestTabTitle(first) != first.Request.URL || h.App.requestTabTitle(second) != second.Request.Name {
		t.Fatal("ordinary tab titles were not restored on close")
	}
	// Normal tab switching and closing retain their original stable identities.
	h.click(first.Request.URL)
	if h.App.Active() != first {
		t.Fatal("tab switching lost its identity")
	}
	h.App.closeRequestTabs(second.ID, false)
	requestRepaint(h)
	if len(h.App.Tabs()) != 1 || h.App.Active() != first || !first.Dirty {
		t.Fatal("normal closing or dirty indication changed")
	}
}

func TestPreviewLayoutsChineseEnglishLightDarkAtMinimumWindow(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
		for _, mode := range []string{"light", "dark"} {
			t.Run(string(lang)+"/"+mode, func(t *testing.T) {
				h := newHarness(t)
				s := h.Svc.Settings.Get()
				s.Language, s.Theme = string(lang), mode
				h.App.ApplySettings(s)
				h.App.Active().Request.URL = "https://example.test/{{unknown}}"
				if err := h.D.Resize(960, 600); err != nil {
					t.Fatal(err)
				}
				h.click(h.App.S.Request.Preview)
				if err := h.D.Resize(960, 600); err != nil {
					t.Fatal(err)
				}
				if !h.shows(h.App.S.Request.PreviewTitle) {
					t.Fatalf("missing localized preview: %s", previewLabels(h))
				}
				found := false
				for _, hit := range h.D.Snapshot().Hits {
					if hit.Label == h.App.S.Request.PreviewClose {
						b := hit.Bounds
						found = b.X >= 0 && b.Y >= 0 && b.X+b.Width <= 960 && b.Y+b.Height <= 600
					}
				}
				if !found {
					t.Fatal("preview close action outside the minimum window")
				}
				h.click(h.App.S.Request.PreviewClose)
			})
		}
	}
}

func TestPreviewIsScopedToHTTPAndClosesOnTabChange(t *testing.T) {
	h := newHarness(t)
	requestEnglish(h)
	h.App.Active().Request.URL = "https://example.test"
	requestRepaint(h)
	h.click(h.App.S.Request.Preview)
	requestRepaint(h)
	first := h.App.Active()
	h.App.NewTab()
	requestRepaint(h)
	if h.App.requestTab(first).previewOpen {
		t.Fatal("preview remained open after switching tabs")
	}
	h.App.Active().Request.Method = webSocketMethod
	h.App.Active().Request.URL = "ws://example.test"
	requestRepaint(h)
	for _, label := range h.hitLabels() {
		if label == h.App.S.Request.Preview {
			t.Fatal("HTTP preview offered in WebSocket editor")
		}
	}
	if h.App.Active().Request.Auth.Type != model.AuthNone {
		t.Fatal("preview changed websocket authentication")
	}
	// Switching directly to a WebSocket tab must also clear the old preview,
	// although WebSocket never calls requestPane.
	h.App.active = 0
	requestRepaint(h)
	h.click(h.App.S.Request.Preview)
	requestRepaint(h)
	h.App.active = 1
	requestRepaint(h)
	if h.App.requestTab(first).previewOpen || h.App.requestTabTitle(h.App.Active()) == h.App.S.Request.PreviewTitle {
		t.Fatal("HTTP preview title masking persisted in the WebSocket editor")
	}
}

func TestPreviewAllowsPreviouslyAuthorizedDownloadToComplete(t *testing.T) {
	for _, phase := range []string{"response pending", "write pending"} {
		t.Run(phase, func(t *testing.T) {
			h := newHarness(t)
			requestEnglish(h)
			tab := h.App.Active()
			tab.Request.URL = "https://example.test/download"
			tab.Request.Body = model.Body{Mode: model.BodyRaw, Text: "unsaved draft"}
			tab.Dirty = true
			requestRepaint(h)
			before, _ := json.Marshal(tab.Request)
			state := h.App.requestTab(tab)
			path := filepath.Join(t.TempDir(), "response.bin")
			body := []byte{0, 1, 2, 255}
			responseStarted, responseRelease := make(chan struct{}), make(chan struct{})
			writeStarted, writeRelease := make(chan struct{}), make(chan struct{})
			defer func() {
				for _, gate := range []chan struct{}{responseRelease, writeRelease} {
					select {
					case <-gate:
					default:
						close(gate)
					}
				}
			}()
			var sends, writes, cancelledWrites atomic.Int32
			h.Svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, _ model.Request) (model.Response, error) {
				if sends.Add(1) == 1 {
					close(responseStarted)
				}
				select {
				case <-responseRelease:
					return model.Response{StatusCode: 200, Body: body}, nil
				case <-ctx.Done():
					return model.Response{}, ctx.Err()
				}
			}
			// Hold the actual file write so the second case can open preview
			// after this Effect has already been declared and begun running.
			writeEffect := ui.NewEffectKeyed(func(ctx context.Context, job *requestDownload) (struct{}, error) {
				if writes.Add(1) == 1 {
					close(writeStarted)
				}
				select {
				case <-writeRelease:
					return struct{}{}, os.WriteFile(job.path, job.body, 0600)
				case <-ctx.Done():
					cancelledWrites.Add(1)
					return struct{}{}, ctx.Err()
				}
			})
			state.write = writeEffect
			// Opening preview without previously authorized work initiates
			// neither a send nor a write, even with a download Effect installed.
			h.click(h.App.S.Request.Preview)
			requestRepaint(h)
			if sends.Load() != 0 || writes.Load() != 0 {
				t.Fatal("preview initiated a send or write")
			}
			h.click(h.App.S.Request.PreviewClose)
			if err := h.D.QueueSaveFile(path, nil); err != nil {
				t.Fatal(err)
			}
			h.click(h.App.S.Request.SendMenu)
			h.click(h.App.S.Request.SendDownload)
			h.waitFor("authorized send to start", func() bool {
				select {
				case <-responseStarted:
					return true
				default:
					return false
				}
			})
			if phase == "write pending" {
				close(responseRelease)
				h.waitFor("authorized write to start", func() bool { return writes.Load() == 1 })
			}
			h.click(h.App.S.Request.Preview)
			requestRepaint(h)
			if phase == "response pending" {
				close(responseRelease)
			}
			h.waitFor("authorized write while preview is open", func() bool {
				select {
				case <-writeStarted:
					return true
				default:
					return false
				}
			})
			// Release the write only after preview has had another paint with
			// the same Effect declaration, to catch disappearance cancellation.
			requestRepaint(h)
			close(writeRelease)
			h.waitFor("download to complete while preview stays open", func() bool {
				data, err := os.ReadFile(path)
				return err == nil && string(data) == string(body) && state.writeJob == nil
			})
			requestRepaint(h)
			if !state.previewOpen || !h.shows(h.App.S.Request.PreviewTitle) {
				t.Fatal("download completion closed the preview")
			}
			if sends.Load() != 1 || writes.Load() != 1 || cancelledWrites.Load() != 0 || state.write != writeEffect {
				t.Fatalf("preview restarted or cancelled authorized work: sends %d, writes %d, cancelled %d", sends.Load(), writes.Load(), cancelledWrites.Load())
			}
			after, _ := json.Marshal(tab.Request)
			if string(before) != string(after) || !tab.Dirty {
				t.Fatal("preview or download completion changed the draft")
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
				t.Fatal("preview created an additional download")
			}
			if len(h.Svc.History.List()) != 1 {
				t.Fatal("preview initiated another send/history entry")
			}
		})
	}
}
