package gui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Direct fixture changes need a paint; Pump only delivers requested wakes.
func repaintTabs(t *testing.T, h *harness) {
	t.Helper()
	if err := h.D.Resize(1280, 800); err != nil {
		t.Fatal(err)
	}
}

func TestTabsNewButtonActivatesNewTab(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	h.click(h.App.S.Tabs.New)
	if len(h.App.Tabs()) != 2 || h.App.Active() == first || h.App.active != 1 {
		t.Fatalf("new button did not open and activate a tab: count=%d active=%d", len(h.App.Tabs()), h.App.active)
	}
	if h.App.Active().Request.URL != "" || h.App.Active().Dirty {
		t.Fatal("new tab should start empty and clean")
	}
}

func TestTabsSwitchShowsOwnURL(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	first.Request.Name = "First request"
	first.Request.URL = "https://first.test/"
	second := h.App.NewTab()
	second.Request.Name = "Second request"
	second.Request.URL = "https://second.test/"
	repaintTabs(t, h)
	h.click(first.Request.Name)
	assertTabsEditorURL(t, h, first.Request.URL)
	h.click(second.Request.Name)
	assertTabsEditorURL(t, h, second.Request.URL)
}

func assertTabsEditorURL(t *testing.T, h *harness, want string) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == "URL" {
			if hit.Value != want {
				t.Fatalf("URL field = %q, want %q", hit.Value, want)
			}
			return
		}
	}
	t.Fatal("URL editor was not painted")
}

func clickTabsClose(t *testing.T, h *harness, index int) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Tabs.Close {
			if index == 0 {
				b := hit.Bounds
				if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
					t.Fatal(err)
				}
				return
			}
			index--
		}
	}
	t.Fatal("close button was not painted")
}

func TestTabsCloseActiveActivatesNeighbour(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	middle := h.App.NewTab()
	last := h.App.NewTab()
	h.App.active = 1
	repaintTabs(t, h)
	clickTabsClose(t, h, 1)
	if len(h.App.Tabs()) != 2 || h.App.Active() != last {
		t.Fatal("closing middle tab did not select its right neighbour")
	}
	clickTabsClose(t, h, 1)
	if len(h.App.Tabs()) != 1 || h.App.Active() != first {
		t.Fatal("closing last tab did not select its left neighbour")
	}
	clickTabsClose(t, h, 0)
	if len(h.App.Tabs()) != 1 || h.App.Active() == first || h.App.Active() == middle {
		t.Fatal("closing final tab should leave a fresh tab")
	}
}

func TestTabsOpenRequestReactivatesExistingID(t *testing.T) {
	h := newHarness(t)
	req := model.NewRequest("saved-request")
	req.URL = "https://saved.test/"
	h.App.OpenRequest(req, "collection")
	first := h.App.Active()
	first.Request.URL = "https://edited.test/"
	first.Dirty = true
	first.Response = &model.Response{Status: "200 OK"}
	h.App.NewTab()
	h.App.OpenRequest(req, "collection")
	repaintTabs(t, h)
	if len(h.App.Tabs()) != 2 || h.App.Active() != first {
		t.Fatal("opening an existing ID duplicated it instead of activating it")
	}
	if !first.Dirty || first.Response == nil || first.Request.URL != "https://edited.test/" {
		t.Fatal("reactivating a request lost its working state")
	}
	assertTabsEditorURL(t, h, first.Request.URL)
}

func TestTabsOpenRequestReusesEmptyTabAndClonesRequest(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	first.Err = "old error"
	first.Tests = []TestResult{{Name: "old test"}}
	req := model.NewRequest("saved")
	req.URL = "https://saved.test/"
	req.Headers = []model.KV{{Key: "X-Test", Value: "original"}}
	h.App.OpenRequest(req, "collection")
	repaintTabs(t, h)
	if len(h.App.Tabs()) != 1 || h.App.Active() != first || first.CollectionID != "collection" {
		t.Fatal("opening a request did not reuse the empty tab")
	}
	if first.Response != nil || first.Err != "" || first.Tests != nil || first.Dirty {
		t.Fatal("loading a request did not clear result state")
	}
	first.Request.Headers[0].Value = "edited"
	if req.Headers[0].Value != "original" {
		t.Fatal("editing a tab changed its source request")
	}
}

func TestTabsOpenRequestKeepsOccupiedTabs(t *testing.T) {
	for _, occupied := range []string{"url", "dirty", "response", "sending"} {
		t.Run(occupied, func(t *testing.T) {
			h := newHarness(t)
			first := h.App.Active()
			switch occupied {
			case "url":
				first.Request.URL = "https://existing.test/"
			case "dirty":
				first.Dirty = true
			case "response":
				first.Response = &model.Response{Status: "200 OK"}
			case "sending":
				first.Request.URL = "https://sending.test/"
				h.App.Send(first)
				first.Request.URL = ""
			}
			req := model.NewRequest("another")
			h.App.OpenRequest(req, "collection")
			repaintTabs(t, h)
			if len(h.App.Tabs()) != 2 || h.App.Active() == first {
				t.Fatal("opening a request replaced an occupied tab")
			}
		})
	}
}

func TestTabsDirtyCloseAsksBeforeDiscard(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	first.Request.Name, first.Dirty = "Unsaved request", true
	repaintTabs(t, h)
	clickTabsClose(t, h, 0)
	if len(h.App.Tabs()) != 1 || !h.shows(h.App.S.Tabs.ConfirmTitle) {
		t.Fatal("dirty close did not ask for confirmation")
	}
	h.click(h.App.S.Tabs.Cancel)
	if h.App.Active() != first || h.shows(h.App.S.Tabs.ConfirmTitle) {
		t.Fatal("cancel did not keep the dirty tab")
	}
	clickTabsClose(t, h, 0)
	h.click(h.App.S.Tabs.Discard)
	if h.App.Active() == first || len(h.App.Tabs()) != 1 || h.shows(h.App.S.Tabs.ConfirmTitle) {
		t.Fatal("discard did not replace the final dirty tab with a fresh tab")
	}
}

func TestTabsConcurrentSendsKeepOwnResponses(t *testing.T) {
	svc := fake.New()
	firstDone, secondDone := make(chan struct{}), make(chan struct{})
	started := make(chan string, 2)
	t.Cleanup(func() {
		select {
		case <-firstDone:
		default:
			close(firstDone)
		}
		select {
		case <-secondDone:
		default:
			close(secondDone)
		}
	})
	svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, req model.Request) (model.Response, error) {
		started <- req.URL
		done := firstDone
		if strings.Contains(req.URL, "second") {
			done = secondDone
		}
		select {
		case <-done:
			return model.Response{Status: "200 OK", Body: []byte(req.URL), URL: req.URL}, nil
		case <-ctx.Done():
			return model.Response{}, ctx.Err()
		}
	}
	h := newHarness(t, svc)
	first := h.App.Active()
	first.Request.Name, first.Request.URL = "First send", "https://first.test/"
	second := h.App.NewTab()
	second.Request.Name, second.Request.URL = "Second send", "https://second.test/"
	repaintTabs(t, h)
	h.click("Send")
	h.click(first.Request.Name)
	h.click("Send")
	h.waitFor("both sends to start", func() bool { return len(started) == 2 })
	if !first.Sending || !second.Sending {
		t.Fatal("both requests should be in flight")
	}
	close(secondDone)
	h.waitFor("inactive second response", func() bool { return second.Response != nil })
	if !first.Sending || first.Response != nil {
		t.Fatal("second response changed first tab's pending send")
	}
	close(firstDone)
	h.waitFor("first response", func() bool { return first.Response != nil })
	for _, tab := range []*Tab{first, second} {
		if tab.Sending || tab.Err != "" || string(tab.Response.Body) != tab.Request.URL {
			t.Fatalf("tab %d received the wrong response: %+v", tab.ID, tab.Response)
		}
		h.click(tab.Request.Name)
		if !h.shows(tab.Request.URL) {
			t.Fatal("switching did not show the tab's response")
		}
	}
	if len(h.Svc.History.List()) != 2 {
		t.Fatal("both completed sends should be recorded")
	}
}

func TestTabsShortcuts(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	if _, err := h.D.ShortcutChord(ui.Primary, 't'); err != nil {
		t.Fatal(err)
	}
	if len(h.App.Tabs()) != 2 || h.App.Active() == first {
		t.Fatal("Primary+T did not create and activate a tab")
	}
	second := h.App.Active()
	if err := h.D.Key(gfx.KeyTab, ui.Ctrl); err != nil {
		t.Fatal(err)
	}
	if h.App.Active() != first {
		t.Fatal("Ctrl+Tab did not wrap to the first tab")
	}
	if err := h.D.Key(gfx.KeyTab, ui.Ctrl|ui.Shift); err != nil {
		t.Fatal(err)
	}
	if h.App.Active() != second {
		t.Fatal("Ctrl+Shift+Tab did not wrap to the last tab")
	}
	second.Dirty = true
	repaintTabs(t, h)
	if _, err := h.D.ShortcutChord(ui.Primary, 'w'); err != nil {
		t.Fatal(err)
	}
	if !h.shows(h.App.S.Tabs.ConfirmTitle) || len(h.App.Tabs()) != 2 {
		t.Fatal("Primary+W did not confirm the dirty close")
	}
	h.click(h.App.S.Tabs.Discard)
	if len(h.App.Tabs()) != 1 || h.App.Active() != first {
		t.Fatal("shortcut close did not return to the neighbour")
	}
}

func TestTabsSidewaysScrollAndRevealSelection(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	first.Request.Name = "First tab"
	for range 7 {
		h.App.NewTab()
	}
	last := h.App.Active()
	last.Request.Name = "Last tab"
	middle := h.App.Tabs()[2]
	middle.Request.Name = "Middle tab"
	h.App.active = 0
	repaintTabs(t, h)
	findTab := func(title string) gfx.Hit {
		t.Helper()
		for _, hit := range h.D.Snapshot().Hits {
			if hit.Label == title {
				return hit
			}
		}
		t.Fatalf("no tab %q", title)
		return gfx.Hit{}
	}
	before := findTab(middle.Request.Name).Bounds
	leftEdge := findTab(first.Request.Name).Bounds.X
	if err := h.D.Scroll(gfx.ScrollEvent{X: before.X + before.Width/2, Y: before.Y + before.Height/2, DeltaX: 200, Precise: true}); err != nil {
		t.Fatal(err)
	}
	scrolled := findTab(middle.Request.Name).Bounds
	if scrolled.X >= before.X {
		t.Fatal("horizontal wheel did not move the tab strip")
	}
	repaintTabs(t, h)
	if findTab(middle.Request.Name).Bounds.X != scrolled.X {
		t.Fatal("repainting reset the user's sideways scroll")
	}
	if err := h.D.Key(gfx.KeyTab, ui.Ctrl|ui.Shift); err != nil {
		t.Fatal(err)
	}
	if h.App.Active() != last {
		t.Fatal("previous shortcut did not select the last tab")
	}
	lastBounds := findTab(last.Request.Name).Bounds
	newBounds := findTab(h.App.S.Tabs.New).Bounds
	if lastBounds.X < leftEdge || lastBounds.X+lastBounds.Width > newBounds.X {
		t.Fatal("selected tab was not scrolled into view")
	}
}

func openTabsContext(t *testing.T, h *harness, title string) {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == title {
			b := hit.Bounds
			if err := h.D.ClickButton(b.X+b.Width/2, b.Y+b.Height/2, gfx.PointerButtonSecondary); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no tab %q", title)
}

func clickTabsContextItem(t *testing.T, h *harness, label string) {
	t.Helper()
	// Menu items paint after tab buttons and may share their accessible name.
	hits := h.D.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label {
			b := hits[i].Bounds
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no menu item %q", label)
}

func TestTabsContextDuplicateClonesEditableRequest(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	first.Request.Name, first.Request.URL = "Original", "https://original.test/"
	first.Request.Headers = []model.KV{{Key: "X-Test", Value: "original"}}
	first.Response = &model.Response{Status: "200 OK"}
	repaintTabs(t, h)
	openTabsContext(t, h, first.Request.Name)
	h.click(h.App.S.Tabs.Duplicate)
	copy := h.App.Active()
	if len(h.App.Tabs()) != 2 || copy == first || copy.Request.ID == first.Request.ID || !copy.Dirty {
		t.Fatal("duplicate should create an active, independently saveable copy")
	}
	if copy.Request.URL != first.Request.URL || copy.Response != nil || copy.Sending {
		t.Fatal("duplicate copied result state or lost request data")
	}
	copy.Request.Headers[0].Value = "changed"
	if first.Request.Headers[0].Value != "original" {
		t.Fatal("duplicate shares mutable request data")
	}
}

func TestTabsContextCloseOthersConfirmsEachDirtyTab(t *testing.T) {
	h := newHarness(t)
	keep := h.App.Active()
	keep.Request.Name = "Keep"
	clean := h.App.NewTab()
	clean.Request.Name = "Clean"
	dirty := h.App.NewTab()
	dirty.Request.Name, dirty.Dirty = "Dirty", true
	last := h.App.NewTab()
	last.Request.Name, last.Dirty = "Last dirty", true
	repaintTabs(t, h)
	openTabsContext(t, h, keep.Request.Name)
	h.click(h.App.S.Tabs.CloseOthers)
	if len(h.App.Tabs()) != 3 || !h.shows(h.App.S.Tabs.ConfirmTitle) || !h.shows(dirty.Request.Name) {
		t.Fatal("close others should close clean tabs and confirm the first dirty tab")
	}
	h.click(h.App.S.Tabs.Discard)
	if len(h.App.Tabs()) != 2 || !h.shows(h.App.S.Tabs.ConfirmTitle) {
		t.Fatal("each dirty tab should be confirmed separately")
	}
	h.click(h.App.S.Tabs.Cancel)
	if h.App.Active() != last || len(h.App.Tabs()) != 2 {
		t.Fatal("cancel should keep the remaining dirty tab")
	}
	openTabsContext(t, h, last.Request.Name)
	clickTabsContextItem(t, h, h.App.S.Tabs.Close)
	h.click(h.App.S.Tabs.Discard)
	if len(h.App.Tabs()) != 1 || h.App.Active() != keep {
		t.Fatal("context close should leave the retained tab active")
	}
}

func tabsHit(t *testing.T, h *harness, label string) gfx.Hit {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == label {
			return hit
		}
	}
	t.Fatalf("no control %q", label)
	return gfx.Hit{}
}

func tabsCloseHit(t *testing.T, h *harness, tab gfx.Hit) gfx.Hit {
	t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label == h.App.S.Tabs.Close && hit.Bounds.X >= tab.Bounds.X && hit.Bounds.X+hit.Bounds.Width <= tab.Bounds.X+tab.Bounds.Width {
			return hit
		}
	}
	t.Fatal("no close control inside tab")
	return gfx.Hit{}
}

func tabsCloseVisible(h *harness, close gfx.Hit) bool {
	for _, p := range h.D.Snapshot().Primitives {
		// The close icon is a vector; count visible vector paint inside its control.
		if p.Kind == app.TrianglePrimitive && p.Color.Alpha > 0 &&
			p.Points[0].X >= close.Bounds.X && p.Points[0].Y >= close.Bounds.Y &&
			p.Points[0].X < close.Bounds.X+close.Bounds.Width && p.Points[0].Y < close.Bounds.Y+close.Bounds.Height {
			return true
		}
	}
	return false
}

func TestTabsHoverRevealsCloseAndReplacesDirtyDot(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		t.Run(fmt.Sprint(dirty), func(t *testing.T) {
			h := newHarness(t)
			first := h.App.Active()
			first.Request.Name, first.Dirty = "Hover me", dirty
			h.App.NewTab()
			repaintTabs(t, h)
			tab := tabsHit(t, h, first.Request.Name)
			close := tabsCloseHit(t, h, tab)
			if tabsCloseVisible(h, close) {
				t.Fatal("inactive tab close visible before hover")
			}
			if err := h.D.Move(tab.Bounds.X+tab.Bounds.Width/2, tab.Bounds.Y+tab.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			if !tabsCloseVisible(h, close) {
				t.Fatal("hover did not reveal the close icon")
			}
			if dirty {
				for _, label := range h.D.Snapshot().Labels {
					if label.Value == h.App.S.Tabs.DirtySymbol && label.Color.Alpha > 0 {
						t.Fatal("dirty dot still visible over close icon")
					}
				}
			}
			// Moving onto the close control must keep its parent's hover look.
			if err := h.D.Move(close.Bounds.X+close.Bounds.Width/2, close.Bounds.Y+close.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			if !tabsCloseVisible(h, close) {
				t.Fatal("close icon vanished when pointer entered it")
			}
			if err := h.D.Click(close.Bounds.X+close.Bounds.Width/2, close.Bounds.Y+close.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			if dirty && !h.shows(h.App.S.Tabs.ConfirmTitle) {
				t.Fatal("hover close bypassed dirty confirmation")
			}
			if !dirty && len(h.App.Tabs()) != 1 {
				t.Fatal("hover close did not close tab")
			}
		})
	}
}

func TestTabsDragReordersAndKeepsActiveRequest(t *testing.T) {
	for _, activeIndex := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(activeIndex), func(t *testing.T) {
			h := newHarness(t)
			first := h.App.Active()
			first.Request.Name = "First"
			middle := h.App.NewTab()
			middle.Request.Name = "Middle"
			last := h.App.NewTab()
			last.Request.Name = "Last"
			h.App.active = activeIndex
			active := h.App.Active()
			repaintTabs(t, h)
			drag := func(from, to string) {
				t.Helper()
				start, target := tabsHit(t, h, from).Bounds, tabsHit(t, h, to).Bounds
				if err := h.D.Press(start.X+start.Width/2, start.Y+start.Height/2, gfx.PointerButtonPrimary); err != nil {
					t.Fatal(err)
				}
				if err := h.D.Move(target.X+target.Width/2, target.Y+target.Height/2); err != nil {
					t.Fatal(err)
				}
				if !h.App.tabBarState.drag.Dragging() {
					t.Fatal("pointer move did not start drag")
				}
				if h.App.tabBarState.drag.Target() != tabsHit(t, h, to).ID {
					t.Fatalf("drag target=%v want=%v source=%v start=%+v target=%+v", h.App.tabBarState.drag.Target(), tabsHit(t, h, to).ID, h.App.tabBarState.drag.Source(), start, target)
				}
				if err := h.D.Release(target.X+target.Width/2, target.Y+target.Height/2, gfx.PointerButtonPrimary); err != nil {
					t.Fatal(err)
				}
			}
			drag(first.Request.Name, last.Request.Name)
			if h.App.Tabs()[0] != middle || h.App.Tabs()[1] != last || h.App.Tabs()[2] != first || h.App.Active() != active {
				t.Fatal("drag right changed active request or order")
			}
			drag(first.Request.Name, middle.Request.Name)
			if h.App.Tabs()[0] != first || h.App.Tabs()[1] != middle || h.App.Tabs()[2] != last || h.App.Active() != active {
				t.Fatal("drag left changed active request or order")
			}
		})
	}
}

func TestTabsReopenClosedRestoresIndependentRequest(t *testing.T) {
	h := newHarness(t)
	closed := h.App.Active()
	closed.Request.Name, closed.Request.URL = "Restore me", "https://restore.test/"
	closed.Request.Headers = []model.KV{{Key: "X-Original", Value: "original"}}
	closed.CollectionID, closed.Dirty = "saved-collection", true
	closed.Response = &model.Response{StatusCode: 200}
	repaintTabs(t, h)
	if _, err := h.D.ShortcutChord(ui.Primary, 'w'); err != nil {
		t.Fatal(err)
	}
	h.click(h.App.S.Tabs.Discard)
	closed.Request.Headers[0].Value = "changed after close"
	if _, err := h.D.ShortcutChord(ui.Primary|ui.Shift, 't'); err != nil {
		t.Fatal(err)
	}
	got := h.App.Active()
	if len(h.App.Tabs()) != 2 || got == closed || got.Request.ID != closed.Request.ID || got.Request.URL != closed.Request.URL ||
		got.CollectionID != "saved-collection" || !got.Dirty || got.Request.Headers[0].Value != "original" || got.Response != nil || got.Sending {
		t.Fatalf("reopened request lost data or reused execution state: %+v", got)
	}
	assertTabsEditorURL(t, h, got.Request.URL)
}

func TestTabsReopenKeepsLastTenInReverseOrder(t *testing.T) {
	h := newHarness(t)
	for i := range 12 {
		tab := h.App.Active()
		tab.Request.Name = fmt.Sprintf("Closed %d", i)
		h.App.closeRequestTabs(tab.ID, false)
	}
	repaintTabs(t, h)
	for i := 11; i >= 2; i-- {
		if _, err := h.D.ShortcutChord(ui.Primary|ui.Shift, 't'); err != nil {
			t.Fatal(err)
		}
		if h.App.Active().Request.Name != fmt.Sprintf("Closed %d", i) {
			t.Fatal("closed tabs restored out of order")
		}
	}
	count := len(h.App.Tabs())
	if _, err := h.D.ShortcutChord(ui.Primary|ui.Shift, 't'); err != nil {
		t.Fatal(err)
	}
	if len(h.App.Tabs()) != count {
		t.Fatal("reopen retained more than ten closed requests")
	}
}

func TestTabsOpenRequestFromHistoryReturnsToWorkbench(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			h := newHarness(t)
			req := model.NewRequest("history-request")
			req.URL = "https://history.test/"
			if existing {
				h.App.OpenRequest(req, "collection")
				h.App.NewTab()
			}
			h.App.Main = PageHistory
			repaintTabs(t, h)
			h.App.OpenRequest(req, "collection")
			repaintTabs(t, h)
			if h.App.Main != PageWorkbench || h.App.Active().Request.ID != req.ID {
				t.Fatal("opening from history did not activate the workbench request")
			}
			assertTabsEditorURL(t, h, req.URL)
		})
	}
}

func TestTabsCloseRightKeepsLeftAndConfirmsDirty(t *testing.T) {
	h := newHarness(t)
	left := h.App.Active()
	left.Request.Name, left.Dirty = "Left dirty", true
	keep := h.App.NewTab()
	keep.Request.Name = "Keep"
	right := h.App.NewTab()
	right.Request.Name = "Right clean"
	dirty := h.App.NewTab()
	dirty.Request.Name, dirty.Dirty = "Right dirty", true
	h.App.active = 1
	repaintTabs(t, h)
	openTabsContext(t, h, keep.Request.Name)
	h.click(h.App.S.Tabs.CloseRight)
	if len(h.App.Tabs()) != 3 || h.App.Tabs()[0] != left || h.App.Tabs()[1] != keep || !h.shows(h.App.S.Tabs.ConfirmTitle) {
		t.Fatal("close right removed left tabs or skipped dirty confirmation")
	}
	h.click(h.App.S.Tabs.Cancel)
	if h.App.requestTabIndex(dirty.ID) < 0 {
		t.Fatal("cancel removed dirty right tab")
	}
	openTabsContext(t, h, keep.Request.Name)
	h.click(h.App.S.Tabs.CloseRight)
	h.click(h.App.S.Tabs.Discard)
	if len(h.App.Tabs()) != 2 || h.App.Tabs()[0] != left || h.App.Active() != keep {
		t.Fatal("discard did not close only tabs to the right")
	}
}

func TestTabsOverflowMenuActivatesChosenTab(t *testing.T) {
	h := newHarness(t)
	first := h.App.Active()
	first.Request.Name = "First hidden tab"
	for i := range 7 {
		h.App.NewTab().Request.Name = fmt.Sprintf("Overflow %d", i)
	}
	repaintTabs(t, h)
	repaintTabs(t, h)
	h.click(h.App.S.Tabs.AllTabs)
	clickTabsContextItem(t, h, first.Request.Name)
	if h.App.Active() != first || h.App.tabBarState.menu {
		t.Fatal("overflow menu did not select chosen tab and close")
	}
	if tabsHit(t, h, first.Request.Name).Bounds.X < SidebarWidth+RailWidth {
		t.Fatal("menu selection did not reveal hidden tab")
	}
}

func TestTabsContextCloseSavedConfirmsEditsAndKeepsUnsaved(t *testing.T) {
	h := newHarness(t)
	unsaved := h.App.Active()
	unsaved.Request.Name, unsaved.Dirty = "Unsaved", true
	saved := h.App.NewTab()
	saved.Request.Name, saved.CollectionID = "Saved", "collection"
	edited := h.App.NewTab()
	edited.Request.Name, edited.CollectionID, edited.Dirty = "Edited saved", "collection", true
	repaintTabs(t, h)
	openTabsContext(t, h, unsaved.Request.Name)
	h.click(h.App.S.Tabs.CloseSaved)
	if len(h.App.Tabs()) != 2 || h.App.requestTabIndex(unsaved.ID) < 0 || !h.shows(h.App.S.Tabs.ConfirmTitle) {
		t.Fatal("close saved did not retain unsaved tabs and confirm saved edits")
	}
	h.click(h.App.S.Tabs.Discard)
	if len(h.App.Tabs()) != 1 || h.App.Active() != unsaved {
		t.Fatal("close saved removed unsaved tab")
	}
}

func TestTabsContextCopiesURL(t *testing.T) {
	h := newHarness(t)
	tab := h.App.Active()
	tab.Request.Name, tab.Request.URL = "Copy me", "https://copy.test/path?x=1"
	repaintTabs(t, h)
	openTabsContext(t, h, tab.Request.Name)
	h.click(h.App.S.Tabs.CopyURL)
	if h.D.Clipboard() != tab.Request.URL {
		t.Fatal("copy URL did not write the request URL")
	}
}

func TestTabsMiddleClickClosesAndConfirmsDirty(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		t.Run(fmt.Sprint(dirty), func(t *testing.T) {
			h := newHarness(t)
			first := h.App.Active()
			first.Request.Name, first.Dirty = "Middle close", dirty
			last := h.App.NewTab()
			repaintTabs(t, h)
			b := tabsHit(t, h, first.Request.Name).Bounds
			if err := h.D.ClickButton(b.X+b.Width/2, b.Y+b.Height/2, gfx.PointerButtonMiddle); err != nil {
				t.Fatal(err)
			}
			if dirty {
				if !h.shows(h.App.S.Tabs.ConfirmTitle) {
					t.Fatal("middle close bypassed dirty confirmation")
				}
				h.click(h.App.S.Tabs.Discard)
			}
			if len(h.App.Tabs()) != 1 || h.App.Active() != last {
				t.Fatal("middle click did not close only its target")
			}
		})
	}
}

func TestTabsLanguagesAndThemesRender(t *testing.T) {
	for _, language := range []string{"zh", "en"} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(language+"/"+theme, func(t *testing.T) {
				h := newHarness(t)
				settings := h.Svc.Settings.Get()
				settings.Language, settings.Theme = language, theme
				h.App.ApplySettings(settings)
				h.App.Active().Dirty = true
				repaintTabs(t, h)
				if !h.shows(h.App.S.Tabs.Title) {
					t.Fatal("localized default tab title missing")
				}
				openTabsContext(t, h, h.App.S.Tabs.Title)
				for _, label := range []string{h.App.S.Tabs.Close, h.App.S.Tabs.CloseRight, h.App.S.Tabs.CloseSaved, h.App.S.Tabs.CopyURL} {
					tabsHit(t, h, label)
				}
				clickTabsContextItem(t, h, h.App.S.Tabs.Close)
				if !h.shows(h.App.S.Tabs.ConfirmTitle) || !h.shows(h.App.S.Tabs.ConfirmDiscard) {
					t.Fatal("localized discard dialog missing")
				}
				h.click(h.App.S.Tabs.Cancel)
			})
		}
	}
}

func BenchmarkTabsPaint(b *testing.B) {
	a := New(fake.New())
	for range 7 {
		a.NewTab()
	}
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(d.Close)
	for range 10 {
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
