package gui

import (
	"context"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/script"
	"github.com/runbrick/reva/internal/service/fake"
	"github.com/vivaui/viva/gfx"
)

func TestResponseToolsCompareManuallySentResponses(t *testing.T) {
	view := responseHarness(t, model.Response{StatusCode: 200, Status: "200 OK", Body: []byte(`{"item":{"price":5},"timestamp":1,"status":true}`)}, nil)
	for range 2 {
		if err := view.D.Resize(960, 600); err != nil {
			t.Fatal(err)
		}
	}
	labels := view.App.S.Response.Tools
	view.click(labels.Title)
	view.click(labels.SetBaseline)
	if err := view.D.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if view.App.response.views[view.App.Active().ID].tools.open {
		t.Fatal("Esc did not close the response tools")
	}
	view.Svc.Requests.(*fake.Requests).Fn = func(context.Context, model.Request) (model.Response, error) {
		return model.Response{StatusCode: 201, Status: "201 Created", Body: []byte(`{"item":{},"timestamp":2,"status":"ready"}`)}, nil
	}
	view.click("Send")
	view.waitFor("second response", func() bool { return !view.App.Active().Sending && view.App.Active().Response.StatusCode == 201 })
	view.click(labels.Title)
	view.typeInto(labels.Ignored, "/timestamp")
	view.click(labels.RunComparison)
	for _, label := range []string{labels.Status, labels.Baseline, labels.Current, "/item/price", labels.Removed} {
		if !view.shows(label) {
			t.Fatalf("comparison is missing %q", label)
		}
	}
	if changes := view.App.response.views[view.App.Active().ID].tools.changes; len(changes) != 2 {
		t.Fatalf("ignored timestamp still appeared: %+v", changes)
	}
	for _, label := range view.D.Snapshot().Labels {
		if label.Value == "/item/price" {
			bounds := label.Bounds
			if err := view.D.Scroll(gfx.ScrollEvent{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2, DeltaY: 300, Precise: true}); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if !view.shows(labels.TypeChanged) {
		t.Fatal("scrolling did not reveal the changed field")
	}
	view.click(labels.InspectCurrent)
	if !view.shows(labels.Selected) || !view.shows("/status") {
		t.Fatal("comparison could not open field actions")
	}
	view.click(labels.Compare)
	view.click(labels.ClearBaseline)
	if !view.shows(labels.NoBaseline) {
		t.Fatal("clearing the baseline did not reset the comparison")
	}
}

func TestResponseToolsGenerateEditableFieldChecksAndCapture(t *testing.T) {
	response := model.Response{StatusCode: 200, Status: "200 OK", Body: []byte(`{"data":{"large/id":9007199254740993}}`)}
	view := responseHarness(t, response, nil)
	tab := view.App.Active()
	tab.Request.TestScript = `pm.test("existing", function () { pm.response.to.have.status(200); });`
	labels := view.App.S.Response.Tools
	view.click(labels.Title)
	view.click(labels.Fields)
	view.click("/data/large~1id")
	view.click(labels.Exists)
	view.click(labels.Title)
	view.click(labels.Equals)
	view.click(labels.Title)
	view.typeInto(labels.Variable, "orderId")
	view.click(labels.Capture)
	if !tab.Dirty || !strings.Contains(tab.Request.TestScript, "existing") || view.App.requestTab(tab).section != requestScripts {
		t.Fatal("generated checks did not extend the editable test script")
	}
	assertions, _, captures, err := script.RunTestsWithCaptures(tab.Request.TestScript, tab.Request, response, nil, 0)
	if err != nil || len(assertions) != 4 || len(captures) != 1 || captures[0].Key != "orderId" || captures[0].Value != "9007199254740993" {
		t.Fatalf("generated script failed or lost numeric precision: %v, %v, %v", assertions, captures, err)
	}
	for _, assertion := range assertions {
		if !assertion.Passed {
			t.Errorf("generated check failed: %+v", assertion)
		}
	}
	response.Body = []byte(`{"data":{"large/id":9007199254740992}}`)
	assertions, _, err = script.RunTests(tab.Request.TestScript, tab.Request, response, nil, 0)
	if err != nil || len(assertions) != 4 || !assertions[1].Passed || assertions[2].Passed {
		t.Fatalf("generated tests failed to distinguish existence from equality: %v %v", assertions, err)
	}
}
