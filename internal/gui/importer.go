package gui

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/runbrick/reqo/internal/importer"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// importerState is the import dialog's UI state. Owned by the import PR.
type importerState struct {
	open, loading bool
	input, err    string
	file          *importFile
	read          *ui.EffectState[string, *importFile]
	filters       [1]app.FileFilter
}

type importFile struct{ path string }

// Limit file loading so an accidentally selected large file cannot exhaust the UI.
const maxImportFileSize = 32 << 20

func readImportFile(ctx context.Context, file *importFile) (string, error) {
	f, err := os.Open(file.path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxImportFileSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxImportFileSize {
		return "", errors.New("import file is too large")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return string(data), nil
}

// importButton declares the top-bar button that opens the import dialog.
func (a *App) importButton(c ui.C) {
	if ui.Button(c, a.S.Import.Title).Outline().Clicked() {
		a.importer.open = true
		a.importer.err = ""
	}
}

// importDialog declares the import dialog (Postman collection / curl).
func (a *App) importDialog(c ui.C) {
	s := &a.importer
	if !s.open {
		s.loading = false
		return
	}
	if s.loading {
		ui.Effect(c, s.read, s.file)
		if text, err, ok := s.read.Take(); ok {
			s.loading = false
			if err != nil {
				s.err = a.S.Import.FileError
			} else {
				s.input, s.err = text, ""
			}
		}
	}
	ui.ModalWith(c, &s.open, a.S.Import.Title, ui.ModalSpec{
		Description: a.S.Import.Description, Width: 720,
	}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.TextArea(c, &s.input, a.S.Import.Input, a.S.Import.Placeholder).FillWidth().Height(320).Disabled(s.loading)
			if ui.Button(c, a.S.Import.LoadFile).Outline().Disabled(s.loading).Clicked() {
				c.After(func() { a.chooseImportFile() })
			}
			if s.loading {
				ui.Text(c, a.S.Import.Loading).Muted()
			}
			if s.err != "" {
				ui.Wrap(c, s.err).Color(c.Theme().Danger).FillWidth()
			}
		})
	}, func() {
		if ui.Button(c, a.S.Import.Cancel).Outline().Clicked() {
			s.open = false
		}
		if ui.Button(c, a.S.Import.Submit).Disabled(s.loading).Clicked() {
			c.After(func() { a.performImport() })
		}
	})
}

func (a *App) chooseImportFile() {
	s := &a.importer
	s.filters[0] = app.FileFilter{Name: a.S.Import.JSONFiles, Pattern: "*.json"}
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
	s.file = &importFile{path: paths[0]}
	s.loading, s.err = true, ""
}

func (a *App) performImport() {
	s := &a.importer
	s.err = ""
	switch importer.Detect(s.input) {
	case importer.KindPostman:
		col, err := importer.Postman([]byte(s.input))
		if err != nil {
			s.err = a.S.Import.InvalidPostman
			return
		}
		col, err = a.Svc.Collections.Import(col)
		if err != nil {
			s.err = a.S.Import.SaveError
			return
		}
		a.Toast(a.S.Import.Success, col.Name, ui.ToastSuccess)
	case importer.KindCurl:
		req, err := importer.Curl(s.input)
		if err != nil {
			s.err = a.S.Import.InvalidCurl
			return
		}
		a.OpenRequest(req, "")
	default:
		if strings.HasPrefix(strings.TrimSpace(s.input), "{") {
			s.err = a.S.Import.InvalidPostman
		} else {
			s.err = a.S.Import.Unknown
		}
		return
	}
	s.open, s.input = false, ""
}
