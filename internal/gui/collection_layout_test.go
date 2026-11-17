package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func collectionLayoutFixture(t *testing.T) (*harness, model.Collection) {
	t.Helper()
	svc := fake.New()
	col, err := svc.Collections.Create("Payments (2026)")
	authMust(t, err)
	request := func(id string, method model.Method, graphql bool) model.Item {
		r := model.NewRequest(id)
		r.Name, r.Method = id, method
		if graphql {
			initGraphQLBody(&r)
		}
		return model.Item{ID: id, Name: r.Name, Type: model.ItemRequest, Request: &r}
	}
	col.Items = []model.Item{
		request("root-http", model.MethodGet, false),
		{ID: "folder", Name: "Invoices", Type: model.ItemFolder, Items: []model.Item{
			request("graphql", model.MethodPost, true),
			request("http", model.MethodOptions, false),
			{ID: "nested", Name: "Events", Type: model.ItemFolder, Items: []model.Item{request("nested-ws", webSocketMethod, false)}},
			{ID: "empty", Name: "Empty folder", Type: model.ItemFolder},
		}},
		request("root-ws", webSocketMethod, false),
		{ID: "root-empty", Name: "Empty root folder", Type: model.ItemFolder},
	}
	col.Variables = []model.KV{{Key: "host", Value: "saved.test"}}
	col.Auth = model.Auth{Type: model.AuthBearer, Token: "{{host}}"}
	authMust(t, svc.Collections.Save(col))
	h := newHarness(t, svc)
	return h, col
}

func collectionLayoutText(t *testing.T, h *harness, text string, workbench bool) gfx.Rect {
	t.Helper()
	for _, label := range h.D.Snapshot().Texts {
		if (label.Value == text || len(label.Value) > 20 && strings.HasPrefix(text, label.Value)) && (!workbench || label.Bounds.X >= RailWidth+h.App.sideSplit.Pos) {
			return label.Bounds
		}
	}
	t.Fatalf("text %q missing", text)
	return gfx.Rect{}
}

func collectionLayoutHit(t *testing.T, h *harness, text string) gfx.Hit {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == text && !hit.Container {
			return hit
		}
	}
	t.Fatalf("control %q missing", text)
	return gfx.Hit{}
}

func TestCollectionOverviewGearCountsReuseAndDisclosure(t *testing.T) {
	h, col := collectionLayoutFixture(t)
	s := h.App.S.Collections
	gear := fmt.Sprintf(s.SettingsFor, col.Name)
	h.click(gear)
	tab := h.App.Active()
	settingsAssertWorkbench(h)
	if tab.TargetID != "" || tab.CollectionID != col.ID || settingsDraft(h).section != collectionSettingsGeneral {
		t.Fatal("direct gear did not select collection overview")
	}
	collectionLayoutText(t, h, fmt.Sprintf(s.RequestCount, 3), true)
	collectionLayoutText(t, h, fmt.Sprintf(s.RequestCount, 2), true)
	collectionLayoutText(t, h, fmt.Sprintf(s.SidebarCount, 5), false)
	bareTitle := false
	for _, label := range h.D.Snapshot().Labels {
		if label.Bounds.Y >= TitlebarHeight && label.Bounds.Y < TitlebarHeight+TabBarHeight && label.Bounds.X > RailWidth+SidebarWidth {
			if label.Value == h.App.collectionSettingsTitle(tab) {
				t.Fatal("tab strip paints the long settings suffix")
			}
			if label.Value == col.Name {
				bareTitle = true
			}
		}
	}
	if !bareTitle {
		t.Fatal("tab strip did not paint the saved bare collection name")
	}
	// The name remains unmodified; parentheses belong to the name, not the count.
	if h.App.collectionSettingsName(tab) != "Payments (2026)" {
		t.Fatal("count changed saved collection name")
	}
	for i, n := range h.App.collections.visible {
		if n.id == "" && n.collectionID == col.ID {
			if !h.App.collections.node(i).Selected {
				t.Fatal("root overview target not highlighted")
			}
			id := h.App.collections.tree.DisclosureID(i)
			for _, hit := range h.D.Snapshot().Hits {
				if hit.ID == id {
					b := hit.Bounds
					authMust(t, h.D.Click(b.X+b.Width/2, b.Y+b.Height/2))
					break
				}
			}
			break
		}
	}
	if len(h.App.collections.visible) != 1 || h.App.Active() != tab {
		t.Fatal("disclosure did not collapse the tree independently of settings")
	}
	collectionsReplace(h, s.Name, "Retained draft")
	h.click(gear)
	if len(h.App.Tabs()) != 2 || h.App.Active() != tab || settingsDraft(h).name != "Retained draft" {
		t.Fatal("gear reopened a second tab, selected a request or reset its draft")
	}
	collectionsContext(h, col.Name, s.Authentication)
	if h.App.Active() != tab || settingsDraft(h).section != collectionSettingsAuthentication {
		t.Fatal("context shortcut did not route to retained settings tab")
	}
	h.click(gear)
	if settingsDraft(h).section != collectionSettingsGeneral || settingsDraft(h).name != "Retained draft" {
		t.Fatal("gear did not return retained draft to overview")
	}
	settingsAssertWorkbench(h)
}

func TestCollectionOverviewFolderActionsAndSavedExport(t *testing.T) {
	h, col := collectionLayoutFixture(t)
	var sends atomic.Int32
	h.Svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		sends.Add(1)
		return model.Response{}, nil
	}
	s := h.App.S.Collections
	h.click(fmt.Sprintf(s.SettingsFor, "Invoices"))
	folderTab, folderDraft := h.App.Active(), settingsDraft(h)
	collectionLayoutText(t, h, fmt.Sprintf(s.RequestCount, 2), true)
	collectionLayoutText(t, h, fmt.Sprintf(s.RequestCount, 1), true)
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == s.Export || hit.Label == s.Variables {
			t.Fatal("folder offered collection-only configuration/export")
		}
	}
	for _, action := range []string{s.NewRequest, s.NewWebSocket} {
		h.App.active = h.App.requestTabIndex(folderTab.ID)
		requestRepaint(h)
		h.click(action)
		created := h.App.Active()
		stored, _ := h.Svc.Collections.Get(col.ID)
		folder := model.FindItem(stored.Items, "folder")
		if created.Kind != requestTabKind || model.FindItem(folder.Items, created.Request.ID) == nil || created.CollectionID != col.ID {
			t.Fatal("overview action did not create a real request in the selected folder")
		}
		if (action == s.NewWebSocket) != isWebSocket(created.Request) || h.App.collections.settings[folderTab.ID] != folderDraft {
			t.Fatal("creation lost settings draft or chose wrong protocol")
		}
	}
	h.App.active = h.App.requestTabIndex(folderTab.ID)
	requestRepaint(h)
	h.click(s.NewFolder)
	if !h.App.collections.dialogOpen || h.App.collections.target.id != "folder" {
		t.Fatal("new-folder action ignored overview target")
	}
	collectionsOption(h, s.Name)
	authMust(t, h.D.Type("Created child"))
	h.click(s.Create)
	stored, _ := h.Svc.Collections.Get(col.ID)
	folder := model.FindItem(stored.Items, "folder")
	if len(folder.Items) != 7 || folder.Items[6].Name != "Created child" || folder.Items[6].Type != model.ItemFolder {
		t.Fatal("folder action did not persist inside the configured folder")
	}
	h.click(fmt.Sprintf(s.SettingsFor, col.Name))
	collectionsReplace(h, s.Name, "Unsaved export name")
	path := filepath.Join(t.TempDir(), "collection.json")
	authMust(t, h.D.QueueSaveFile(path, nil))
	h.click(s.Export)
	data, err := os.ReadFile(path)
	authMust(t, err)
	var exported model.Collection
	authMust(t, json.Unmarshal(data, &exported))
	stored, _ = h.Svc.Collections.Get(col.ID)
	if !reflect.DeepEqual(exported, stored) || exported.Name != col.Name || !h.App.Active().Dirty {
		t.Fatal("native export did not use the saved collection or persisted the draft")
	}
	if sends.Load() != 0 || len(h.Svc.History.List()) != 0 {
		t.Fatal("overview actions executed requests")
	}
	if h.App.Active().Kind != collectionSettingsTabKind || h.App.collections.dialogOpen || h.App.collections.saveOpen {
		t.Fatal("export changed configuration target or left a dialog open")
	}
}

func TestCollectionOverviewDuplicateNamesHaveDistinctGears(t *testing.T) {
	h, col := collectionLayoutFixture(t)
	_, err := h.Svc.Collections.Create(col.Name)
	authMust(t, err)
	requestRepaint(h)
	seen := make(map[string]bool)
	for _, n := range h.App.collections.visible {
		if n.branch {
			if seen[n.settings] || !strings.Contains(n.settings, n.name) {
				t.Fatal("gear names are ambiguous or omit the display name")
			}
			seen[n.settings] = true
			collectionLayoutHit(t, h, n.settings)
		}
	}
}

func TestCollectionOverviewResponsiveGeometry(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		for _, mode := range []string{"light", "dark"} {
			for _, size := range [][2]int{{960, 600}, {1440, 900}, {1800, 1000}} {
				t.Run(fmt.Sprintf("%s/%s/%dx%d", lang, mode, size[0], size[1]), func(t *testing.T) {
					h, col := collectionLayoutFixture(t)
					settings := h.Svc.Settings.Get()
					settings.Language, settings.Theme = lang, mode
					h.App.ApplySettings(settings)
					tab := h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
					d := settingsDraft(h)
					d.name = "A retained draft with a long display name"
					authMust(t, h.D.Resize(size[0], size[1]))
					authMust(t, h.D.Resize(size[0], size[1]))
					s := h.App.S.Collections
					snapshot := h.D.Snapshot()
					api := collectionLayoutText(t, h, s.APISection, true)
					guide := collectionLayoutText(t, h, s.GuideSection, true)
					nav := collectionLayoutHit(t, h, s.General)
					name := collectionLayoutHit(t, h, s.Name)
					save := collectionLayoutHit(t, h, s.Save)
					discard := collectionLayoutHit(t, h, s.DiscardReload)
					if save.Bounds.Y != discard.Bounds.Y || save.Bounds.Y > nav.Bounds.Y || name.Bounds.Y > nav.Bounds.Y || name.Bounds.Width < Space6 {
						t.Fatal("name/save/discard do not share a usable compact heading")
					}
					for _, hit := range []gfx.Hit{name, save, discard, nav, collectionLayoutHit(t, h, s.Variables), collectionLayoutHit(t, h, s.Authentication)} {
						b := hit.Bounds
						if b.X < RailWidth+220 || b.X+b.Width > float32(size[0])+1 || b.Y+b.Height > float32(size[1]) || b.Width <= 0 {
							t.Fatalf("heading/navigation clipped: %q %+v", hit.Label, b)
						}
					}
					if nav.Role != gfx.RoleTab || !nav.Selected {
						t.Fatal("overview is not an active tab")
					}
					underline := false
					for _, p := range snapshot.Primitives {
						if p.Color == h.App.theme.Primary && p.Bounds.Height == Space1/2 && p.Bounds.X == nav.Bounds.X && p.Bounds.Width == nav.Bounds.Width && p.Bounds.Y == nav.Bounds.Y+nav.Bounds.Height-Space1/2 {
							underline = true
						}
					}
					if !underline || api.Y < nav.Bounds.Y+nav.Bounds.Height || api.Y > nav.Bounds.Y+nav.Bounds.Height+Space6 {
						t.Fatalf("underline missing or body has a vertical gap: nav %+v API %+v", nav.Bounds, api)
					}
					if size[0] >= 1440 {
						if api.Y != guide.Y || guide.X <= api.X+Space6 || guide.X >= float32(size[0])-Space6 {
							t.Fatalf("API and guide are not simultaneous columns: %+v %+v", api, guide)
						}
						intro := collectionLayoutText(t, h, s.GuideIntro, true)
						if intro.Width < 300 || intro.Width < api.Width-Space1 || intro.Width > api.Width+Space1 {
							t.Fatalf("unbalanced columns: guide %+v", intro)
						}
					} else {
						if guide.X != api.X || guide.Y <= api.Y+Space6 {
							t.Fatalf("narrow overview did not stack: %+v %+v", api, guide)
						}
						authMust(t, h.D.Scroll(gfx.ScrollEvent{X: api.X + Space6, Y: float32(size[1]) - Space6, DeltaY: 2000, Precise: true}))
						if !h.shows(s.GuideEditingTitle) {
							t.Fatal("stacked guide cannot be reached by scrolling")
						}
					}
					settingsAssertWorkbench(h)
					if h.App.Active() != tab || d.name != "A retained draft with a long display name" {
						t.Fatal("resize changed target or lost name draft")
					}
				})
			}
		}
	}
}

func TestCollectionOverviewSidebarResizeAndIndependentScroll(t *testing.T) {
	h, col := collectionLayoutFixture(t)
	settings := h.Svc.Settings.Get()
	settings.Language = "en"
	h.App.ApplySettings(settings)
	h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
	authMust(t, h.D.Resize(1440, 600))
	s := h.App.S.Collections
	api := collectionLayoutText(t, h, s.APISection, true)
	guide := collectionLayoutText(t, h, s.GuideSection, true)
	authMust(t, h.D.Scroll(gfx.ScrollEvent{X: guide.X + Space6, Y: 500, DeltaY: 2000, Precise: true}))
	if got := collectionLayoutText(t, h, s.APISection, true); got.Y != api.Y || !h.shows(s.GuideEditingTitle) {
		t.Fatal("scrolling the guide moved the API column or failed to reveal its last section")
	}
	h.App.sideSplit.Pos = 650
	authMust(t, h.D.Resize(1440, 600))
	authMust(t, h.D.Resize(1440, 600))
	api = collectionLayoutText(t, h, s.APISection, true)
	guide = collectionLayoutText(t, h, s.GuideSection, true)
	if api.X != guide.X || guide.Y <= api.Y {
		t.Fatal("overview did not stack when the resized sidebar reduced available workbench width")
	}
}

// Optional review artifacts use Viva's real offscreen renderer, never a window.
// Run with REVA_COLLECTION_CAPTURE_DIR pointing outside the repository.
func TestCollectionOverviewHeadlessCapture(t *testing.T) {
	dir := os.Getenv("REVA_COLLECTION_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set REVA_COLLECTION_CAPTURE_DIR to capture the real headless page")
	}
	authMust(t, os.MkdirAll(dir, 0700))
	for _, variant := range []struct {
		lang, theme   string
		width, height int
	}{{"zh", "light", 1800, 1000}, {"en", "dark", 1440, 900}, {"en", "light", 960, 600}} {
		h, col := collectionLayoutFixture(t)
		settings := h.Svc.Settings.Get()
		settings.Language, settings.Theme = variant.lang, variant.theme
		h.App.ApplySettings(settings)
		h.App.openCollectionSettings(collectionNode{collectionID: col.ID}, collectionSettingsGeneral)
		page := ui.NewPage(ui.PageConfig{View: h.App.Page, Theme: &h.App.theme})
		session, err := headless.Open(page.Config(app.Config{Width: variant.width, Height: variant.height}), nil, headless.Options{Scale: 1})
		authMust(t, err)
		path := filepath.Join(dir, fmt.Sprintf("collection-%s-%s-%dx%d.png", variant.lang, variant.theme, variant.width, variant.height))
		err = session.Screenshot(path)
		session.Close()
		authMust(t, err)
		t.Log(path)
	}
}
