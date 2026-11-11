package gui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
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
	h.click(h.App.S.Shell.Collections)
	if h.App.Section != SectionCollections || h.App.Main != PageWorkbench {
		t.Fatalf("Collections click: section=%d main=%d", h.App.Section, h.App.Main)
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
