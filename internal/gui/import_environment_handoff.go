package gui

import (
	"strings"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/model"
	"github.com/vivaui/viva/ui"
)

type importEnvironmentHandoff struct {
	collection                model.Collection
	activeURLs, candidateURLs importURLReadiness
}

func (s *importerState) usesImportURL() bool {
	return s.environmentHandoff == nil && s.source == importSwaggerURL
}

func (s *importerState) canContinueWithEnvironment(kind importer.Kind) bool {
	return s.environmentHandoff == nil && !s.appendToCollection && kind == importer.KindPostman
}

func (a *App) beginEnvironmentHandoff(collection model.Collection) {
	s := &a.importer
	s.invalidateReview()
	s.input, s.fileName = "", ""
	s.file, s.job = nil, nil
	active, _ := a.Svc.Environments.Get(a.Svc.Settings.Get().ActiveEnvID)
	s.environmentHandoff = &importEnvironmentHandoff{collection: collection, activeURLs: importMissingURLNames(collection, active)}
}

func (a *App) importEnvironmentSetup(c ui.C, busy bool) {
	s, text := &a.importer, a.S.Import
	a.importEnvironmentReadiness(c, false)
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		if ui.Button(c, text.LoadFile).Small().Outline().Icon(icons.Upload).Disabled(busy).Clicked() {
			c.After(a.chooseImportFile)
		}
		if s.fileName != "" {
			ui.Text(c, s.fileName).Grow(1).MinWidth(0).Ellipsis().Muted()
		}
	})
	if ui.TextArea(c, &s.input, text.Input, text.HandoffPlaceholder).FillWidth().Grow(1).
		MinHeight(c.Theme().Metrics.ControlHeight).Disabled(busy).Changed() {
		s.fileName = ""
		s.invalidateReview()
	}
}

func (a *App) importEnvironmentReadiness(c ui.C, reviewing bool) {
	handoff, text := a.importer.environmentHandoff, a.S.Import
	ui.Wrap(c, text.HandoffSelectionHelp).FillWidth().FontSize(c.Theme().Metrics.FontSizeS).Muted()
	ui.Wrap(c, text.HandoffAdvisory).FillWidth().FontSize(c.Theme().Metrics.FontSizeS).Muted()
	a.importURLReadinessLine(c, text.HandoffActiveScope, handoff.activeURLs)
	if reviewing {
		a.importURLReadinessLine(c, text.HandoffCandidateScope, handoff.candidateURLs)
	}
}

func (a *App) importURLReadinessLine(c ui.C, scope string, readiness importURLReadiness) {
	text := a.S.Import
	status := strings.Join(readiness.names, ", ")
	if readiness.omitted {
		if status == "" {
			status = text.HandoffOmitted
		} else {
			status += " · " + text.HandoffOmitted
		}
	} else if status == "" {
		status = text.HandoffNoMissing
	}
	ui.Wrap(c, scope+" · "+status).FillWidth().FontSize(c.Theme().Metrics.FontSizeS)
}

type importURLReadiness struct {
	names   []string
	omitted bool
}

const maxImportAdvisoryNames = 5

func importMissingURLNames(collection model.Collection, env model.Environment) importURLReadiness {
	defined := make(map[string]bool)
	for _, rows := range [][]model.KV{collection.Variables, env.Variables} {
		for _, row := range rows {
			if !row.Disabled {
				defined[row.Key] = true
			}
		}
	}
	var result importURLReadiness
	seen := make(map[string]bool, maxImportAdvisoryNames)
	var visit func([]model.Item) bool
	visit = func(items []model.Item) bool {
		for _, item := range items {
			if item.Type == model.ItemFolder && !visit(item.Items) {
				return false
			}
			if item.Request == nil {
				continue
			}
			remaining := item.Request.URL
			for {
				_, afterOpen, found := strings.Cut(remaining, "{{")
				if !found {
					break
				}
				inside, afterClose, found := strings.Cut(afterOpen, "}}")
				if !found {
					break
				}
				remaining = afterClose
				name := strings.TrimSpace(inside)
				if name == "" || defined[name] || seen[name] {
					continue
				}
				if !advisoryPlaceholderName(name) {
					result.omitted = true
					continue
				}
				if len(result.names) == maxImportAdvisoryNames {
					result.omitted = true
					return false
				}
				seen[name] = true
				result.names = append(result.names, strings.Clone(name))
			}
		}
		return true
	}
	visit(collection.Items)
	return result
}

func advisoryPlaceholderName(name string) bool {
	if len(name) > 40 {
		return false
	}
	for _, char := range name {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-' || char == '.') {
			return false
		}
	}
	return true
}
