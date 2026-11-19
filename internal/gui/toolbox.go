package gui

import (
	"context"
	"io"
	"os"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/toolbox"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type toolboxState struct {
	selected toolbox.Tool
	closed   [4]bool
	drafts   [toolbox.Count]toolboxDraft
	job      *toolboxWork
	effect   *ui.EffectState[toolboxResult, *toolboxWork]
}

type toolboxDraft struct {
	input, aux string
	alternate  bool
	algorithm  toolbox.Algorithm
	result     toolboxResult
	ready      bool
	err        error
	view       responseCodeView
}

type toolboxWork struct {
	job  toolbox.Job
	path string
}

type toolboxResult struct {
	text   string
	image  gfx.Bitmap
	color  [4]byte
	swatch bool
}

var toolboxGroups = [...][]toolbox.Tool{
	{toolbox.Base64, toolbox.URL, toolbox.JWT, toolbox.JSONEscape, toolbox.Unicode},
	{toolbox.Hash, toolbox.HMAC, toolbox.AES},
	{toolbox.JSON, toolbox.XML, toolbox.Hex, toolbox.Image, toolbox.Color},
	{toolbox.Timestamp, toolbox.UUID, toolbox.Regex},
}

var toolboxIcons = [toolbox.Count]gfx.Icon{
	toolbox.Base64: icons.HardDrive, toolbox.URL: icons.Link, toolbox.JWT: icons.User,
	toolbox.JSONEscape: icons.Braces, toolbox.Unicode: icons.Globe, toolbox.Hash: icons.Hash,
	toolbox.HMAC: icons.Hash, toolbox.AES: icons.LockKeyhole, toolbox.JSON: icons.Braces,
	toolbox.XML: icons.SquareTerminal, toolbox.Hex: icons.Binary, toolbox.Image: icons.Image,
	toolbox.Color: icons.Palette, toolbox.Timestamp: icons.Clock, toolbox.UUID: icons.Fingerprint, toolbox.Regex: icons.Regex,
}

var toolboxToggle = []bool{false, true}
var toolboxHashes = []toolbox.Algorithm{toolbox.SHA256, toolbox.SHA512, toolbox.SHA1, toolbox.MD5}

func (a *App) toolboxName(tool toolbox.Tool) string {
	labels := a.S.Toolbox
	return [...]string{"Base64", "URL", "JWT", labels.JSONEscape, labels.Unicode, labels.Hash, "HMAC", labels.AES,
		"JSON", "XML", "Hex", labels.Image, labels.Color, labels.Timestamp, "UUID", labels.Regex}[tool]
}

func (a *App) toolboxPage(c ui.C) {
	c = c.Key("toolbox-page")
	ui.Row(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
		a.toolboxNavigation(c)
		ui.Divider(c).Vertical()
		a.toolboxEditor(c.KeyInt(int(a.toolbox.selected)))
	})
}

func (a *App) toolboxNavigation(c ui.C) {
	state, theme, labels := &a.toolbox, c.Theme(), a.S.Toolbox
	groupNames := [...]string{labels.Encoding, labels.Crypto, labels.Views, labels.Other}
	ui.Column(c).Width(Space6 * 16).Shrink(0).FillHeight().Background(a.palette.Sidebar).Children(func() {
		ui.Text(c, labels.Title).FontSize(theme.Metrics.FontSizeL).Bold().Pad(Space4)
		ui.Divider(c)
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Scrollbars().Children(func() {
			ui.Column(c).FillWidth().Pad(Space3).Gap(Space4).Children(func() {
				for index, tools := range toolboxGroups {
					group := c.KeyInt(index)
					ui.Column(group).FillWidth().Gap(Space1).Children(func() {
						open := !state.closed[index]
						chevron := icons.ChevronRight
						if open {
							chevron = icons.ChevronDown
						}
						header := ui.Row(group).FillWidth().Height(theme.Metrics.ControlHeight).Items(ui.Center).PadX(Space2).
							Radius(theme.Metrics.RadiusControl).HitWith(ui.ID(group), groupNames[index], gfx.HitOptions{Role: gfx.RoleButton, Expandable: true, Expanded: open}).
							Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring)).Children(func() {
							ui.Text(group, groupNames[index]).Bold().Grow(1)
							ui.Icon(group, chevron).Size(Space4, Space4).Color(theme.MutedForeground)
						})
						if header.Clicked() {
							group.After(func() { state.closed[index] = open })
						}
						if open {
							for start := 0; start < len(tools); start += 4 {
								row := group.KeyInt(start)
								ui.Row(row).FillWidth().Gap(Space1).Children(func() {
									for _, tool := range tools[start:min(start+4, len(tools))] {
										a.toolboxTile(row.KeyInt(int(tool)), tool)
									}
								})
							}
						}
					})
				}
			})
		})
	})
}

func (a *App) toolboxTile(c ui.C, tool toolbox.Tool) {
	state, theme, name := &a.toolbox, c.Theme(), a.toolboxName(tool)
	selected := state.selected == tool
	iconColor := theme.MutedForeground
	tile := ui.Column(c).Size(Space4*5+Space2, Space6*3).Shrink(0).Gap(Space2).Center().
		Radius(theme.Metrics.RadiusControl).HitWith(ui.ID(c), name, gfx.HitOptions{Role: gfx.RoleButton, Selected: selected}).Tooltip(name).
		Hover(ui.Look{}.Background(theme.Hover)).Focus(ui.Look{}.Ring(2, theme.Ring))
	textColor := theme.Foreground
	if selected {
		tile.Background(a.palette.RailActive)
		iconColor = theme.AccentForeground
		textColor = theme.AccentForeground
	}
	tile.Children(func() {
		ui.Icon(c, toolboxIcons[tool]).Size(Space6, Space6).Color(iconColor)
		ui.Text(c, name).FontSize(theme.Metrics.FontSizeS).Color(textColor)
	})
	if tile.Clicked() {
		c.After(func() { state.selected = tool })
	}
}

func (a *App) toolboxEditor(c ui.C) {
	state, labels, theme := &a.toolbox, a.S.Toolbox, c.Theme()
	tool := state.selected
	draft := &state.drafts[tool]
	ui.Scroll(c).Grow(1).MinWidth(0).FillHeight().Scrollbars().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(Space6 * 38).Pad(Space4).Gap(Space3).Children(func() {
			ui.Row(c).FillWidth().Items(ui.Center).Gap(Space2).Children(func() {
				ui.Icon(c, toolboxIcons[tool]).Size(Space6, Space6).Color(theme.MutedForeground)
				ui.Text(c, a.toolboxName(tool)).FontSize(theme.Metrics.FontSizeL).Bold().Grow(1)
				busy := state.job != nil && state.job.job.Tool == tool
				if ui.Button(c, labels.Clear).Icon(icons.Trash2).Ghost().Disabled(draft.input == "" && draft.aux == "" && !draft.ready && draft.err == nil && !busy).Clicked() {
					draft.input, draft.aux = "", ""
					state.invalidate(tool)
				}
			})
			ui.Divider(c)
			a.toolboxOptions(c, tool, draft)
			if tool != toolbox.UUID {
				ui.Text(c, labels.Input).Bold()
				if ui.TextArea(c, &draft.input, labels.Input, a.toolboxPlaceholder(tool)).Mono().FillWidth().Height(Space6 * 6).Changed() {
					state.invalidate(tool)
				}
			}
			a.toolboxActions(c, tool, draft)
			a.toolboxOutput(c, tool, draft)
		})
	})
}

func (a *App) toolboxOptions(c ui.C, tool toolbox.Tool, draft *toolboxDraft) {
	labels, theme := a.S.Toolbox, c.Theme()
	toggle := func(falseLabel, trueLabel string, value *bool) {
		if ui.Choice(c, value, toolboxToggle, func(selected bool) string {
			if selected {
				return trueLabel
			}
			return falseLabel
		}).Changed() {
			a.toolbox.invalidate(tool)
		}
	}
	switch tool {
	case toolbox.Base64:
		toggle(labels.Standard, "Base64URL", &draft.alternate)
	case toolbox.URL:
		toggle(labels.Component, labels.Form, &draft.alternate)
	case toolbox.Hash, toolbox.HMAC:
		choices := toolboxHashes
		if tool == toolbox.HMAC {
			choices = choices[:2]
		}
		if ui.Choice(c, &draft.algorithm, choices, toolbox.Algorithm.String).Changed() {
			a.toolbox.invalidate(tool)
		}
	case toolbox.Timestamp:
		toggle(labels.Seconds, labels.Millis, &draft.alternate)
	}
	if tool == toolbox.Hash || tool == toolbox.AES || tool == toolbox.JWT || tool == toolbox.Regex {
		hint := labels.HashHint
		color := theme.MutedForeground
		switch tool {
		case toolbox.AES:
			hint = labels.AESHint
		case toolbox.JWT:
			hint = labels.JWTHint
			color = a.palette.Warning
		case toolbox.Regex:
			hint = labels.RegexHint
		}
		ui.Wrap(c, hint).FillWidth().FontSize(theme.Metrics.FontSizeS).Color(color)
	}
	var auxLabel string
	switch tool {
	case toolbox.HMAC:
		auxLabel = labels.HMACKey
	case toolbox.AES:
		auxLabel = labels.AESKey
	case toolbox.Regex:
		auxLabel = labels.Pattern
	}
	if auxLabel != "" {
		ui.Text(c, auxLabel).Bold()
		field := ui.TextField(c, &draft.aux, auxLabel, "").Mono().FillWidth()
		if tool != toolbox.Regex {
			field.Secure()
		}
		if field.Changed() {
			a.toolbox.invalidate(tool)
		}
	}
}

func (a *App) toolboxPlaceholder(tool toolbox.Tool) string {
	labels := a.S.Toolbox
	switch tool {
	case toolbox.URL:
		return labels.URLHint
	case toolbox.JSONEscape:
		return labels.JSONEscapeHint
	case toolbox.Unicode:
		return labels.UnicodeHint
	case toolbox.XML:
		return labels.XMLHint
	case toolbox.Image:
		return labels.ImageHint
	case toolbox.Color:
		return labels.ColorHint
	case toolbox.Timestamp:
		return labels.TimeHint
	default:
		return ""
	}
}

func (a *App) toolboxActions(c ui.C, tool toolbox.Tool, draft *toolboxDraft) {
	labels, state := a.S.Toolbox, &a.toolbox
	primary, secondary := labels.Run, ""
	switch tool {
	case toolbox.Base64, toolbox.URL, toolbox.JSONEscape, toolbox.Unicode, toolbox.Hex:
		primary, secondary = labels.Encode, labels.Decode
	case toolbox.AES:
		primary, secondary = labels.Encrypt, labels.Decrypt
	case toolbox.JSON:
		primary, secondary = labels.Format, labels.Compact
	case toolbox.Image, toolbox.XML, toolbox.JWT, toolbox.Color:
		primary = labels.Inspect
	case toolbox.UUID:
		primary = labels.Generate
	}
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		busy := state.job != nil && state.job.job.Tool == tool
		if ui.Button(c, primary).Disabled(busy).Clicked() {
			state.start(tool, draft, false, "")
		}
		if secondary != "" && ui.Button(c, secondary).Outline().Disabled(busy).Clicked() {
			state.start(tool, draft, true, "")
		}
		if tool == toolbox.Image && ui.Button(c, labels.ImageFile).Icon(icons.FolderOpen).Outline().Disabled(busy).Clicked() {
			c.After(func() {
				paths, err := app.OpenFiles(app.OpenFileOptions{Title: labels.ImageFile})
				if err != nil {
					state.invalidate(tool)
					draft.err = toolbox.ErrImage
				} else if len(paths) != 0 {
					draft.input = ""
					state.start(tool, draft, false, paths[0])
				}
			})
		}
		if busy {
			ui.Text(c, labels.Working).Muted()
		}
	})
}

func (a *App) toolboxOutput(c ui.C, tool toolbox.Tool, draft *toolboxDraft) {
	labels, theme := a.S.Toolbox, c.Theme()
	if draft.err != nil {
		message := labels.Invalid
		switch draft.err {
		case toolbox.ErrLimit:
			message = labels.Limit
		case toolbox.ErrKey:
			message = labels.KeyError
		case toolbox.ErrDecrypt:
			message = labels.BadCipher
		case toolbox.ErrBinary:
			message = labels.Binary
		case toolbox.ErrImage:
			message = labels.BadImage
		case toolbox.ErrNoMatch:
			ui.Wrap(c, labels.NoMatch).FillWidth().Muted()
			return
		}
		ui.Wrap(c, message).FillWidth().Color(theme.Danger)
		return
	}
	ui.Row(c).FillWidth().Items(ui.Center).Gap(Space2).Children(func() {
		ui.Text(c, labels.Output).Bold().Grow(1)
		if tool != toolbox.Image && ui.Button(c, labels.Copy).Icon(icons.Copy).Outline().Disabled(!draft.ready).Clicked() {
			c.After(func() {
				if err := app.SetClipboardText(draft.result.text); err != nil {
					a.Toast(a.S.Common.Error, labels.CopyError, ui.ToastError)
				} else {
					a.Toast(labels.Copied, "", ui.ToastSuccess)
				}
			})
		}
	})
	if bitmap := draft.result.image; bitmap.Width > 0 {
		maxSide := Space6 * 14
		scale := min(1, maxSide/float32(max(bitmap.Width, bitmap.Height)))
		ui.Image(c, bitmap).Size(float32(bitmap.Width)*scale, float32(bitmap.Height)*scale)
		return
	}
	if draft.result.swatch {
		channels := draft.result.color
		ui.Slot(c).FillWidth().Height(Space6*2).Background(gfx.RGBA(channels[0], channels[1], channels[2], float32(channels[3])/255)).
			Radius(theme.Metrics.RadiusControl).Border(1, theme.Border)
	}
	height := Space6 * 7
	if tool == toolbox.UUID || tool == toolbox.Hash || tool == toolbox.HMAC || tool == toolbox.Color || tool == toolbox.Timestamp {
		height = Space6 * 4
	}
	ui.Column(c).FillWidth().Height(height).Border(1, theme.Border).Radius(theme.Metrics.RadiusControl).Clip().Children(func() {
		if draft.ready {
			draft.view.declare(c, labels.Output, a.palette, true, nil)
		}
	})
}

func (state *toolboxState) invalidate(tool toolbox.Tool) {
	draft := &state.drafts[tool]
	draft.result, draft.ready, draft.err = toolboxResult{}, false, nil
	draft.view = responseCodeView{}
	if state.job != nil && state.job.job.Tool == tool {
		state.job = nil
	}
}

func (state *toolboxState) start(tool toolbox.Tool, draft *toolboxDraft, secondary bool, path string) {
	state.invalidate(tool)
	job := toolbox.Job{Tool: tool, Input: draft.input, Aux: draft.aux, Reverse: secondary, Alternate: draft.alternate, Algorithm: draft.algorithm}
	if tool == toolbox.JSON {
		job.Alternate = secondary
	}
	state.job = &toolboxWork{job: job, path: path}
}

func (a *App) declareToolbox(c ui.C) {
	state := &a.toolbox
	if state.job == nil {
		return
	}
	if state.effect == nil {
		state.effect = ui.NewEffectKeyed(runToolbox).Sequential()
	}
	ui.Effect(c, state.effect, state.job)
	if result, err, ok := state.effect.Take(); ok {
		draft := &state.drafts[state.job.job.Tool]
		state.job = nil
		draft.result, draft.ready, draft.err = result, err == nil, err
		if draft.ready && result.image.Width == 0 {
			draft.view.set(result.text, nil)
		}
	}
}

func runToolbox(ctx context.Context, work *toolboxWork) (toolboxResult, error) {
	if err := ctx.Err(); err != nil {
		return toolboxResult{}, err
	}
	var result toolbox.Result
	var err error
	if work.path != "" {
		result.Image, err = readToolboxImage(work.path)
	} else {
		result, err = toolbox.Run(work.job)
	}
	if err != nil {
		return toolboxResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return toolboxResult{}, err
	}
	if work.job.Tool == toolbox.Image {
		bitmap := responseImage(result.Image)
		if bitmap.Width == 0 {
			return toolboxResult{}, toolbox.ErrImage
		}
		return toolboxResult{image: bitmap}, nil
	}
	return toolboxResult{text: result.Text, color: result.Color, swatch: result.Swatch}, nil
}

func readToolboxImage(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, toolbox.ErrImage
	}
	if info.Size() > toolbox.MaxImageSize {
		return nil, toolbox.ErrLimit
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, toolbox.ErrImage
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, toolbox.MaxImageSize+1))
	if err != nil {
		return nil, toolbox.ErrImage
	}
	if len(data) > toolbox.MaxImageSize {
		return nil, toolbox.ErrLimit
	}
	return data, nil
}
