package gui

import (
	"testing"

	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

// Each rail item must select only its own section: one click must not reach
// the other items (they once shared a hit identity).
func TestRailItemsSelectTheirOwnSection(t *testing.T) {
	h := newHarness(t)
	h.App.SelectSection(SectionEnvironments)
	h.pump()
	h.click(h.App.S.Shell.APIs)
	if h.App.Section != SectionAPIs || h.App.Main != PageWorkbench {
		t.Fatalf("APIs click: section=%d main=%d, want APIs on the workbench", h.App.Section, h.App.Main)
	}
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Shell.Collections {
			t.Fatal("collections rail entry is still visible")
		}
	}
	ids := map[gfx.ID]string{}
	for _, hit := range h.D.Snapshot().Hits {
		for _, s := range Sections {
			if hit.Label == h.App.sectionLabel(s) {
				if other, dup := ids[hit.ID]; dup && other != hit.Label {
					t.Fatalf("rail items %q and %q share identity %v", other, hit.Label, hit.ID)
				}
				ids[hit.ID] = hit.Label
			}
		}
	}
}

// English rail captions are the longest; they must fit their items.
func TestRailCaptionsFitInEnglish(t *testing.T) {
	svc := fake.New()
	s := svc.Settings.Get()
	s.Language = "en"
	if err := svc.Settings.Save(s); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, svc)
	h.pump()
	for _, p := range headless.CheckLayout(h.D.Snapshot()) {
		if p.Bounds.X < RailWidth && p.Bounds.Y > TitlebarHeight {
			t.Errorf("rail layout: %+v", p)
		}
	}
}
