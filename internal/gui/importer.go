package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Jobs are immutable snapshots. A new pointer identifies every attempt, including
// loading the same file again; effects never read or mutate the App off-thread.
type importFile struct{ path string }
type importParseJob struct {
	input   string
	url     bool
	options importer.Options
}

type importerState struct {
	source                             importSource
	paste, details, appendToCollection bool
	basePath, host                     bool
	fileName, collectionID             string
	collectionIDs                      []string
	collectionNames                    map[string]string
	collectionLabel                    func(string) string
	open, loading, parsing, committing bool
	input, err                         string
	url                                string
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

type importSource uint8

const (
	importAuto importSource = iota
	importNative
	importPostman
	importApifox
	importApiPost
	importCurl
	importHAR
	importSwagger
	importSwaggerURL
	importUnsupported
)

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
	input, options := job.input, job.options
	if job.url {
		var err error
		input, options.DocumentURL, err = importer.FetchDocument(ctx, job.input)
		if err != nil {
			return importer.Result{}, err
		}
	}
	result, err := importer.ParseWithOptions(input, options)
	if cancel := ctx.Err(); cancel != nil {
		return importer.Result{}, cancel
	}
	return result, err
}

func (s *importerState) content() string {
	if s.source == importSwaggerURL {
		return s.url
	}
	return s.input
}

func (s *importerState) pathOptions() importer.Options {
	return importer.Options{AddBasePath: s.basePath, AddHost: s.host}
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
		s := &a.importer
		s.collectionIDs = s.collectionIDs[:0]
		s.collectionNames = make(map[string]string)
		for _, col := range a.Svc.Collections.List() {
			s.collectionIDs = append(s.collectionIDs, col.ID)
			s.collectionNames[col.ID] = col.Name
		}
		if _, ok := s.collectionNames[s.collectionID]; !ok {
			s.collectionID = ""
			s.appendToCollection = false
			if tab := a.Active(); tab != nil {
				s.collectionID = tab.CollectionID
			}
			if s.collectionID == "" && len(s.collectionIDs) > 0 {
				s.collectionID = s.collectionIDs[0]
			}
		}
		if s.collectionLabel == nil {
			s.collectionLabel = func(id string) string { return s.collectionNames[id] }
		}
		a.importer.open = true
	}
}

func (a *App) importDialog(c ui.C) {
	s := &a.importer
	if !s.open {
		s.dismiss()
		return
	}
	if s.preview != nil && s.reviewedInput != s.content() {
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
				s.fileName = filepath.Base(s.file.path)
			}
		}
	}
	if s.parsing {
		ui.Effect(c, s.parse, s.job)
		if result, err, ok := s.parse.Take(); ok {
			s.parsing = false
			// Even a programmatic edit cannot apply a result for older input.
			if s.job.input == s.content() && s.job.url == (s.source == importSwaggerURL) && s.job.options == s.pathOptions() {
				if err != nil {
					s.err = a.importError(err, a.S.Import.Unknown)
				} else if !s.accepts(result.Kind) {
					s.err = a.S.Import.SourceMismatch
				} else if destinationError := a.importDestinationError(result); destinationError != "" {
					s.err = destinationError
				} else {
					s.preview, s.reviewedInput, s.acknowledged = &result, s.job.input, false
				}
			}
		}
	}
	busy := s.loading || s.parsing || s.committing
	ui.ModalWith(c, &s.open, a.S.Import.ManualTitle, ui.ModalSpec{
		Description: a.S.Import.Description, Width: Space6 * 44, Height: min(Space6*31, c.Bounds().Height-Space4*2),
	}, func() {
		gap := Space2
		if c.Bounds().Height < Space6*30 {
			gap = Space1
		}
		ui.Column(c).Fill().Gap(gap).Children(func() {
			if s.preview == nil {
				a.importSetup(c, busy)
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
			if ui.Button(c, a.S.Import.Review).Disabled(busy || strings.TrimSpace(s.content()) == "").Clicked() {
				a.reviewImport()
			}
		} else {
			if ui.Button(c, a.S.Import.Back).Outline().Disabled(busy).Clicked() {
				s.invalidateReview()
				s.paste = true
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

func (s *importerState) accepts(kind importer.Kind) bool {
	switch s.source {
	case importNative:
		return kind == importer.KindNativeCollection || kind == importer.KindNativeEnvironment
	case importPostman, importApifox, importApiPost:
		return kind == importer.KindPostman || kind == importer.KindPostmanEnvironment
	case importCurl:
		return kind == importer.KindCurl
	case importHAR:
		return kind == importer.KindHAR
	case importSwagger, importSwaggerURL:
		return kind == importer.KindSwagger || kind == importer.KindOpenAPI
	default:
		return s.source == importAuto
	}
}

func (a *App) importSourceHelp() string {
	s, text := &a.importer, a.S.Import
	switch s.source {
	case importNative:
		return text.NativeHelp
	case importPostman:
		return text.PostmanHelp
	case importApifox:
		return text.ApifoxHelp
	case importApiPost:
		return text.ApiPostHelp
	case importCurl:
		return text.CurlHelp
	case importHAR:
		return text.HARHelp
	case importSwagger:
		return text.SwaggerHelp
	case importSwaggerURL:
		return text.SwaggerURLHelp
	default:
		return text.AutoHelp
	}
}

func (a *App) importSetup(c ui.C, busy bool) {
	s, text, theme := &a.importer, a.S.Import, c.Theme()
	compact := c.Bounds().Height < Space6*30
	ui.Row(c).FillWidth().Items(ui.Center).Children(func() {
		ui.Text(c, text.Sources)
		ui.Spacer(c).Grow(1)
		if compact {
			ui.Text(c, text.MoreSources).FontSize(theme.Metrics.FontSizeS).Muted()
		}
	})
	if compact {
		ui.Scroll(c).FillWidth().Height(theme.Metrics.ControlHeight + Space2).Shrink(0).Scrollbars().Children(func() { a.importSources(c, busy, compact) })
	} else {
		a.importSources(c, busy, compact)
	}
	ui.Divider(c)
	ui.Flow(c).FillWidth().Gap(Space3).Items(ui.Center).Children(func() {
		ui.Text(c, text.Location).Width(Space6 * 4).Shrink(0)
		a.importLocationOption(c, true, text.CurrentCollection, busy || len(s.collectionIDs) == 0)
		a.importLocationOption(c, false, text.NewCollection, busy)
		if s.appendToCollection {
			if busy {
				ui.Text(c, s.collectionNames[s.collectionID]).Ellipsis().MaxWidth(Space6 * 10)
			} else if ui.Select(c, &s.collectionID, s.collectionIDs, s.collectionLabel).Label(text.SelectCollection).Width(Space6 * 10).Changed() {
				s.invalidateReview()
			}
		}
	})
	ui.Divider(c)
	ui.Flow(c).FillWidth().Gap(Space3).Items(ui.Center).Children(func() {
		ui.Text(c, text.PathHandling).Width(Space6 * 4).Shrink(0)
		pathDisabled := busy || s.source != importSwagger && s.source != importSwaggerURL && s.source != importAuto
		ui.Row(c).Gap(Space2).Items(ui.Center).Children(func() {
			ui.Text(c, text.AddBasePath).Muted()
			if ui.Switch(c, &s.basePath, "").Label(text.AddBasePath).Disabled(pathDisabled).Changed() {
				s.invalidateReview()
			}
		})
		ui.Row(c).Gap(Space2).Items(ui.Center).Children(func() {
			ui.Text(c, text.AddHost).Muted()
			if ui.Switch(c, &s.host, "").Label(text.AddHost).Disabled(pathDisabled).Changed() {
				s.invalidateReview()
			}
		})
		ui.Button(c, text.KeepPaths).Icon(icons.Info).IconOnly().Ghost().Tooltip(text.KeepPaths)
	})
	ui.Divider(c)
	if s.source == importSwaggerURL {
		ui.Wrap(c, text.SwaggerURLHelp).FillWidth().FontSize(theme.Metrics.FontSizeS).Muted()
		if ui.TextField(c, &s.url, text.SchemaURL, text.SchemaURLPlaceholder).FillWidth().Disabled(busy).Changed() {
			s.invalidateReview()
		}
		ui.Spacer(c).Grow(1)
		return
	}
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		file := ui.Button(c, text.SelectFile).Small().Ghost().Disabled(busy)
		if !s.paste {
			file.Secondary()
		}
		if file.Clicked() {
			s.paste = false
		}
		paste := ui.Button(c, text.PasteContent).Small().Ghost().Disabled(busy)
		if s.paste {
			paste.Secondary()
		}
		if paste.Clicked() {
			s.paste = true
		}
		ui.Spacer(c).Grow(1)
		if s.paste {
			if ui.Button(c, text.LoadFile).Small().Outline().Icon(icons.Upload).Disabled(busy).Clicked() {
				c.After(func() { a.chooseImportFile() })
			}
		}
		if ui.Button(c, text.Details).Small().Link().Clicked() {
			s.details = !s.details
		}
	})
	if s.details {
		ui.Scroll(c).FillWidth().Height(Space6 * 3).Shrink(1).MinHeight(0).Scrollbars().Children(func() {
			ui.Wrap(c, text.Help).FillWidth().FontSize(theme.Metrics.FontSizeS).Muted()
		})
	}
	if s.paste {
		if ui.TextArea(c, &s.input, text.Input, text.Placeholder).FillWidth().Grow(1).MinHeight(theme.Metrics.ControlHeight).Disabled(busy).Changed() {
			s.fileName = ""
			s.invalidateReview()
		}
	} else {
		panel := ui.ButtonBase(c, text.LoadFile).FillWidth().Grow(1).MinHeight(theme.Metrics.ControlHeight*2).
			Background(a.palette.TableHeader).Border(Space1/4, theme.Border).Radius(theme.Metrics.RadiusControl).
			Hover(ui.Look{}.Border(theme.Primary).Background(theme.Hover)).Focus(ui.Look{}.Ring(Space1/2, theme.Ring)).Disabled(busy).
			HitWith(ui.ID(c), text.LoadFile, gfx.HitOptions{Role: gfx.RoleButton, Disabled: busy})
		panel.Children(func() {
			ui.Column(c).FillWidth().Center().Pad(Space3).Gap(Space2).Children(func() {
				large := !compact && !s.details && s.err == "" && !busy
				if large {
					ui.Row(c).Size(Space6*2, Space6*2).Center().Radius(theme.Metrics.RadiusSurface).Background(theme.Secondary).Children(func() {
						ui.Icon(c, icons.Upload).Size(Space6, Space6).Color(theme.MutedForeground)
					})
				}
				if s.fileName != "" {
					ui.Text(c, fmt.Sprintf(text.FileLoaded, s.fileName, float64(len(s.input))/1024)).Ellipsis().FillWidth().TextCenter().Color(theme.Primary)
				} else {
					ui.Text(c, text.LoadFile).Color(theme.Primary)
				}
				ui.Wrap(c, a.importSourceHelp()).FillWidth().TextCenter().FontSize(theme.Metrics.FontSizeS).Muted()
				if large {
					ui.Wrap(c, text.UploadHint).FillWidth().TextCenter().FontSize(theme.Metrics.FontSizeS).Muted()
				}
			})
		})
		if !busy && panel.Clicked() {
			c.After(func() { a.chooseImportFile() })
		}
	}
}

func (a *App) importLocationOption(c ui.C, appendToCollection bool, label string, disabled bool) {
	s, theme := &a.importer, c.Theme()
	b := ui.ButtonBase(c, label).Height(theme.Metrics.ControlHeight).Gap(Space2).Disabled(disabled).
		HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleRadio, Disabled: disabled, Checked: s.appendToCollection == appendToCollection})
	if disabled {
		b.Opacity(0.5)
	}
	b.Children(func() {
		icon, color := icons.Circle, theme.Border
		if s.appendToCollection == appendToCollection {
			icon, color = icons.CircleCheck, theme.Primary
		}
		ui.Icon(c, icon).Size(Space4, Space4).Color(color)
		ui.Text(c, label)
	})
	if appendToCollection {
		b.Tooltip(a.S.Import.CurrentHelp)
		if len(s.collectionIDs) == 0 {
			b.Tooltip(a.S.Import.NoCollection)
		}
	}
	if !disabled && b.Clicked() {
		s.appendToCollection = appendToCollection
		s.invalidateReview()
	}
}

func (a *App) importSources(c ui.C, busy, compact bool) {
	s, text, theme := &a.importer, a.S.Import, c.Theme()
	sources := [...]struct {
		label  string
		source importSource
	}{
		{text.SwaggerSource, importSwagger}, {text.SwaggerURLSource, importSwaggerURL}, {text.PostmanSource, importPostman}, {text.ApifoxSource, importApifox}, {text.YAPISource, importUnsupported},
		{text.ApiPostSource, importApiPost}, {text.CurlSource, importCurl}, {text.MarkdownSource, importUnsupported}, {text.HARSource, importHAR}, {text.InsomniaSource, importUnsupported},
		{text.APIDocSource, importUnsupported}, {text.ApizzaSource, importUnsupported}, {text.EolinkSource, importUnsupported}, {text.NativeSource, importNative}, {text.AutoSource, importAuto},
	}
	if compact {
		// Keep the selected default and useful sources in the first viewport.
		ordered, at := sources, 0
		ordered[at] = sources[len(sources)-1]
		at++
		for _, entry := range sources[:len(sources)-1] {
			if entry.source != importUnsupported {
				ordered[at] = entry
				at++
			}
		}
		for _, entry := range sources {
			if entry.source == importUnsupported {
				ordered[at] = entry
				at++
			}
		}
		sources = ordered
	}
	ui.Column(c).FillWidth().Gap(Space2).Children(func() {
		for row := range 3 {
			rowC := c.KeyInt(row)
			ui.Row(rowC).FillWidth().Gap(Space3).Children(func() {
				for col := range 5 {
					i := row*5 + col
					entry, cardC := sources[i], rowC.KeyInt(col)
					selected := s.source == entry.source
					border, background := theme.Border, a.palette.Surface
					if selected {
						border, background = theme.Primary, theme.Accent
					}
					height, pad := theme.Metrics.ControlHeight+Space4, Space3
					if compact {
						height, pad = theme.Metrics.ControlHeight+Space2, Space2
					}
					card := ui.ButtonBase(cardC, entry.label).Width(0).Grow(1).MinWidth(0).Height(height).Pad(pad).Gap(Space2).
						Background(background).Border(Space1/4, border).Radius(theme.Metrics.RadiusControl).
						Hover(ui.Look{}.Border(theme.Primary)).Focus(ui.Look{}.Ring(Space1/2, theme.Ring)).Disabled(busy || entry.source == importUnsupported).
						HitWith(ui.ID(cardC), entry.label, gfx.HitOptions{Role: gfx.RoleRadio, Disabled: busy || entry.source == importUnsupported, Checked: selected})
					if entry.source == importUnsupported || busy {
						card.Opacity(0.5)
					}
					card.Children(func() {
						icon, color := icons.Braces, theme.Success
						switch entry.source {
						case importPostman, importApifox, importApiPost:
							icon, color = icons.Send, a.palette.Post
						case importCurl:
							icon, color = icons.SquareTerminal, theme.Primary
						case importHAR:
							icon, color = icons.FilePlus, a.palette.Warning
						case importNative, importAuto:
							icon, color = icons.Layers, theme.Primary
						case importUnsupported:
							color = theme.MutedForeground
						}
						ui.Icon(cardC, icon).Size(Space4, Space4).Color(color).Shrink(0)
						label := ui.Text(cardC, entry.label).Grow(1).MinWidth(0)
						if compact {
							label.FontSize(theme.Metrics.FontSizeS)
						}
						indicator, tint := icons.Circle, theme.Border
						if selected {
							indicator, tint = icons.CircleCheck, theme.Primary
						}
						ui.Icon(cardC, indicator).Size(Space4, Space4).Color(tint).Shrink(0)
					})
					if entry.source == importUnsupported {
						card.Tooltip(text.UnsupportedSource)
					}
					if !busy && entry.source != importUnsupported && card.Clicked() {
						s.source = entry.source
						s.invalidateReview()
						if s.source == importCurl {
							s.paste = true
						}
					}
				}
			})
		}
	})
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
	if len(s.content()) > importer.MaxInputBytes {
		s.err = a.S.Import.InputTooLarge
		return
	}
	if s.parse == nil {
		s.parse = ui.NewEffectKeyed(parseImport)
	}
	s.job = &importParseJob{input: s.content(), url: s.source == importSwaggerURL, options: s.pathOptions()}
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
	case importer.KindSwagger:
		format = text.SwaggerCollection
	case importer.KindOpenAPI:
		format = text.OpenAPICollection
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
	if s.appendToCollection {
		ui.Wrap(c, fmt.Sprintf(text.AppendDestination, s.collectionNames[s.collectionID])).FillWidth()
		ui.Wrap(c, text.CurrentHelp).FillWidth().FontSize(c.Theme().Metrics.FontSizeS).Muted()
	} else {
		ui.Wrap(c, text.NewEntity).FillWidth().FontSize(c.Theme().Metrics.FontSizeS).Muted()
	}
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
	if !s.open || !s.committing || s.preview == nil || s.reviewedInput != s.content() {
		return
	}
	s.committing = false
	r := s.preview
	if len(r.Warnings) > 0 && !s.acknowledged {
		return
	}
	s.err = ""
	if s.appendToCollection {
		if s.err = a.importDestinationError(*r); s.err != "" {
			return
		}
	}
	title, name := a.S.Import.Success, ""
	switch {
	case r.Collection != nil:
		var col model.Collection
		var err error
		if s.appendToCollection {
			col, _ = a.Svc.Collections.Get(s.collectionID)
			// Append to copies: failed saves must not mutate a service's cached tree.
			items := make([]model.Item, len(col.Items), len(col.Items)+1)
			copy(items, col.Items)
			auth := r.Collection.Auth
			if auth.Type == "" {
				auth.Type = model.AuthNone
			}
			col.Items = append(items, model.Item{ID: storage.NewID(), Type: model.ItemFolder, Name: r.Collection.Name, Auth: auth, Items: copyImportItems(r.Collection.Items)})
			variables := make([]model.KV, 0, len(col.Variables)+len(r.Collection.Variables))
			variables = append(variables, col.Variables...)
			col.Variables = append(variables, r.Collection.Variables...)
			err = a.Svc.Collections.Save(col)
		} else {
			col, err = a.Svc.Collections.Import(*r.Collection)
		}
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
		request, collectionID := r.Request.Clone(), ""
		if s.appendToCollection {
			collectionID = s.collectionID
			request.ID = ""
			var err error
			request, err = a.Svc.Collections.SaveRequest(collectionID, "", request)
			if err != nil {
				s.err = a.S.Import.SaveError
				return
			}
		}
		a.OpenRequest(request, collectionID)
		title = a.S.Import.RequestSuccess
	default:
		return
	}
	s.dismiss()
	s.input = ""
	s.fileName = ""
	a.Toast(title, name, ui.ToastSuccess)
}

// Recheck immediately before saving, so a deleted destination or a variable
// edited since review cannot silently change the meaning of imported requests.
func (a *App) importDestinationError(result importer.Result) string {
	if !a.importer.appendToCollection {
		return ""
	}
	col, ok := a.Svc.Collections.Get(a.importer.collectionID)
	if !ok {
		return a.S.Import.MissingDestination
	}
	if result.Environment != nil {
		return a.S.Import.EnvironmentDestination
	}
	if result.Collection != nil {
		type variable struct {
			value    string
			disabled bool
			conflict bool
		}
		existing := make(map[string]variable, len(col.Variables))
		for _, row := range col.Variables {
			prior, found := existing[row.Key]
			conflict := prior.conflict || found && (prior.value != row.Value || prior.disabled != row.Disabled)
			existing[row.Key] = variable{row.Value, row.Disabled, conflict}
		}
		for _, incoming := range result.Collection.Variables {
			if prior, found := existing[incoming.Key]; found && (prior.conflict || incoming.Value != prior.value || incoming.Disabled != prior.disabled) {
				return a.S.Import.VariableConflict
			}
		}
	}
	return ""
}

func copyImportItems(source []model.Item) []model.Item {
	items := make([]model.Item, len(source))
	for i, item := range source {
		item.ID = storage.NewID()
		if item.Request != nil {
			r := item.Request.Clone()
			r.ID = item.ID
			item.Request = &r
		}
		if item.Items != nil {
			item.Items = copyImportItems(item.Items)
		}
		items[i] = item
	}
	return items
}
