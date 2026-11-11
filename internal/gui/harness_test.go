package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/runbrick/reqo/internal/service"
	"github.com/runbrick/reqo/internal/service/fake"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// harness drives an App headlessly through app.Driver. Every feature test
// in this package uses it. Add helpers in your own *_test.go file; change
// this one only for a bug.
type harness struct {
	t   *testing.T
	App *App
	Svc service.Services
	D   *app.Driver
}

// newHarness opens the App over in-memory services: fake.New(), or svc.
func newHarness(t *testing.T, svc ...service.Services) *harness {
	t.Helper()
	s := fake.New()
	if len(svc) > 0 {
		s = svc[0]
	}
	a := New(s)
	page := ui.NewPage(ui.PageConfig{View: a.Page, Theme: &a.theme})
	d, err := app.NewDriver(page.Config(app.Config{Width: 1280, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return &harness{t: t, App: a, Svc: s, D: d}
}

// pump repaints once, delivering finished effects.
func (h *harness) pump() {
	h.t.Helper()
	if err := h.D.Pump(); err != nil {
		h.t.Fatal(err)
	}
}

// click presses the middle of the first control whose name starts with name.
func (h *harness) click(name string) {
	h.t.Helper()
	for _, hit := range h.D.Snapshot().Hits {
		if strings.HasPrefix(hit.Label, name) {
			b := hit.Bounds
			if err := h.D.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				h.t.Fatal(err)
			}
			return
		}
	}
	h.t.Fatalf("no control named %q on screen; have %q", name, h.hitLabels())
}

// typeInto clicks the field named name and types text into it.
func (h *harness) typeInto(name, text string) {
	h.t.Helper()
	h.click(name)
	if err := h.D.Type(text); err != nil {
		h.t.Fatal(err)
	}
}

// shows reports whether any painted label contains text.
func (h *harness) shows(text string) bool {
	for _, l := range h.D.Snapshot().Labels {
		if strings.Contains(l.Value, text) {
			return true
		}
	}
	return false
}

// waitFor repaints until cond holds or two seconds pass.
func (h *harness) waitFor(what string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s", what)
		}
		select {
		case <-h.D.Wakes():
		case <-time.After(10 * time.Millisecond):
		}
		h.pump()
	}
}

func (h *harness) hitLabels() []string {
	var out []string
	for _, hit := range h.D.Snapshot().Hits {
		if hit.Label != "" {
			out = append(out, hit.Label)
		}
	}
	return out
}
