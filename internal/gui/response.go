package gui

import "github.com/vivaui/viva/ui"

// responseState is the response viewer's UI state. Owned by the response PR.
type responseState struct{}

// responsePane declares the response of t: status, duration, size, headers
// and body (pretty / raw).
func (a *App) responsePane(c ui.C, t *Tab) {
	ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
		switch {
		case t.Sending:
			ui.Spinner(c)
		case t.Err != "":
			ui.Text(c, t.Err).Color(c.Theme().Danger)
		case t.Response != nil:
			ui.Text(c, t.Response.Status).Bold()
			ui.Text(c, string(t.Response.Body)).Mono()
		default:
			ui.Text(c, a.S.Response.Title).Muted()
		}
	})
}
