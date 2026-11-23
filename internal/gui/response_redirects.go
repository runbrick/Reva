package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type responseRedirectState struct {
	source  *model.Response
	lang    i18n.Lang
	rows    []responseRedirectRow
	summary string
}

type responseRedirectRow struct {
	heading, target, outcome string
}

func (state *responseRedirectState) prepare(response *model.Response, lang i18n.Lang, labels i18n.ResponseRedirects) {
	if state.source == response && state.lang == lang {
		return
	}
	state.source, state.lang = response, lang
	state.rows = make([]responseRedirectRow, len(response.Redirects))
	var summary strings.Builder
	summary.WriteString(labels.Title)
	for index, hop := range response.Redirects {
		method := string(hop.Method)
		if method == "" {
			method = labels.HiddenMethod
		}
		if hop.NextMethod != "" {
			pattern := labels.MethodChange
			if !hop.Followed && hop.Reason != model.RedirectTransportError {
				pattern = labels.PlannedMethod
			}
			method = fmt.Sprintf(pattern, method, hop.NextMethod)
		}
		target := hop.Target
		if target == "" {
			target = labels.NoOrigin
		}
		auth := labels.AuthNotRemoved
		if hop.AuthRemoved {
			auth = labels.AuthRemoved
		}
		row := responseRedirectRow{
			heading: fmt.Sprintf(labels.Hop, index+1, method, hop.StatusCode, float64(hop.Duration)/float64(time.Millisecond)),
			target:  target,
			outcome: fmt.Sprintf(labels.Outcome, responseRedirectReason(hop.Reason, labels), auth),
		}
		state.rows[index] = row
		fmt.Fprintf(&summary, "\n\n%s\n%s\n%s", row.heading, row.target, row.outcome)
	}
	state.summary = summary.String()
}

func responseRedirectReason(reason model.RedirectReason, labels i18n.ResponseRedirects) string {
	switch reason {
	case model.RedirectFollowed:
		return labels.Followed
	case model.RedirectDisabled:
		return labels.Disabled
	case model.RedirectLimit:
		return labels.Limit
	case model.RedirectPolicy:
		return labels.Policy
	case model.RedirectMissingTarget:
		return labels.MissingTarget
	case model.RedirectMalformedTarget:
		return labels.MalformedTarget
	case model.RedirectTransportError:
		return labels.TransportError
	default:
		return labels.NotSent
	}
}

func (a *App) responseRedirects(c ui.C, response *model.Response) {
	lang := a.S.Lang
	labels := i18n.ResponseRedirectsFor(lang)
	state := ui.State[responseRedirectState](c, "redirects")
	state.prepare(response, lang, labels)
	theme := c.Theme()
	ui.Column(c).FillWidth().Gap(Space2).Children(func() {
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			ui.Text(c, labels.Title).Bold().Grow(1).Ellipsis()
			if ui.Button(c, labels.Copy).Icon(icons.Copy).Small().Outline().Clicked() {
				summary := state.summary
				c.After(func() {
					if err := app.SetClipboardText(summary); err != nil {
						a.Toast(labels.CopyError, "", ui.ToastError)
					}
				})
			}
		})
		for index, row := range state.rows {
			ui.Column(c.KeyInt(index)).FillWidth().Pad(Space2).Gap(Space1).
				Radius(theme.Metrics.RadiusControl).Background(theme.Card).Children(func() {
				ui.Wrap(c, row.heading).Bold().FillWidth()
				ui.Wrap(c, row.target).Mono().FillWidth()
				ui.Wrap(c, row.outcome).Muted().FontSize(theme.Metrics.FontSizeS).FillWidth()
			})
		}
	})
}
