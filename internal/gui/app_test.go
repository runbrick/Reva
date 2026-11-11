package gui

import "testing"

func TestSendShowsResponseAndRecordsHistory(t *testing.T) {
	h := newHarness(t)
	h.App.Active().Request.URL = "https://example.test/ping"
	h.pump()
	h.click("Send")
	h.waitFor("response", func() bool { return h.App.Active().Response != nil })
	h.pump()
	if !h.shows("200 OK") {
		t.Fatal("status not shown")
	}
	if n := len(h.Svc.History.List()); n != 1 {
		t.Fatalf("history has %d entries, want 1", n)
	}
}
