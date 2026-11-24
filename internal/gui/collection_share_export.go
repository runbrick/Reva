package gui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/shareexport"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type collectionShareExportState struct {
	open, confirmed  bool
	format, selected shareexport.Format
	preview          shareexport.Preview
	fileView         responseCodeView
	notices          []string
	summary          string
	problem          string
	job              *collectionShareWrite
	effect           *ui.EffectState[struct{}, *collectionShareWrite]
}

type collectionShareWrite struct {
	path string
	data []byte
}

var collectionShareFormats = [...]shareexport.Format{shareexport.Native, shareexport.Postman, shareexport.Hurl, shareexport.K6}

func (a *App) collectionShareExportEntry(c ui.C, tab *Tab, exists bool) {
	labels := &a.S.Collections
	ui.Row(c).FillWidth().PadTop(Space4).Gap(Space2).Items(ui.Center).Children(func() {
		ui.Text(c, labels.ExportSection).Muted().Grow(1).FontSize(c.Theme().Metrics.FontSizeS)
		if ui.Button(c, labels.ShareExport).Small().Outline().Icon(icons.Download).Disabled(!exists).Clicked() {
			c.After(func() {
				draft := a.collections.settings[tab.ID]
				draft.shareExport = collectionShareExportState{open: true}
				a.buildCollectionShareExport(draft, tab.CollectionID, shareexport.Native)
			})
		}
	})
}

func (a *App) buildCollectionShareExport(draft *collectionSettingsDraft, collectionID string, format shareexport.Format) {
	state, labels := &draft.shareExport, &a.S.Collections
	*state = collectionShareExportState{open: true, format: format, selected: format}
	col, ok := a.Svc.Collections.Get(collectionID)
	if !ok {
		state.problem = labels.SettingsMissing
		return
	}
	preview, err := shareexport.Build(col, format)
	if err != nil {
		state.problem = labels.ShareInvalid
		if errors.Is(err, shareexport.ErrLimit) {
			state.problem = labels.ShareLimit
		}
		return
	}
	state.preview = preview
	state.summary = fmt.Sprintf(labels.ShareCounts, preview.Included, preview.Requests)
	for _, notice := range preview.Notices {
		line := shareIssueLabel(a.S, notice.Issue)
		if notice.Request > 0 {
			prefix := labels.ShareRequest
			if notice.Skipped {
				prefix = labels.ShareSkipped
			}
			line = fmt.Sprintf(prefix, notice.Request, line)
		}
		state.notices = append(state.notices, line)
	}
	if len(preview.Data) == 0 {
		state.problem = labels.ShareEmpty
		return
	}
	state.fileView.set(string(preview.Data), nil)
}

func (a *App) collectionShareExportDialog(c ui.C, tab *Tab, draft *collectionSettingsDraft) {
	state, labels := &draft.shareExport, &a.S.Collections
	if !state.open {
		return
	}
	if state.job != nil {
		ui.Effect(c, state.effect, state.job)
		if _, err, done := state.effect.Take(); done {
			state.job, state.effect = nil, nil
			if err == nil {
				*state = collectionShareExportState{}
				a.Toast(labels.Exported, "", ui.ToastSuccess)
				return
			}
			state.problem = labels.ShareSaveFailed
		}
	}
	theme := c.Theme()
	spec := ui.ModalSpec{Width: 30 * Space6, Height: 25 * Space6, NoClose: state.job != nil}
	dismissed := ui.ModalWith(c, &state.open, labels.ShareExport, spec, func() {
		ui.Column(c).Fill().Gap(Space2).Children(func() {
			if state.job == nil {
				ui.Select(c, &state.selected, collectionShareFormats[:], func(format shareexport.Format) string {
					return shareFormatLabel(labels, format)
				}).Label(labels.ShareFormat).FillWidth()
				if format := state.selected; format != state.format {
					c.After(func() { a.buildCollectionShareExport(draft, tab.CollectionID, format) })
				}
			}
			ui.Wrap(c, labels.SharePrivacy).Muted().FontSize(theme.Metrics.FontSizeS)
			ui.Wrap(c, shareFormatHelp(labels, state.format)).Muted().FontSize(theme.Metrics.FontSizeS)
			if state.summary != "" {
				ui.Text(c, state.summary).Bold()
			}
			if state.problem != "" {
				ui.Wrap(c, state.problem).Color(theme.Danger)
			}
			if len(state.notices) > 0 {
				ui.Text(c, labels.ShareNotices).Bold().FontSize(theme.Metrics.FontSizeS)
				ui.Scroll(c.Key("notices")).FillWidth().Height(4 * Space6).Scrollbars().Label(labels.ShareNotices).Children(func() {
					ui.Column(c).FillWidth().Gap(Space1).Children(func() {
						for index, notice := range state.notices {
							ui.Wrap(c.KeyInt(index), notice).FontSize(theme.Metrics.FontSizeS)
						}
					})
				})
			}
			if len(state.preview.Data) > 0 {
				ui.Text(c, labels.ShareFilePreview).Bold().FontSize(theme.Metrics.FontSizeS)
				state.fileView.declare(c.Key("file"), labels.ShareFilePreview, a.palette, false, nil)
				ui.Checkbox(c, &state.confirmed, labels.ShareConfirm).Disabled(state.job != nil)
			}
		})
	}, func() {
		if ui.Button(c, labels.Cancel).Outline().Disabled(state.job != nil).Clicked() {
			c.After(func() { *state = collectionShareExportState{} })
		}
		if ui.Button(c, labels.ShareSave).Icon(icons.Save).Disabled(!state.confirmed || len(state.preview.Data) == 0 || state.job != nil || state.selected != state.format).Clicked() {
			c.After(func() { a.saveCollectionShareExport(state) })
		}
	})
	if dismissed {
		c.After(func() { *state = collectionShareExportState{} })
	}
}

func (a *App) saveCollectionShareExport(state *collectionShareExportState) {
	labels := &a.S.Collections
	extension := state.format.Extension()
	path, err := app.SaveFile(app.SaveFileOptions{
		Title: labels.ShareSave, SuggestedName: state.format.Filename(), DefaultExtension: extension,
		Filters: []app.FileFilter{{Name: strings.ToUpper(extension), Pattern: "*." + extension}},
	})
	if errors.Is(err, app.ErrCanceled) || err == nil && path == "" {
		return
	}
	if err != nil {
		state.problem = labels.ShareSaveFailed
		return
	}
	state.problem = ""
	state.job = &collectionShareWrite{path: path, data: state.preview.Data}
	state.effect = ui.NewEffectKeyed(func(_ context.Context, job *collectionShareWrite) (struct{}, error) {
		return struct{}{}, storage.WriteFileAtomic(job.path, job.data)
	})
}

func shareFormatLabel(labels *i18n.Collections, format shareexport.Format) string {
	return [...]string{labels.ShareNative, labels.SharePostman, labels.ShareHurl, labels.ShareK6}[format]
}

func shareFormatHelp(labels *i18n.Collections, format shareexport.Format) string {
	return [...]string{labels.ShareNativeHelp, labels.SharePostmanHelp, labels.ShareHurlHelp, labels.ShareK6Help}[format]
}

func shareIssueLabel(strings *i18n.Strings, issue shareexport.Issue) string {
	labels := &strings.Collections
	switch issue {
	case shareexport.Variables:
		return labels.ShareVariables
	case shareexport.Environments:
		return labels.ShareEnvironments
	case shareexport.Credentials:
		return labels.ShareCredentials
	case shareexport.Descriptions:
		return labels.ShareDescriptions
	case shareexport.URL:
		return labels.ShareURL
	case shareexport.QuerySyntax:
		return labels.ShareQuerySyntax
	case shareexport.Values:
		return labels.ShareValues
	case shareexport.BodyValues:
		return labels.ShareBodyValues
	case shareexport.ContentType:
		return labels.ShareContentType
	case shareexport.Files:
		return labels.ShareFiles
	case shareexport.Scripts:
		return labels.ShareScripts
	case shareexport.Options:
		return labels.ShareOptions
	case shareexport.Protocol:
		return strings.GRPC.ExportProtocol
	case shareexport.BodyMode:
		return labels.ShareBodyMode
	case shareexport.Method:
		return labels.ShareMethod
	case shareexport.RowName:
		return labels.ShareRowName
	case shareexport.Disabled:
		return labels.ShareDisabled
	case shareexport.Folders:
		return labels.ShareFolders
	case shareexport.AuthOverride:
		return labels.ShareAuthOverride
	case shareexport.DuplicateHeader:
		return labels.ShareDuplicateHeader
	case shareexport.ManagedHeader:
		return labels.ShareManagedHeader
	default:
		return labels.ShareUnsupportedAuth
	}
}
