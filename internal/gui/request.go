package gui

import "github.com/vivaui/viva/ui"

// requestState is the request editor's UI state (selected section, table
// editors, ...). Owned by the request editor PR.
type requestState struct{}

// requestPane declares the request editor for t: method, URL, Send/Cancel,
// and sections for params, headers, body, auth and scripts.
func (a *App) requestPane(c ui.C, t *Tab) {
	ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Items(ui.Center).Children(func() {
			ui.TextField(c, &t.Request.URL, "URL", "https://").Grow(1)
			if t.Sending {
				if ui.Button(c, a.S.Common.Cancel).Clicked() {
					a.Cancel(t)
				}
			} else if ui.Button(c, "Send").Clicked() {
				a.Send(t)
			}
		})
	})
}
