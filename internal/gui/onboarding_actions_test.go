package gui

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/runbrick/reva/internal/service/store"
	"github.com/runbrick/reva/internal/storage"
	"github.com/runbrick/reva/internal/syncing"
	"github.com/vivaui/viva/gfx"
)

const (
	onboardingSampleURL    = "https://example.invalid/hello"
	onboardingEnvSecret    = "private-active-token"
	onboardingDraftSecret  = "private-edited-request"
	onboardingPracticeEnv  = "QA environment"
	onboardingEnvDraftName = "QA environment edit"
)

func TestOnboardingPracticeOpensIndependentOfflineDrafts(t *testing.T) {
	for _, language := range []string{"zh", "en"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(language+"/"+theme, func(t *testing.T) {
				h, unchanged := newOnboardingActionsHarness(t, language, theme, false)
				settings, request := h.App.S.Settings, h.App.S.Request
				practice, sampleName, importAction := "离线预览示例", "示例请求", "导入已有请求"
				if language == "en" {
					practice, sampleName, importAction = "Preview sample offline", "Sample request", "Import existing requests"
				}
				if settings.GuidePractice != practice || settings.GuidePracticeName != sampleName || settings.GuideImport != importAction {
					t.Fatal("guide actions and sample name do not match the localized contract")
				}
				originals := onboardingActionDrafts(h)
				for attempt := 1; attempt <= 2; attempt++ {
					h.click(settings.Title)
					h.click(settings.OpenGuide)
					for _, label := range []string{practice, importAction, settings.GuideSkip, settings.GuideDone} {
						if onboardingActionModalHit(t, h, label).Disabled {
							t.Fatalf("guide button %q is disabled", label)
						}
					}
					if attempt == 1 {
						h.click(practice)
					} else {
						onboardingFocusModalAction(t, h, practice)
						if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
							t.Fatal(err)
						}
					}
					if h.shows(settings.GuideTitle) || !h.shows(request.PreviewTitle) || !h.shows(onboardingSampleURL) {
						t.Fatal("practice did not replace the guide with the sample's pre-send preview")
					}
					if !onboardingRegressionShows(h, fmt.Sprintf(request.PreviewActiveEnvironment, onboardingPracticeEnv)) ||
						!onboardingRegressionShows(h, "Accept: "+request.PreviewMasked) {
						t.Fatal("preview does not show the preserved active environment and masked sample header")
					}
					onboardingAssertPreviewRedaction(t, h)

					sample := h.App.Active()
					expected := model.NewRequest(sample.Request.ID)
					expected.Name, expected.URL = sampleName, onboardingSampleURL
					expected.Headers = []model.KV{{Key: "Accept", Value: "application/json"}}
					if sample.Request.ID == "" || sample.Kind != requestTabKind || sample.CollectionID != "" ||
						!reflect.DeepEqual(sample.Request, expected) || sample.Sending || sample.Response != nil {
						t.Fatalf("practice did not open a normal unsaved standalone GET: %+v", sample.Request)
					}
					for _, prior := range originals {
						if sample.ID == prior.tabID || sample.Request.ID == prior.request.ID {
							t.Fatal("practice reused an existing tab or request identity")
						}
					}
					if got := onboardingActionDrafts(h); len(got) != len(originals)+1 || got[len(originals)].tabID != sample.ID || !reflect.DeepEqual(got[:len(originals)], originals) {
						t.Fatal("practice did not append exactly one draft while preserving prior drafts")
					}
					onboardingActionModalHit(t, h, request.PreviewClose)
					unchanged()
					if attempt == 1 {
						h.click(request.PreviewClose)
					} else {
						onboardingFocusModalAction(t, h, request.PreviewClose)
						if err := h.D.Key(gfx.KeyEscape, 0); err != nil {
							t.Fatal(err)
						}
					}
					if h.shows(request.PreviewTitle) || h.App.Active() != sample || !h.shows(onboardingSampleURL) {
						t.Fatal("closing the preview did not leave the sample in the editor")
					}
					editedURL := fmt.Sprintf("https://example.invalid/edited-%d", attempt)
					requestType(h, request.URL, editedURL, true)
					if !sample.Dirty || sample.Request.URL != editedURL {
						t.Fatal("editing the sample did not update its ordinary request draft")
					}
					originals = onboardingActionDrafts(h)
					unchanged()
				}
				envManage(h)
				if !h.shows(onboardingEnvDraftName) || !h.shows(h.App.S.Env.Unsaved) {
					t.Fatal("practice discarded the prior environment draft")
				}
				unchanged()
			})
		}
	}
}

func TestOnboardingImportUsesCanonicalReviewAndCancel(t *testing.T) {
	for _, language := range []string{"zh", "en"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(language+"/"+theme, func(t *testing.T) {
				h, unchanged := newOnboardingActionsHarness(t, language, theme, true)
				settings, importer := h.App.S.Settings, h.App.S.Import
				drafts, active := onboardingActionDrafts(h), h.App.Active()
				assertUnchanged := func() {
					t.Helper()
					unchanged()
					if h.App.Active() != active || !reflect.DeepEqual(onboardingActionDrafts(h), drafts) {
						t.Fatal("import entry, review or cancellation changed the active tab or prior drafts")
					}
				}
				h.click(settings.Title)
				h.click(settings.ReplayGuide)
				onboardingFocusModalAction(t, h, settings.GuideImport)
				if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
					t.Fatal(err)
				}
				if h.shows(settings.GuideTitle) || !h.shows(importer.ManualTitle) || !onboardingActionModalHit(t, h, importer.Review).Disabled {
					t.Fatal("guide import did not open the canonical dialog at its empty input step")
				}
				assertUnchanged()
				importTypeInto(h, `curl --insecure https://example.invalid/import-only`)
				if importSmokeHit(t, h, importer.Review).Disabled || h.App.importer.preview != nil || h.App.importer.parsing {
					t.Fatal("pasting did not await an explicit review")
				}
				assertUnchanged()
				reviewImportForTest(h)
				if h.App.importer.preview == nil || !onboardingActionModalHit(t, h, importer.Submit).Disabled {
					t.Fatal("a warned curl review must require acknowledgement before confirmation")
				}
				assertUnchanged()
				h.click(importer.Submit)
				assertUnchanged()
				h.click(importer.Acknowledge)
				if onboardingActionModalHit(t, h, importer.Submit).Disabled {
					t.Fatal("acknowledgement did not enable the separate confirmation step")
				}
				assertUnchanged()
				onboardingActionModalHit(t, h, importer.Cancel)
				onboardingFocusModalAction(t, h, importer.Cancel)
				if err := h.D.Key(gfx.KeyEnter, 0); err != nil {
					t.Fatal(err)
				}
				if h.shows(importer.ManualTitle) || h.shows(settings.GuideTitle) {
					t.Fatal("cancel left the importer or guide visible")
				}
				assertUnchanged()
			})
		}
	}
}

func newOnboardingActionsHarness(t *testing.T, language, theme string, completed bool) (*harness, func()) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(storage.EnvHome, dir)
	disk, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	saved := model.NewRequest("saved-request")
	saved.Name, saved.URL, saved.Auth.Type = "Stored request", "https://{{host}}/saved", model.AuthInherit
	col := model.Collection{ID: "existing-collection", Name: "Existing collection", Auth: model.Auth{Type: model.AuthBearer, Token: "private-collection-token"},
		Items: []model.Item{{ID: saved.ID, Name: saved.Name, Type: model.ItemRequest, Request: &saved}}}
	env := model.Environment{ID: "active-environment", Name: onboardingPracticeEnv, Variables: []model.KV{
		{Key: "host", Value: "saved.example.invalid"}, {Key: "token", Value: onboardingEnvSecret},
	}}
	settings := model.DefaultSettings()
	settings.Language, settings.Theme, settings.ActiveEnvID, settings.OnboardingCompleted = language, theme, env.ID, completed
	settings.HistoryLimit, settings.RequestTimeoutSec, settings.NoFollowRedirects = 500, 120, true
	config := syncConfig{Provider: "webdav", URL: "http://127.0.0.1:1", Auto: true}
	for _, err := range []error{
		disk.Collections().Save(col), disk.Environments().Save(env), disk.Settings().Save(settings),
		disk.History().Add(model.HistoryEntry{Request: saved, StatusCode: 204}),
		storage.WriteJSON(filepath.Join(dir, "sync-config.json"), config),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	disk, err = store.Open(dir)
	if err != nil || len(disk.LoadErrors()) != 0 {
		t.Fatal("could not reload fixture", err)
	}
	var senderCalls atomic.Int32
	svc := service.Services{
		Requests: &fake.Requests{Fn: func(context.Context, model.Request) (model.Response, error) {
			senderCalls.Add(1)
			return model.Response{}, fmt.Errorf("onboarding must not send")
		}},
		Collections: disk.Collections(), Environments: disk.Environments(),
		Settings: &onboardingSettingsProbe{SettingsService: disk.Settings()}, History: disk.History(),
	}
	h := newHarness(t, svc)
	h.typeInto(h.App.S.Request.URL, "https://example.invalid/standalone-draft")
	h.App.OpenRequest(saved, col.ID)
	requestRepaint(h)
	requestType(h, h.App.S.Request.URL, "https://example.invalid/"+onboardingDraftSecret, true)
	h.App.NewTab()
	if err := h.D.Resize(960, 600); err != nil {
		t.Fatal(err)
	}
	envManage(h)
	requestType(h, h.App.S.Env.Name, onboardingEnvDraftName, true)
	if draft := h.App.env.current(); draft == nil || draft.name != onboardingEnvDraftName || !draft.dirty() {
		t.Fatal("fixture did not establish an unsaved environment draft")
	}

	memory := syncing.Capture(svc)
	files := researchDisk(t, dir)
	syncState := &h.App.settings.sync
	syncState.backupPassword = "test-backup-password"
	fingerprint := syncFingerprint(syncing.Shared(svc))
	syncState.autoFingerprint = fingerprint
	return h, func() {
		t.Helper()
		h.pump()
		if calls := senderCalls.Load(); calls != 0 {
			t.Fatalf("onboarding invoked the HTTP sender %d times", calls)
		}
		if saves := svc.Settings.(*onboardingSettingsProbe).saves; saves != 0 {
			t.Fatalf("onboarding actions called Settings.Save %d times", saves)
		}
		if !reflect.DeepEqual(syncing.Capture(svc), memory) || !reflect.DeepEqual(researchDisk(t, dir), files) {
			t.Fatal("onboarding changed settings, history, collections, environments or sync files")
		}
		if syncState.config != config || syncState.autoFingerprint != fingerprint || syncState.job != nil || syncState.running {
			t.Fatal("onboarding changed the armed auto-sync state or launched sync work")
		}
	}
}

type onboardingActionDraft struct {
	tabID        int
	request      model.Request
	collectionID string
	dirty        bool
}

func onboardingActionDrafts(h *harness) []onboardingActionDraft {
	drafts := make([]onboardingActionDraft, 0, len(h.App.Tabs()))
	for _, tab := range h.App.Tabs() {
		drafts = append(drafts, onboardingActionDraft{tab.ID, tab.Request.Clone(), tab.CollectionID, tab.Dirty})
	}
	return drafts
}

func onboardingActionModalHit(t *testing.T, h *harness, label string) gfx.Hit {
	t.Helper()
	snapshot := h.D.Snapshot()
	for _, hit := range snapshot.Hits {
		if hit.Label == label && hit.Layer > 0 {
			bounds := hit.Bounds
			if bounds.Width <= 0 || bounds.Height <= 0 || bounds.X < 0 || bounds.Y < 0 ||
				bounds.X+bounds.Width > float32(snapshot.Width) || bounds.Y+bounds.Height > float32(snapshot.Height) {
				t.Fatalf("modal action %q is outside the %dx%d viewport: %+v", label, snapshot.Width, snapshot.Height, bounds)
			}
			return hit
		}
	}
	t.Fatalf("no modal action named %q; have %q", label, h.hitLabels())
	return gfx.Hit{}
}

func onboardingFocusModalAction(t *testing.T, h *harness, label string) {
	t.Helper()
	hit := onboardingActionModalHit(t, h, label)
	snapshot := h.D.Snapshot()
	for range len(snapshot.Hits) {
		if snapshot.Focused == hit.ID {
			return
		}
		if err := h.D.Key(gfx.KeyTab, 0); err != nil {
			t.Fatal(err)
		}
		snapshot = h.D.Snapshot()
	}
	t.Fatalf("Tab could not focus modal action %q", label)
}

func onboardingAssertPreviewRedaction(t *testing.T, h *harness) {
	t.Helper()
	var frame strings.Builder
	snapshot := h.D.Snapshot()
	for _, label := range snapshot.Labels {
		fmt.Fprintln(&frame, label.Value)
	}
	for _, hit := range snapshot.Hits {
		fmt.Fprintln(&frame, hit.Label, hit.Value, hit.Hint, hit.Description)
	}
	for _, hidden := range []string{onboardingEnvSecret, onboardingDraftSecret, "private-collection-token", "application/json"} {
		if strings.Contains(frame.String(), hidden) {
			t.Errorf("offline preview exposed %q in its painted or accessible content", hidden)
		}
	}
}
