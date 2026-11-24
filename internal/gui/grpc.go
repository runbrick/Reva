package gui

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/runbrick/reva/internal/grpcclient"
	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
	"github.com/runbrick/reva/internal/vars"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

const grpcPreviewBytes = 4 << 10

type grpcState struct {
	tabs map[int]*grpcTab
}

type grpcTab struct {
	requestID string
	metadata  kvTableState
	section   int
	effect    *ui.EffectState[grpcclient.Result, *grpcclient.Request]
	job       *grpcclient.Request
	result    *grpcclient.Result
	err       error
	output    responseCodeView
}

func (a *App) newGRPCRequest(node collectionNode) {
	request := model.NewRequest(storage.NewID())
	request.Method, request.Name, request.Body.Mode = model.MethodGRPC, a.S.GRPC.NewRequest, model.BodyRaw
	if node.collectionID != "" {
		a.saveNewCollectionRequest(node, request)
		return
	}
	a.NewTab().Request = request
	a.Main, a.Section = PageWorkbench, SectionAPIs
}

func (a *App) grpcTab(tab *Tab) *grpcTab {
	if a.grpc.tabs == nil {
		a.grpc.tabs = make(map[int]*grpcTab)
	}
	state := a.grpc.tabs[tab.ID]
	if state == nil || state.requestID != tab.Request.ID {
		state = &grpcTab{requestID: tab.Request.ID}
		state.effect = ui.NewEffectKeyed(func(ctx context.Context, job *grpcclient.Request) (grpcclient.Result, error) {
			return grpcclient.Call(ctx, *job)
		})
		a.grpc.tabs[tab.ID] = state
	}
	return state
}

func (a *App) declareGRPC(c ui.C) {
	for id, state := range a.grpc.tabs {
		index := a.requestTabIndex(id)
		if index < 0 || a.tabs[index].Request.Method != model.MethodGRPC || a.tabs[index].Request.ID != state.requestID {
			delete(a.grpc.tabs, id)
			continue
		}
		if state.job == nil {
			continue
		}
		ui.Effect(c.KeyInt(id), state.effect, state.job)
		if result, err, ok := state.effect.Take(); ok {
			state.job, state.result, state.err = nil, &result, err
			if len(result.Message) != 0 {
				shown := result.Message[:min(len(result.Message), grpcPreviewBytes)]
				var preview strings.Builder
				for offset := 0; offset < len(shown); offset += 24 {
					if offset > 0 {
						preview.WriteByte('\n')
					}
					preview.WriteString(hex.EncodeToString(shown[offset:min(offset+24, len(shown))]))
				}
				state.output.set(preview.String(), nil)
			}
		}
	}
}

func (a *App) invokeGRPC(tab *Tab, state *grpcTab) {
	if state.job != nil {
		return
	}
	values := a.VarsFor(tab)
	options := tab.Request.Options.Resolve(a.Svc.Settings.Get())
	timeout := time.Duration(options.TimeoutMs) * time.Millisecond
	if options.TimeoutMs == model.NoTimeout {
		timeout = grpcclient.NoTimeout
	}
	state.job = &grpcclient.Request{
		URL: vars.Expand(tab.Request.URL, values), Hex: vars.Expand(tab.Request.Body.Text, values),
		Metadata: vars.ExpandKVs(tab.Request.Headers, values), Timeout: timeout,
		InsecureTLS: options.VerifyTLS == model.ToggleOff,
	}
	state.err, state.result, state.output = nil, nil, responseCodeView{}
}

func (a *App) grpcPane(c ui.C, tab *Tab) {
	state, labels := a.grpcTab(tab), a.S.GRPC
	theme := c.Theme()
	ui.Column(c).Fill().Pad(Space4).Gap(Space3).Children(func() {
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			a.MethodBadge(c, tab.Request.Method)
			urlField := ui.TextField(c, &tab.Request.URL, labels.URL, labels.URLHint).Grow(1).MinWidth(0)
			if urlField.Changed() {
				tab.Dirty = true
			}
			if state.job != nil {
				if ui.Button(c, labels.Cancel).Icon(icons.X).Shrink(0).Clicked() {
					a.Cancel(tab)
				}
			} else if ui.Button(c, labels.Invoke).Icon(icons.Play).Shrink(0).Disabled(strings.TrimSpace(tab.Request.URL) == "").Clicked() || urlField.Submitted() && strings.TrimSpace(tab.Request.URL) != "" {
				a.invokeGRPC(tab, state)
			}
			if ui.Button(c, labels.Save).Icon(icons.Save).Ghost().Shrink(0).Clicked() {
				c.After(func() { a.RequestSave(tab) })
			}
		})
		ui.Tabs(c, &state.section, []int{0, 1}, func(section int) string {
			return [...]string{labels.Payload, labels.Metadata}[section]
		})
		ui.Scroll(c.KeyInt(state.section)).FillWidth().Grow(1).MinHeight(0).Scrollbars().Label(labels.RequestContent).Children(func() {
			ui.Column(c).FillWidth().Gap(Space2).Children(func() {
				if state.section == 1 {
					ui.Wrap(c, labels.MetadataHint).Muted().FontSize(theme.Metrics.FontSizeS)
					if a.kvTableWithLabels(c, &state.metadata, &tab.Request.Headers, "", labels.MetadataKey, labels.MetadataValue, a.S.Request.Description, labels.AddMetadata) {
						tab.Dirty = true
					}
					return
				}
				ui.Text(c, labels.Input).Bold()
				ui.Wrap(c, labels.InputHint).Muted().FontSize(theme.Metrics.FontSizeS)
				if ui.TextArea(c, &tab.Request.Body.Text, labels.Input, "").Mono().FillWidth().Height(4 * Space6).Changed() {
					tab.Dirty = true
				}
			})
		})
		ui.Divider(c)
		a.grpcResponse(c, state)
	})
	if ui.Shortcut(c, ui.Primary, "s") {
		c.After(func() { a.RequestSave(tab) })
	}
}

func (a *App) grpcResponse(c ui.C, state *grpcTab) {
	labels, theme := a.S.GRPC, c.Theme()
	ui.Column(c).FillWidth().Grow(1).MinHeight(0).Gap(Space2).Children(func() {
		ui.Text(c, labels.Response).Bold()
		if state.job != nil {
			ui.Text(c, labels.Calling).Muted()
			return
		}
		result := state.result
		if state.err != nil {
			ui.Wrap(c, a.grpcError(state.err, result)).Color(theme.Danger)
			if result != nil {
				ui.Text(c, fmt.Sprintf(labels.Time, result.Duration.Round(time.Millisecond))).Muted()
			}
			return
		}
		if result == nil {
			ui.Text(c, labels.Waiting).Muted()
			return
		}
		ui.Row(c).FillWidth().Gap(Space3).Items(ui.Center).Children(func() {
			color := theme.Success
			if result.Status != 0 {
				color = theme.Danger
			}
			ui.Text(c, fmt.Sprintf(labels.Status, result.Status)).Bold().Color(color)
			ui.Text(c, fmt.Sprintf(labels.Time, result.Duration.Round(time.Millisecond))).Muted()
			if result.Message != nil {
				ui.Text(c, fmt.Sprintf(labels.Size, len(result.Message))).Muted()
			}
		})
		if result.StatusMessage != "" {
			ui.Scroll(c.Key("status-message")).FillWidth().MaxHeight(3 * Space6).Shrink(0).Scrollbars().Children(func() {
				ui.Wrap(c, result.StatusMessage)
			})
		}
		if result.Message == nil {
			ui.Text(c, labels.NoPayload).Muted()
			return
		}
		if len(result.Message) == 0 {
			ui.Text(c, labels.EmptyPayload).Muted()
			return
		}
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			ui.Text(c, labels.Output).Grow(1).MinWidth(0).Ellipsis()
			if ui.Button(c, labels.CopyHex).Icon(icons.Copy).Ghost().Shrink(0).Clicked() {
				if err := app.SetClipboardText(hex.EncodeToString(result.Message)); err != nil {
					a.Toast(a.S.Response.CopyError, "", ui.ToastError)
				}
			}
		})
		if len(result.Message) > grpcPreviewBytes {
			ui.Text(c, labels.PreviewLimit).Muted().FontSize(theme.Metrics.FontSizeS)
		}
		state.output.declare(c, labels.Output, a.palette, true, nil)
	})
}

func (a *App) grpcError(err error, result *grpcclient.Result) string {
	labels := a.S.GRPC
	switch {
	case errors.Is(err, context.Canceled):
		return labels.Cancelled
	case errors.Is(err, context.DeadlineExceeded):
		return labels.Timeout
	case errors.Is(err, grpcclient.ErrURL):
		return labels.InvalidURL
	case errors.Is(err, grpcclient.ErrHex):
		return labels.InvalidHex
	case errors.Is(err, grpcclient.ErrMetadata):
		return labels.InvalidMetadata
	case errors.Is(err, grpcclient.ErrLimit):
		return labels.Limit
	case errors.Is(err, grpcclient.ErrHTTPStatus) && result != nil:
		return fmt.Sprintf(labels.HTTPStatus, result.HTTPStatus)
	case errors.Is(err, grpcclient.ErrProtocol):
		return labels.Protocol
	case errors.Is(err, grpcclient.ErrCompression):
		return labels.Compression
	default:
		return labels.Transport
	}
}
