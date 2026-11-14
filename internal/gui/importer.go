package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// Jobs are immutable snapshots. A new pointer identifies every attempt, including
// loading the same file again; effects never read or mutate the App off-thread.
type importFile struct{ path string }
type importParseJob struct{ input string }

type importerState struct {
	open, loading, parsing, committing bool
	input, err                         string
	file                               *importFile
	read                               *ui.EffectState[string, *importFile]
	job                                *importParseJob
	parse                              *ui.EffectState[importer.Result, *importParseJob]
	preview                            *importer.Result
	reviewedInput                      string
	acknowledged                       bool
	warnings                           *ui.ListState
	filters                            [3]app.FileFilter
}

const maxImportFileSize = importer.MaxInputBytes

func readImportFile(ctx context.Context, file *importFile) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f, err := os.Open(file.path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, importer.MaxInputBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > importer.MaxInputBytes {
		return "", &importer.ParseError{Code: "input_too_large", Path: "$"}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return string(data), nil
}

func parseImport(ctx context.Context, job *importParseJob) (importer.Result, error) {
	if err := ctx.Err(); err != nil {
		return importer.Result{}, err
	}
	result, err := importer.Parse(job.input)
	if cancel := ctx.Err(); cancel != nil {
		return importer.Result{}, cancel
	}
	return result, err
}

func (s *importerState) invalidateReview() {
	s.preview, s.reviewedInput, s.acknowledged, s.err = nil, "", false, ""
	s.warnings = nil
}

func (s *importerState) dismiss() {
	s.open, s.loading, s.parsing, s.committing = false, false, false, false
	s.file, s.job = nil, nil
	s.invalidateReview()
}

func (a *App) importButton(c ui.C) {
	if ui.Button(c, a.S.Import.Title).Outline().Clicked() {
		a.importer.dismiss()
		a.importer.open = true
	}
}

func (a *App) importDialog(c ui.C) {
	s := &a.importer
	if !s.open {
		s.dismiss()
		return
	}
	if s.preview != nil && s.reviewedInput != s.input {
		s.invalidateReview()
	}
	if s.loading {
		ui.Effect(c, s.read, s.file)
		if text, err, ok := s.read.Take(); ok {
			s.loading = false
			if err != nil {
				s.err = a.importError(err, a.S.Import.FileError)
			} else {
				s.invalidateReview()
				s.input = text
			}
		}
	}
	if s.parsing {
		ui.Effect(c, s.parse, s.job)
		if result, err, ok := s.parse.Take(); ok {
			s.parsing = false
			// Even a programmatic edit cannot apply a result for older input.
			if s.job.input == s.input {
				if err != nil {
					s.err = a.importError(err, a.S.Import.Unknown)
				} else {
					s.preview, s.reviewedInput, s.acknowledged = &result, s.job.input, false
				}
			}
		}
	}
	busy := s.loading || s.parsing || s.committing
	ui.ModalWith(c, &s.open, a.S.Import.Title, ui.ModalSpec{
		Description: a.S.Import.Description, Width: Space6 * 30, Height: Space6 * 21,
	}, func() {
		ui.Column(c).Fill().Gap(Space2).Children(func() {
			if s.preview == nil {
				if ui.TextArea(c, &s.input, a.S.Import.Input, a.S.Import.Placeholder).FillWidth().Grow(1).MinHeight(0).Disabled(busy).Changed() {
					s.invalidateReview()
				}
				if ui.Button(c, a.S.Import.LoadFile).Outline().Disabled(busy).Clicked() {
					c.After(func() { a.chooseImportFile() })
				}
				ui.Wrap(c, a.S.Import.Help).FontSize(c.Theme().Metrics.FontSizeS).Muted().FillWidth()
			} else {
				a.importReview(c)
			}
			if s.loading {
				ui.Text(c, a.S.Import.Loading).Muted()
			}
			if s.parsing {
				ui.Text(c, a.S.Import.Parsing).Muted()
			}
			if s.err != "" {
				ui.Scroll(c).FillWidth().Height(Space6 * 3).Shrink(0).Scrollbars().Children(func() {
					ui.Wrap(c, s.err).Color(c.Theme().Danger).FillWidth()
				})
			}
		})
	}, func() {
		if ui.Button(c, a.S.Import.Cancel).Outline().Clicked() {
			s.dismiss()
		}
		if s.preview == nil {
			if ui.Button(c, a.S.Import.Review).Disabled(busy || strings.TrimSpace(s.input) == "").Clicked() {
				a.reviewImport()
			}
		} else {
			if ui.Button(c, a.S.Import.Back).Outline().Disabled(busy).Clicked() {
				s.invalidateReview()
			}
			allowed := s.preview != nil && (len(s.preview.Warnings) == 0 || s.acknowledged)
			if ui.Button(c, a.S.Import.Submit).Disabled(busy || !allowed).Clicked() {
				s.committing = true
				c.After(func() { a.performImport() })
			}
		}
	})
	// Modal dismissal (Escape, outside click, close) also invalidates late work
	// and a queued commit. The next paint stops declaring its effects.
	if !s.open {
		s.dismiss()
	}
}

func (a *App) importError(err error, fallback string) string {
	var failure *importer.ParseError
	if errors.As(err, &failure) {
		return a.S.Import.ErrorMessage(failure.Code) + "\n" + failure.Path
	}
	return fallback
}

func (a *App) chooseImportFile() {
	s := &a.importer
	if !s.open || s.loading || s.parsing || s.committing {
		return
	}
	s.filters = [3]app.FileFilter{
		{Name: a.S.Import.JSONFiles, Pattern: "*.json"},
		{Name: a.S.Import.HARFiles, Pattern: "*.har"},
		{Name: a.S.Import.AllFiles, Pattern: "*"},
	}
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: a.S.Import.LoadFile, Filters: s.filters[:]})
	if errors.Is(err, app.ErrCanceled) {
		return
	}
	if err != nil || len(paths) == 0 {
		s.err = a.S.Import.FileError
		return
	}
	if s.read == nil {
		s.read = ui.NewEffectKeyed(readImportFile)
	}
	s.invalidateReview()
	s.file = &importFile{path: paths[0]}
	s.loading = true
}

func (a *App) reviewImport() {
	s := &a.importer
	if !s.open || s.loading || s.parsing || s.committing {
		return
	}
	s.invalidateReview()
	if len(s.input) > importer.MaxInputBytes {
		s.err = a.S.Import.InputTooLarge
		return
	}
	if s.parse == nil {
		s.parse = ui.NewEffectKeyed(parseImport)
	}
	s.job = &importParseJob{input: s.input}
	s.parsing = true
}

func (a *App) importReview(c ui.C) {
	s, text := &a.importer, a.S.Import
	r := s.preview
	format := text.Curl
	switch r.Kind {
	case importer.KindNativeCollection:
		format = text.NativeCollection
	case importer.KindNativeEnvironment:
		format = text.NativeEnvironment
	case importer.KindPostman:
		format = text.PostmanCollection
	case importer.KindPostmanEnvironment:
		format = text.PostmanEnvironment
	case importer.KindHAR:
		format = text.HAR
	}
	name := text.NewRequest
	requests, folders, variables := 0, 0, 0
	if r.Collection != nil {
		name, variables = r.Collection.Name, len(r.Collection.Variables)
		requests, folders = importCounts(r.Collection.Items)
	}
	if r.Environment != nil {
		name, variables = r.Environment.Name, len(r.Environment.Variables)
	}
	if r.Request != nil {
		requests = 1
	}
	ui.Text(c, fmt.Sprintf(text.Format, format)).Bold()
	ui.Wrap(c, fmt.Sprintf(text.Destination, name)).FillWidth().MaxLines(2)
	ui.Text(c, fmt.Sprintf(text.Counts, requests, folders, variables))
	ui.Wrap(c, text.NewEntity).FillWidth().FontSize(c.Theme().Metrics.FontSizeS).Muted()
	if len(r.Warnings) == 0 {
		ui.Wrap(c, text.NoWarnings).Color(c.Theme().Success).FillWidth()
		ui.Spacer(c).Grow(1)
		return
	}
	ui.Text(c, fmt.Sprintf(text.Warnings, len(r.Warnings))).Color(a.palette.Warning)
	if s.warnings == nil {
		s.warnings = &ui.ListState{NoTypeahead: true}
	}
	s.warnings.Label = func(i int) string { return text.WarningMessage(r.Warnings[i].Code) + " " + r.Warnings[i].Path }
	// List declares only visible rows, including when a bounded input produces
	// thousands of diagnostics. Each row has the list's stable index identity.
	ui.List(c, s.warnings, len(r.Warnings), func(row ui.ListRow) {
		warning := r.Warnings[row.Index]
		row.Element.PadY(Space1).Children(func() {
			ui.Column(row.C).FillWidth().Pad(Space2).Gap(Space1).Background(a.palette.TableHeader).Radius(c.Theme().Metrics.RadiusControl).Children(func() {
				ui.Wrap(row.C, text.WarningMessage(warning.Code)).FillWidth()
				ui.Wrap(row.C, warning.Path).FillWidth().FontSize(c.Theme().Metrics.FontSizeS).Muted()
			})
		})
	}).Label(fmt.Sprintf(text.Warnings, len(r.Warnings))).FillWidth().Grow(1).MinHeight(0).Scrollbars()
	ui.Checkbox(c, &s.acknowledged, text.Acknowledge).Changed()
}

func importCounts(items []model.Item) (requests, folders int) {
	for _, item := range items {
		if item.Type == model.ItemFolder {
			folders++
			r, f := importCounts(item.Items)
			requests, folders = requests+r, folders+f
		} else if item.Request != nil {
			requests++
		}
	}
	return
}

// performImport runs only after the confirmation paint, once, on the UI thread.
// Service writes are synchronous/atomic by contract. Failed writes keep review
// and input intact for retry; environment import never creates an empty entity.
func (a *App) performImport() {
	s := &a.importer
	if !s.open || !s.committing || s.preview == nil || s.reviewedInput != s.input {
		return
	}
	s.committing = false
	r := s.preview
	if len(r.Warnings) > 0 && !s.acknowledged {
		return
	}
	s.err = ""
	title, name := a.S.Import.Success, ""
	switch {
	case r.Collection != nil:
		col, err := a.Svc.Collections.Import(*r.Collection)
		if err != nil {
			s.err = a.S.Import.SaveError
			return
		}
		name = col.Name
	case r.Environment != nil:
		env := *r.Environment
		env.ID, env.Version = storage.NewID(), model.SchemaVersion
		if err := a.Svc.Environments.Save(env); err != nil {
			s.err = a.S.Import.SaveError
			return
		}
		title, name = a.S.Import.EnvironmentSuccess, env.Name
	case r.Request != nil:
		a.OpenRequest(r.Request.Clone(), "")
		title = a.S.Import.RequestSuccess
	default:
		return
	}
	s.dismiss()
	s.input = ""
	a.Toast(title, name, ui.ToastSuccess)
}
