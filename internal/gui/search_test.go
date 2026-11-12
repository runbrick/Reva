package gui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/runbrick/reqo/internal/i18n"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func searchKey(t *testing.T, h *harness, key gfx.Key) {
	t.Helper()
	if err := h.D.Key(key, 0); err != nil {
		t.Fatal(err)
	}
	h.pump()
}

func searchOpen(t *testing.T, h *harness) {
	t.Helper()
	if handled, err := h.D.ShortcutChord(ui.Primary, 'k'); err != nil || !handled {
		t.Fatalf("Primary+K: handled=%v, error=%v", handled, err)
	}
	h.pump()
	if !h.App.search.open {
		t.Fatal("Primary+K did not open search")
	}
}

func searchType(t *testing.T, h *harness, query string) {
	t.Helper()
	if err := h.D.Type(query); err != nil {
		t.Fatal(err)
	}
	h.pump()
}

func searchHasHit(h *harness, name string) bool {
	inside := false
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Search.Field {
			inside = true
		}
		if inside && hit.Label == name {
			return true
		}
	}
	return false
}

func searchFixture(t *testing.T) service.Services {
	t.Helper()
	svc := fake.New()
	col, err := svc.Collections.Create("Commerce")
	if err != nil {
		t.Fatal(err)
	}
	folder, err := svc.Collections.AddFolder(col.ID, "", "Customers")
	if err != nil {
		t.Fatal(err)
	}
	r := model.NewRequest("request-search")
	r.Name, r.URL = "Get accounts", "https://api.test/accounts"
	if _, err := svc.Collections.SaveRequest(col.ID, folder.ID, r); err != nil {
		t.Fatal(err)
	}
	r.ID, r.Name, r.URL = "history-search", "Recent audit", "https://audit.test/events"
	if err := svc.History.Add(model.HistoryEntry{Request: r}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Environments.Create("Staging"); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestSearchShortcutAutofocusAndEscape(t *testing.T) {
	for _, page := range Pages {
		t.Run(fmt.Sprint(page), func(t *testing.T) {
			h := newHarness(t)
			h.App.Main = page
			if err := h.D.Resize(1280, 800); err != nil {
				t.Fatal(err)
			}
			searchOpen(t, h)
			snapshot := h.D.Snapshot()
			focused := false
			for _, hit := range snapshot.Hits {
				if hit.Label == h.App.S.Search.Field && hit.ID == snapshot.Focused {
					focused = true
				}
			}
			if !focused {
				t.Fatal("search field did not receive autofocus")
			}
			searchType(t, h, "no matching query")
			if !h.shows(h.App.S.Search.Empty) {
				t.Fatal("typing without a click did not filter the palette")
			}
			searchKey(t, h, gfx.KeyEscape)
			if h.App.search.open || searchHasHit(h, h.App.S.Search.Field) {
				t.Fatal("Escape left the palette visible")
			}
			searchOpen(t, h)
			if h.App.search.query != "" {
				t.Fatal("reopening did not reset the query")
			}
		})
	}
}

func TestSearchFiltersCollectionsHistoryAndCommands(t *testing.T) {
	for _, tt := range []struct{ query, want, absent string }{
		{"ACCOUNTS", "Get accounts", "Recent audit"},
		{"customers", "Get accounts", "Recent audit"},
		{"api.test", "Get accounts", "Recent audit"},
		{"AUDIT.TEST", "Recent audit", "Get accounts"},
		{"切换主题", i18n.For(i18n.Zh).Search.ToggleTheme, "Recent audit"},
	} {
		t.Run(tt.query, func(t *testing.T) {
			h := newHarness(t, searchFixture(t))
			searchOpen(t, h)
			searchType(t, h, tt.query)
			if !searchHasHit(h, tt.want) || searchHasHit(h, tt.absent) {
				t.Fatalf("filter %q: hits=%q", tt.query, h.hitLabels())
			}
		})
	}
}

func TestSearchEnterRequestReturnsToWorkbench(t *testing.T) {
	h := newHarness(t, searchFixture(t))
	h.App.Main = PageHistory
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	searchOpen(t, h)
	searchType(t, h, "accounts")
	searchKey(t, h, gfx.KeyEnter)
	if h.App.Main != PageWorkbench || h.App.Active().Request.ID != "request-search" || h.App.Active().CollectionID == "" {
		t.Fatalf("request was not opened from its collection: %+v", h.App.Active())
	}
	if h.App.search.open {
		t.Fatal("choosing a request did not close search")
	}
}

func TestSearchThemeAndLanguageCommandsPersist(t *testing.T) {
	h := newHarness(t)
	settings := h.Svc.Settings.Get()
	settings.Language, settings.ActiveEnvID, settings.RequestTimeoutSec = "en", "keep-env", 17
	if err := h.Svc.Settings.Save(settings); err != nil {
		t.Fatal(err)
	}
	h.App.ApplySettings(settings)
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	searchOpen(t, h)
	searchType(t, h, "Toggle theme")
	searchKey(t, h, gfx.KeyEnter)
	saved := h.Svc.Settings.Get()
	if !h.App.Dark() || saved.Theme != "dark" || saved.ActiveEnvID != "keep-env" || saved.RequestTimeoutSec != 17 {
		t.Fatalf("theme command did not persist without overwriting other settings: %+v", saved)
	}
	searchOpen(t, h)
	searchType(t, h, "Switch language")
	searchKey(t, h, gfx.KeyEnter)
	if h.App.S.Lang != i18n.Zh || h.Svc.Settings.Get().Language != "zh" {
		t.Fatal("language command did not apply and save Chinese")
	}
	searchOpen(t, h)
	searchType(t, h, h.App.S.Search.ToggleTheme)
	searchKey(t, h, gfx.KeyEnter)
	if h.App.Dark() || h.Svc.Settings.Get().Theme != "light" {
		t.Fatal("theme command did not toggle back to light")
	}
}

func TestSearchPlatformHintsAndLanguages(t *testing.T) {
	for _, platform := range []string{"windows", "darwin"} {
		for _, lang := range []i18n.Lang{i18n.Zh, i18n.En} {
			t.Run(platform+"/"+string(lang), func(t *testing.T) {
				h := newHarness(t)
				settings := h.Svc.Settings.Get()
				settings.Language = string(lang)
				h.App.ApplySettings(settings)
				h.App.Platform = platform
				if err := h.D.Resize(1280, 800); err != nil {
					t.Fatal(err)
				}
				want := "Ctrl K"
				if platform == "darwin" {
					want = "⌘ K"
				}
				if !h.shows(want) || !h.shows(h.App.S.Search.Placeholder) {
					t.Fatalf("missing localized search field or %q", want)
				}
				h.click(h.App.S.Search.Title)
				h.pump()
				if !searchHasHit(h, h.App.S.Search.Field) {
					t.Fatal("clicking the titlebar search did not open the localized palette")
				}
				searchType(t, h, h.App.S.Search.NewRequest)
				searchKey(t, h, gfx.KeyEnter)
				if len(h.App.Tabs()) != 2 {
					t.Fatal("localized new request command did not open a new tab")
				}
			})
		}
	}
}

func TestSearchArrowsSkipGroupsAndClickEnvironment(t *testing.T) {
	h := newHarness(t, searchFixture(t))
	searchOpen(t, h)
	searchKey(t, h, gfx.KeyDown)
	searchKey(t, h, gfx.KeyUp)
	searchKey(t, h, gfx.KeyEnter)
	if h.App.Active().Request.ID != "request-search" {
		t.Fatal("arrows did not return to the first request across the group heading")
	}
	searchOpen(t, h)
	searchType(t, h, "Staging")
	h.click("Staging")
	h.pump()
	if h.Svc.Settings.Get().ActiveEnvID == "" || h.App.search.open {
		t.Fatal("click did not select and save the environment")
	}
}

type searchCountingCollections struct {
	service.CollectionService
	lists int
}

func (s *searchCountingCollections) List() []model.Collection {
	s.lists++
	return s.CollectionService.List()
}

func TestSearchCachesIndexUntilQueryChanges(t *testing.T) {
	svc := searchFixture(t)
	collections := &searchCountingCollections{CollectionService: svc.Collections}
	svc.Collections = collections
	h := newHarness(t, svc)
	// History does not paint the collection sidebar, isolating search's reads.
	h.App.Main = PageHistory
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	before := collections.lists
	searchOpen(t, h)
	if collections.lists != before+1 {
		t.Fatalf("opening rebuilt index %d times", collections.lists-before)
	}
	before = collections.lists
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	if collections.lists != before {
		t.Fatal("unchanged paint rebuilt the search index")
	}
	searchType(t, h, "accounts")
	if collections.lists != before+1 {
		t.Fatal("query change did not rebuild the index once")
	}
	searchKey(t, h, gfx.KeyEscape)
	before = collections.lists
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	if collections.lists != before {
		t.Fatal("closed search reads collections")
	}
}

type searchFailSettings struct{ service.SettingsService }

func (s searchFailSettings) Save(model.Settings) error { return errors.New("save failed") }

func TestSearchSaveFailureDoesNotApplyTheme(t *testing.T) {
	svc := fake.New()
	svc.Settings = searchFailSettings{svc.Settings}
	h := newHarness(t, svc)
	searchOpen(t, h)
	searchType(t, h, h.App.S.Search.ToggleTheme)
	searchKey(t, h, gfx.KeyEnter)
	if h.App.Dark() || h.Svc.Settings.Get().Theme != "light" || !h.shows("save failed") {
		t.Fatal("failed save must keep the active theme and show an error")
	}
}

func TestSearchUnicodeMatchAndPrefixRanking(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Unicode")
	for i, name := range []string{"Other match", "Match first", "Kelvin 请求"} {
		r := model.NewRequest(fmt.Sprint(i))
		r.Name, r.URL = name, "https://example.test/"+fmt.Sprint(i)
		if _, err := svc.Collections.SaveRequest(col.ID, "", r); err != nil {
			t.Fatal(err)
		}
	}
	h := newHarness(t, svc)
	searchOpen(t, h)
	searchType(t, h, "MATCH")
	searchKey(t, h, gfx.KeyEnter)
	if h.App.Active().Request.Name != "Match first" {
		t.Fatal("prefix match was not ranked above a substring match")
	}
	searchOpen(t, h)
	searchType(t, h, "kelvin 请求")
	if !searchHasHit(h, "Kelvin 请求") || !h.shows("Kelvin 请求") {
		t.Fatal("Unicode case folding corrupted the highlighted text")
	}
}

func TestSearchHistoryLimitAndVirtualization(t *testing.T) {
	svc := fake.New()
	for i := range 80 {
		r := model.NewRequest(fmt.Sprint(i))
		r.URL = fmt.Sprintf("https://history.test/%02d", i)
		if err := svc.History.Add(model.HistoryEntry{Request: r}); err != nil {
			t.Fatal(err)
		}
	}
	h := newHarness(t, svc)
	searchOpen(t, h)
	searchType(t, h, "history.test")
	visible := 0
	for _, hit := range h.D.Snapshot().Hits {
		if strings.HasPrefix(hit.Label, "https://history.test/") {
			visible++
		}
	}
	if visible == 0 || visible >= 50 {
		t.Fatalf("expected only visible rows to be declared, got %d", visible)
	}
	searchKey(t, h, gfx.KeyEscape)
	searchOpen(t, h)
	searchType(t, h, "https://history.test/00")
	if !h.shows(h.App.S.Search.Empty) {
		t.Fatal("search included history older than the latest 50 URLs")
	}
}

func TestSearchOpensHistoryAndExistingTab(t *testing.T) {
	h := newHarness(t, searchFixture(t))
	searchOpen(t, h)
	searchType(t, h, "audit.test")
	searchKey(t, h, gfx.KeyEnter)
	if h.App.Active().Request.ID != "history-search" || h.App.Active().CollectionID != "" {
		t.Fatal("history search did not open an unsaved request")
	}
	opened := h.App.Active()
	h.App.NewTab()
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
	count := len(h.App.Tabs())
	searchOpen(t, h)
	searchType(t, h, "Recent audit")
	searchKey(t, h, gfx.KeyEnter)
	if h.App.Active() != opened || len(h.App.Tabs()) != count {
		t.Fatal("open-tab result did not activate the existing tab")
	}
}

func TestSearchCommandsOpenCollectionImportAndSettings(t *testing.T) {
	for _, command := range []string{"New collection", "Import", "Settings"} {
		t.Run(command, func(t *testing.T) {
			svc := fake.New()
			settings := svc.Settings.Get()
			settings.Language = "en"
			if err := svc.Settings.Save(settings); err != nil {
				t.Fatal(err)
			}
			h := newHarness(t, svc)
			searchOpen(t, h)
			searchType(t, h, command)
			searchKey(t, h, gfx.KeyEnter)
			switch command {
			case "New collection":
				if !h.App.collections.dialogOpen || h.App.Section != SectionAPIs || h.App.Main != PageWorkbench {
					t.Fatal("new collection command did not open the creation dialog")
				}
			case "Import":
				if !h.App.importer.open {
					t.Fatal("import command did not open the importer")
				}
			case "Settings":
				if h.App.Main != PageSettings {
					t.Fatal("settings command did not open settings")
				}
			}
		})
	}
}

func TestSearchArrowsRevealVirtualizedResults(t *testing.T) {
	svc := fake.New()
	col, _ := svc.Collections.Create("Large collection")
	for i := range 80 {
		r := model.NewRequest(fmt.Sprint(i))
		r.Name, r.URL = fmt.Sprintf("Request %02d", i), fmt.Sprintf("https://api.test/%d", i)
		if _, err := svc.Collections.SaveRequest(col.ID, "", r); err != nil {
			t.Fatal(err)
		}
	}
	h := newHarness(t, svc)
	searchOpen(t, h)
	searchType(t, h, "Request ")
	for range 79 {
		searchKey(t, h, gfx.KeyDown)
	}
	h.waitFor("selected result to scroll into view", func() bool { return searchHasHit(h, "Request 79") })
	searchKey(t, h, gfx.KeyEnter)
	if h.App.Active().Request.ID != "79" {
		t.Fatal("Enter did not open the selected result after scrolling")
	}
}

func BenchmarkSearchClosedPaint(b *testing.B) {
	a := New(fake.New())
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { a.searchBox(c); a.searchPalette(c) }, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	for range 20 {
		if err := d.Resize(1280, 800); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := d.Resize(1280, 800); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchOpenPaint(b *testing.B) {
	a := New(fake.New())
	a.openSearch()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { a.searchPalette(c) }, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		b.Fatal(err)
	}
	defer d.Close()
	for range 20 {
		if err := d.Resize(1280, 800); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := d.Resize(1280, 800); err != nil {
			b.Fatal(err)
		}
	}
}
