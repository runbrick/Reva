package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func authMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func authFixture(t *testing.T) (*harness, model.Collection, model.Item, *Tab) {
	t.Helper()
	ui.MemoCheck(true)
	t.Cleanup(func() { ui.MemoCheck(false) })
	svc := fake.New()
	col, err := svc.Collections.Create("Private API")
	authMust(t, err)
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{shared}}"}
	col.Variables = []model.KV{{Key: "shared", Value: "collection-token"}}
	authMust(t, svc.Collections.Save(col))
	folder, err := svc.Collections.AddFolder(col.ID, "", "Shared folder")
	authMust(t, err)
	r := model.NewRequest("auth-request")
	r.Name, r.URL, r.Auth = "Inherited request", "https://example.test/items", model.Auth{Type: model.AuthInherit}
	_, err = svc.Collections.SaveRequest(col.ID, folder.ID, r)
	authMust(t, err)
	h := newHarness(t, svc)
	requestEnglish(h)
	h.click(r.Name)
	tab := h.App.Active()
	h.App.requestTab(tab).section = requestAuth
	requestRepaint(h)
	col, _ = svc.Collections.Get(col.ID)
	return h, col, folder, tab
}

func authAssertMasked(h *harness, field, secret string) {
	h.t.Helper()
	hit := requestField(h, field, 0)
	if !hit.Secure || hit.Value != "" {
		h.t.Fatalf("%s exposes secret through accessibility: %#v", field, hit)
	}
	for _, hit := range h.D.Snapshot().Hits {
		if strings.Contains(hit.Value, secret) || strings.Contains(hit.Label, secret) {
			h.t.Fatal("secret present in accessible value/name")
		}
	}
	if h.shows(secret) {
		h.t.Fatal("secret painted as readable text")
	}
}

func TestAuthEditorMaskingInactiveFieldsAndTabDirty(t *testing.T) {
	h, _, _, tab := authFixture(t)
	other := h.App.NewTab()
	h.App.active = 0
	tab.Request.Auth = model.Auth{Type: model.AuthBearer, Token: "token-private-marker", Username: "alice", Password: "password-private-marker", Key: "X-Key", Value: "key-private-marker"}
	tab.Dirty = false
	requestRepaint(h)
	authAssertMasked(h, h.App.S.Request.Token, "token-private-marker")
	h.click(h.App.S.Request.Basic)
	authAssertMasked(h, h.App.S.Request.Password, "password-private-marker")
	h.click(h.App.S.Request.APIKey)
	authAssertMasked(h, h.App.S.Request.APIKeyValue, "key-private-marker")
	h.click(h.App.S.Request.Bearer)
	if tab.Request.Auth.Token != "token-private-marker" || tab.Request.Auth.Password != "password-private-marker" || tab.Request.Auth.Value != "key-private-marker" || !tab.Dirty || other.Dirty {
		t.Fatalf("mode switch lost inactive fields or dirtied another tab: %#v", tab.Request.Auth)
	}
	tab.Dirty = false
	requestType(h, h.App.S.Request.Token, "edited", true)
	if tab.Request.Auth.Token != "edited" || !tab.Dirty || other.Dirty {
		t.Fatal("masked token edit did not remain scoped to its tab")
	}
}

func TestAuthLegacyViewAndLiveSourcePreview(t *testing.T) {
	h, col, folder, tab := authFixture(t)
	tab.Request.Auth, tab.Dirty = model.Auth{}, false
	requestRepaint(h)
	if tab.Request.Auth != (model.Auth{}) || tab.Dirty {
		t.Fatal("viewing old auth changed request")
	}
	collectionsOption(h, h.App.S.Request.AuthInherit)
	tab.Dirty = false
	if !h.shows(fmt.Sprintf(h.App.S.Request.AuthFromCollection, h.App.S.Request.Bearer, col.Name)) || h.shows("collection-token") {
		t.Fatal("collection provenance missing or secret leaked")
	}
	col.Items = cloneCollectionItems(col.Items)
	f := model.FindItem(col.Items, folder.ID)
	f.Auth, f.Name = model.Auth{Type: model.AuthBasic, Username: "private-user", Password: "private-password"}, "Renamed scope"
	authMust(t, h.Svc.Collections.Save(col))
	requestRepaint(h)
	if !h.shows(fmt.Sprintf(h.App.S.Request.AuthFromFolder, h.App.S.Request.Basic, f.Name)) || h.shows("private-password") || h.shows("private-user") || tab.Dirty {
		t.Fatal("preview not live, leaked credentials, or dirtied request")
	}
	f.Auth = model.Auth{Type: model.AuthNone}
	authMust(t, h.Svc.Collections.Save(col))
	requestRepaint(h)
	if !h.shows(h.App.S.Request.AuthParentNone) {
		t.Fatal("folder None barrier not explained")
	}
	collectionsContext(h, f.Name, h.App.S.Collections.Authentication)
	collectionsOption(h, h.App.S.Request.AuthInherit)
	if !h.shows(fmt.Sprintf(h.App.S.Request.AuthFromCollection, h.App.S.Request.Bearer, col.Name)) {
		t.Fatal("folder preview included itself rather than its parent")
	}
}

func TestAuthDialogCancelEscapeOutsideAndLatestTreeSave(t *testing.T) {
	h, col, folder, tab := authFixture(t)
	for _, closeBy := range []string{"cancel", "escape", "outside"} {
		collectionsContext(h, col.Name, h.App.S.Collections.Authentication)
		collectionsOption(h, h.App.S.Request.Basic)
		requestType(h, h.App.S.Request.Password, "unsaved-password", false)
		authAssertMasked(h, h.App.S.Request.Password, "unsaved-password")
		switch closeBy {
		case "cancel":
			h.click(h.App.S.Collections.Cancel)
		case "escape":
			authMust(t, h.D.Key(gfx.KeyEscape, 0))
		case "outside":
			// The rail control is outside the modal; its center tests backdrop dismissal.
			h.click(h.App.S.Shell.APIs)
		}
		h.pump()
		stored, _ := h.Svc.Collections.Get(col.ID)
		if h.App.collections.authOpen || stored.Auth != col.Auth || tab.Dirty {
			t.Fatalf("%s persisted draft or left modal open", closeBy)
		}
	}
	collectionsContext(h, folder.Name, h.App.S.Collections.Authentication)
	if h.App.collections.authDraft.Type != model.AuthInherit {
		t.Fatal("empty folder should edit as Inherit")
	}
	collectionsOption(h, h.App.S.Request.APIKey)
	requestType(h, h.App.S.Request.APIKeyName, "X-New", false)
	requestType(h, h.App.S.Request.APIKeyValue, "saved-key", false)
	latest, _ := h.Svc.Collections.Get(col.ID)
	latest.Items = cloneCollectionItems(latest.Items)
	latest.Name = "Independent rename"
	latest.Auth = model.Auth{Type: model.AuthBasic, Username: "independent"}
	latest.Variables = append(latest.Variables, model.KV{Key: "new", Value: "independent"})
	model.FindItem(latest.Items, tab.Request.ID).Request.URL = "https://example.test/independent"
	authMust(t, h.Svc.Collections.Save(latest))
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	stored, _ := h.Svc.Collections.Get(col.ID)
	want := latest
	want.Items = cloneCollectionItems(latest.Items)
	model.FindItem(want.Items, folder.ID).Auth = model.Auth{Type: model.AuthAPIKey, Key: "X-New", Value: "saved-key", In: model.APIKeyInHeader}
	if h.App.collections.authOpen || !reflect.DeepEqual(stored, want) || tab.Dirty {
		t.Fatal("auth save lost latest independent edits or changed request draft")
	}
	collectionsContext(h, stored.Name, h.App.S.Collections.Authentication)
	collectionsOption(h, h.App.S.Request.None)
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	stored, _ = h.Svc.Collections.Get(col.ID)
	if stored.Auth.Type != model.AuthNone || model.FindItem(stored.Items, folder.ID).Auth != model.FindItem(want.Items, folder.ID).Auth {
		t.Fatal("root auth save changed folder auth")
	}
}

type authFailingCollections struct {
	service.CollectionService
	fail   bool
	writes int
}

func (s *authFailingCollections) Save(col model.Collection) error {
	s.writes++
	if s.fail {
		return errors.New("credential-private-backend-error")
	}
	return s.CollectionService.Save(col)
}

func TestAuthDialogFailureRetryAndChangedTarget(t *testing.T) {
	h, col, folder, _ := authFixture(t)
	failing := &authFailingCollections{CollectionService: h.Svc.Collections, fail: true}
	h.App.Svc.Collections, h.Svc.Collections = failing, failing
	collectionsContext(h, folder.Name, h.App.S.Collections.Authentication)
	collectionsOption(h, h.App.S.Request.Bearer)
	requestType(h, h.App.S.Request.Token, "draft-secret", false)
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	stored, _ := failing.Get(col.ID)
	if !h.App.collections.authOpen || h.App.collections.authDraft.Token != "draft-secret" || model.FindItem(stored.Items, folder.ID).Auth != (model.Auth{}) || !h.shows(h.App.S.Collections.AuthSaveFailed) || h.shows("credential-private-backend-error") {
		t.Fatal("failed save leaked error, mutated shared slices, or discarded draft")
	}
	failing.fail = false
	collectionsOption(h, h.App.S.Collections.Save)
	h.pump()
	stored, _ = failing.Get(col.ID)
	if h.App.collections.authOpen || model.FindItem(stored.Items, folder.ID).Auth.Token != "draft-secret" || failing.writes != 2 {
		t.Fatal("retry did not persist retained draft")
	}
	for _, changed := range []string{"type", "deleted", "collection-deleted"} {
		t.Run(changed, func(t *testing.T) {
			// Recreate the original tree for each stale-target case.
			authMust(t, failing.CollectionService.Save(col))
			h.App.collectionAuthDialog(collectionNode{collectionID: col.ID, id: folder.ID, name: folder.Name})
			h.App.collections.authDraft = model.Auth{Type: model.AuthBasic, Username: "draft-user"}
			latest, _ := failing.Get(col.ID)
			latest.Items = cloneCollectionItems(latest.Items)
			switch changed {
			case "type":
				model.FindItem(latest.Items, folder.ID).Type = model.ItemRequest
				authMust(t, failing.CollectionService.Save(latest))
			case "deleted":
				authMust(t, failing.DeleteItem(col.ID, folder.ID))
			case "collection-deleted":
				authMust(t, failing.Delete(col.ID))
			}
			writes := failing.writes
			h.App.saveCollectionAuth()
			if !h.App.collections.authOpen || h.App.collections.authDraft.Username != "draft-user" || h.App.collections.authError != h.App.S.Collections.AuthTargetMissing || failing.writes != writes {
				t.Fatal("stale target wrote data or lost draft")
			}
			h.App.collections.authOpen = false
		})
	}
}

func TestAuthCreationScopes(t *testing.T) {
	h, col, folder, _ := authFixture(t)
	node := collectionNode{collectionID: col.ID, id: folder.ID, name: folder.Name}
	for _, graphQL := range []bool{false, true} {
		h.App.createCollectionRequest(node, graphQL)
		tab := h.App.Active()
		stored, _ := h.Svc.Collections.Get(col.ID)
		if tab.Request.Auth.Type != model.AuthInherit || model.FindItem(model.FindItem(stored.Items, folder.ID).Items, tab.Request.ID) == nil {
			t.Fatal("new collection HTTP/GraphQL request did not inherit in selected folder")
		}
	}
	h.App.newWebSocket(node)
	if h.App.Active().Request.Auth.Type != model.AuthNone {
		t.Fatal("WebSocket opted into HTTP inheritance")
	}
	if h.App.NewTab().Request.Auth.Type != model.AuthNone {
		t.Fatal("standalone HTTP request opted into inheritance")
	}
	h.App.newGraphQLTab()
	if h.App.Active().Request.Auth.Type != model.AuthNone {
		t.Fatal("standalone GraphQL request opted into inheritance")
	}
}

func TestAuthPassedTabVariablesScriptPrecedenceAndCopyNoMutation(t *testing.T) {
	h, col, _, tab := authFixture(t)
	env, err := h.Svc.Environments.Create("Active")
	authMust(t, err)
	env.Variables = []model.KV{{Key: "shared", Value: "environment-token"}, {Key: "disabled", Value: "ignored", Disabled: true}}
	authMust(t, h.Svc.Environments.Save(env))
	settings := h.Svc.Settings.Get()
	settings.ActiveEnvID = env.ID
	authMust(t, h.Svc.Settings.Save(settings))
	tab.Request.PreScript = `pm.variables.set("shared", "script-token"); pm.request.headers.upsert({key:"X-Script",value:"yes"});`
	other, err := h.Svc.Collections.Create("Other active collection")
	authMust(t, err)
	other.Variables = []model.KV{{Key: "only", Value: "wrong"}}
	authMust(t, h.Svc.Collections.Save(other))
	col.Variables = append(col.Variables, model.KV{Key: "only", Value: "passed-tab"})
	authMust(t, h.Svc.Collections.Save(col))
	tab.Request.Headers = []model.KV{{Key: "X-Only", Value: "{{only}}"}}
	active := h.App.NewTab()
	active.CollectionID = other.ID
	before := tab.Request.Clone()
	command, err := h.App.requestCURLFor(tab)
	if err != nil || !strings.Contains(command, "Authorization: Bearer environment-token") || !strings.Contains(command, "X-Only: passed-tab") || strings.Contains(command, "X-Script") || !reflect.DeepEqual(tab.Request, before) || tab.Dirty || active.Dirty {
		t.Fatalf("inactive-tab cURL mutated draft or ran script/wrong scope: %s %v", command, err)
	}
	requests := requestCapture(h)
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("inactive-tab response", func() bool { return !tab.Sending })
	var got model.Request
	select {
	case got = <-requests:
	default:
		t.Fatal("inactive-tab send did not reach service")
	}
	if got.Auth.Token != "script-token" || got.Headers[0].Value != "passed-tab" || len(got.Headers) != 2 || got.Headers[1].Key != "X-Script" || !reflect.DeepEqual(tab.Request, before) || tab.Dirty {
		t.Fatalf("passed-tab/script precedence: %#v", got)
	}
	// Without an environment override, the passed collection supplies the token.
	settings.ActiveEnvID = ""
	authMust(t, h.Svc.Settings.Save(settings))
	command, err = h.App.requestCURLFor(tab)
	if err != nil || !strings.Contains(command, "Bearer collection-token") {
		t.Fatalf("passed-tab collection scope lost: %s %v", command, err)
	}
}

func TestAuthSendStartHistorySnapshotAndReplayWithoutParent(t *testing.T) {
	h, col, folder, tab := authFixture(t)
	started := make(chan model.Request, 1)
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	h.Svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, req model.Request) (model.Response, error) {
		started <- req
		select {
		case <-release:
			return model.Response{StatusCode: 200, Status: "200 OK"}, nil
		case <-ctx.Done():
			return model.Response{}, ctx.Err()
		}
	}
	h.App.Send(tab)
	requestRepaint(h)
	h.waitFor("in-flight request", func() bool { return len(started) > 0 })
	if sent := <-started; sent.Auth.Token != "collection-token" || tab.job.req.Auth != col.Auth {
		t.Fatal("send did not resolve and snapshot unexpanded parent at start")
	}
	col.Items = cloneCollectionItems(col.Items)
	model.FindItem(col.Items, folder.ID).Auth = model.Auth{Type: model.AuthBearer, Token: "later-parent"}
	authMust(t, h.Svc.Collections.Save(col))
	tab.Request.Auth = model.Auth{Type: model.AuthBasic, Username: "later-editor"}
	close(release)
	h.waitFor("completed history", func() bool { return !tab.Sending })
	entries := h.Svc.History.List()
	if len(entries) != 1 || entries[0].Request.Auth != col.Auth {
		t.Fatalf("history lost send-start unexpanded auth: %#v", entries)
	}
	authMust(t, h.Svc.Collections.Delete(col.ID))
	h.App.CloseTab(h.App.active)
	h.App.openHistory(entries[0], false)
	replay := h.App.Active()
	if replay.CollectionID != "" || replay.Request.Auth != col.Auth {
		t.Fatal("history replay depends on deleted parent")
	}
	requests := requestCapture(h)
	h.App.Send(replay)
	requestRepaint(h)
	h.waitFor("history replay response", func() bool { return !replay.Sending })
	select {
	case got := <-requests:
		if got.Auth != col.Auth {
			t.Fatalf("replay changed stored placeholder auth: %#v", got.Auth)
		}
	default:
		t.Fatal("history replay did not reach service")
	}
}

func TestAuthLiveParentSendAndSelectedUnresolvedVariables(t *testing.T) {
	h, col, folder, tab := authFixture(t)
	tab.Request.Auth.Token = "{{inactive_request_token}}"
	col.Items = cloneCollectionItems(col.Items)
	f := model.FindItem(col.Items, folder.ID)
	for _, cfg := range []model.Auth{
		{Type: model.AuthBearer, Token: "{{shared}}", Password: "{{inactive_password}}"},
		{Type: model.AuthBasic, Username: "{{missing_user}}", Password: "{{missing_password}}", Token: "{{inactive_token}}"},
		{Type: model.AuthNone},
	} {
		f.Auth = cfg
		authMust(t, h.Svc.Collections.Save(col))
		requestRepaint(h)
		wantMissing := []string(nil)
		if cfg.Type == model.AuthBasic {
			wantMissing = []string{"missing_user", "missing_password"}
		}
		if got := h.App.UnresolvedVars(); !slices.Equal(got, wantMissing) {
			t.Fatalf("unresolved warnings used inactive fields: %v, want %v", got, wantMissing)
		}
		requests := requestCapture(h)
		h.App.Send(tab)
		requestRepaint(h)
		h.waitFor("live parent send", func() bool { return !tab.Sending })
		var sent model.Request
		select {
		case sent = <-requests:
		default:
			t.Fatal("send never reached service")
		}
		want := cfg
		if want.Type == model.AuthBearer {
			want.Token = "collection-token"
		}
		if sent.Auth != want || tab.Request.Auth.Type != model.AuthInherit || tab.Dirty {
			t.Fatalf("live parent send mutated draft or used stale config: %#v, want %#v", sent.Auth, want)
		}
	}
}

func TestAuthCURLLiveParentSharedOverridesAndQueryMultiset(t *testing.T) {
	h, col, folder, tab := authFixture(t)
	// A read of the current parent must observe each edit without reopening the request.
	col.Items = cloneCollectionItems(col.Items)
	f := model.FindItem(col.Items, folder.ID)
	f.Auth = model.Auth{Type: model.AuthAPIKey, Key: "key", Value: "{{shared}}", In: model.APIKeyInQuery}
	authMust(t, h.Svc.Collections.Save(col))
	tab.Request.URL = "https://example.test?key=old&key=older&x=a&x=a#anchor"
	tab.Request.Params = []model.KV{{Key: "key", Value: "old"}, {Key: "x", Value: "a"}, {Key: "x", Value: "a"}, {Key: "x", Value: "b"}, {Key: "disabled", Value: "off", Disabled: true}}
	before := tab.Request.Clone()
	command, err := h.App.requestCURLFor(tab)
	if err != nil || !strings.Contains(command, "?key=collection-token&x=a&x=a&x=b#anchor") || !reflect.DeepEqual(tab.Request, before) || tab.Dirty {
		t.Fatalf("cURL query multiset/replacement: %s %v", command, err)
	}
	f.Auth = model.Auth{Type: model.AuthBearer, Token: "new-parent"}
	authMust(t, h.Svc.Collections.Save(col))
	tab.Request.Headers = []model.KV{{Key: "authorization"}, {Key: "AUTHORIZATION", Value: "explicit"}, {Key: "Authorization", Value: "disabled", Disabled: true}}
	command, err = h.App.requestCURLFor(tab)
	if err != nil || !strings.Contains(command, "'Authorization;'") || !strings.Contains(command, "'Authorization: explicit'") || strings.Contains(command, "new-parent") || strings.Contains(command, "disabled") {
		t.Fatalf("cURL header override: %s %v", command, err)
	}
	tab.Request.Headers = nil
	command, err = h.App.requestCURLFor(tab)
	if err != nil || !strings.Contains(command, "Bearer new-parent") {
		t.Fatalf("cURL parent edit not live: %s %v", command, err)
	}
	f.Auth = model.Auth{Type: model.AuthNone}
	authMust(t, h.Svc.Collections.Save(col))
	command, err = h.App.requestCURLFor(tab)
	if err != nil || strings.Contains(command, "Authorization") {
		t.Fatalf("cURL ignored None barrier: %s %v", command, err)
	}
}

func TestAuthFailureLeavesClipboardNetworkAndPriorDownloadUntouched(t *testing.T) {
	for _, failure := range []string{"missing-item", "missing-collection", "invalid-parent", "expanded-invalid-parent", "invalid-query"} {
		t.Run(failure, func(t *testing.T) {
			h, col, _, tab := authFixture(t)
			var calls atomic.Int32
			h.Svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
				calls.Add(1)
				return model.Response{}, nil
			}
			switch failure {
			case "missing-item":
				authMust(t, h.Svc.Collections.DeleteItem(col.ID, tab.Request.ID))
			case "missing-collection":
				authMust(t, h.Svc.Collections.Delete(col.ID))
			case "invalid-parent":
				col.Auth = model.Auth{Type: "unsupported", Token: "private-marker"}
				authMust(t, h.Svc.Collections.Save(col))
			case "expanded-invalid-parent":
				col.Variables = []model.KV{{Key: "shared", Value: "private-marker\r\nInjected: yes"}}
				authMust(t, h.Svc.Collections.Save(col))
			case "invalid-query":
				tab.Request.URL = "https://example.test?bad=%zz"
			}
			requestRepaint(h)
			h.D.SetClipboard("existing clipboard")
			h.click(h.App.S.Request.SendMenu)
			h.click(h.App.S.Request.CopyCURL)
			if h.D.Clipboard() != "existing clipboard" || tab.Sending || tab.Dirty || calls.Load() != 0 || h.shows("private-marker") {
				t.Fatal("failed cURL changed clipboard/draft or sent/leaked auth")
			}
			if failure == "invalid-query" {
				return
			} // URL validation belongs to HTTP, not the UI's auth gate.
			prior := &model.Response{StatusCode: 200, Body: []byte("prior response")}
			tab.Response = prior
			path := filepath.Join(t.TempDir(), "download.bin")
			authMust(t, h.D.QueueSaveFile(path, nil))
			h.click(h.App.S.Request.SendMenu)
			h.click(h.App.S.Request.SendDownload)
			h.pump()
			if tab.Sending || tab.job != nil || tab.Response != prior || tab.Err == "" || h.App.requestTab(tab).downloadJob != nil || calls.Load() != 0 {
				t.Fatal("auth failure started a job/download or discarded previous response")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("auth failure downloaded previous response: %v", err)
			}
		})
	}
}

func TestAuthDialogAndEditorLanguagesThemesLayout(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(lang+"/"+theme, func(t *testing.T) {
				h, col, folder, tab := authFixture(t)
				settings := h.Svc.Settings.Get()
				settings.Language, settings.Theme = lang, theme
				h.App.ApplySettings(settings)
				requestRepaint(h)
				if !h.shows(fmt.Sprintf(h.App.S.Request.AuthFromCollection, h.App.S.Request.Bearer, col.Name)) || h.App.Dark() != (theme == "dark") {
					t.Fatal("localized preview/theme missing")
				}
				// Check auth labels in the real page and modal, excluding unrelated shell labels.
				check := func() {
					preview := fmt.Sprintf(h.App.S.Request.AuthFromCollection, h.App.S.Request.Bearer, col.Name)
					labels := &h.App.S.Request
					authLabels := []string{labels.AuthInherit, labels.None, labels.Bearer, labels.Basic, labels.APIKey,
						labels.Token, labels.Username, labels.Password, labels.APIKeyName, labels.APIKeyValue,
						labels.InHeader, labels.InQuery, labels.AuthParentNone, preview}
					for _, issue := range headless.CheckLayout(h.D.Snapshot()) {
						if slices.Contains(authLabels, issue.Text) {
							t.Errorf("auth layout %s: %q (%s)", issue.Kind, issue.Text, issue.Detail)
						}
					}
				}
				for _, kind := range inheritedAuthTypes {
					tab.Request.Auth = authBenchmarkConfig(kind)
					requestRepaint(h)
					check()
				}
				tab.Request.Auth = model.Auth{Type: model.AuthInherit}
				for _, target := range []collectionNode{{collectionID: col.ID, name: col.Name}, {collectionID: col.ID, id: folder.ID, name: folder.Name}} {
					h.App.collectionAuthDialog(target)
					kinds := model.AuthTypes
					if target.id != "" {
						kinds = inheritedAuthTypes
					}
					for _, kind := range kinds {
						h.App.collections.authDraft = authBenchmarkConfig(kind)
						requestRepaint(h)
						if !h.shows(h.App.S.Collections.Authentication) || !h.App.collections.authOpen || tab.Dirty {
							t.Fatal("localized auth modal missing/changed draft")
						}
						check()
					}
					h.App.collections.authOpen = false
				}
			})
		}
	}
}
