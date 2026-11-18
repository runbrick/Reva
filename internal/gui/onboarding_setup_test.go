package gui

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/fake"
)

func TestOnboardingPracticePreservesInFlightDraft(t *testing.T) {
	h := newHarness(t)
	draft := h.App.Active()
	draft.Request.URL = "https://draft.invalid"
	draft.Dirty = true
	editor := h.App.requestTab(draft)
	editor.section = requestHeaders

	var sends atomic.Int32
	release := make(chan struct{})
	h.Svc.Requests.(*fake.Requests).Fn = func(ctx context.Context, _ model.Request) (model.Response, error) {
		sends.Add(1)
		select {
		case <-release:
			return model.Response{StatusCode: 200}, nil
		case <-ctx.Done():
			return model.Response{}, ctx.Err()
		}
	}
	h.App.Send(draft)
	requestRepaint(h)
	h.waitFor("draft send to start", func() bool { return sends.Load() == 1 })
	job := draft.job
	h.App.Main, h.App.Section = PageSettings, SectionEnvironments

	h.App.openOnboardingPractice("Practice request")
	requestRepaint(h)
	if len(h.App.Tabs()) != 2 || h.App.Tabs()[0] != draft || h.App.Active() == draft || !h.shows(h.App.S.Request.PreviewTitle) {
		t.Fatal("practice did not open preview in a separate tab")
	}
	if !draft.Dirty || draft.Request.URL != "https://draft.invalid" || !draft.Sending || draft.job != job {
		t.Fatal("opening practice changed the in-flight draft")
	}
	if h.App.requestTab(draft) != editor || editor.section != requestHeaders {
		t.Fatal("opening practice reset the draft editor")
	}

	close(release)
	h.waitFor("draft response while preview is open", func() bool { return !draft.Sending })
	if draft.Response == nil || draft.Response.StatusCode != 200 || sends.Load() != 1 {
		t.Fatal("opening practice interrupted the draft send or started another send")
	}
}
