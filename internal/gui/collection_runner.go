package gui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type collectionRunnerState struct {
	open, stopOnFailure, useCookieSession, cancelling bool
	scopedEmpty                                       bool
	folderUnavailable, environmentUnavailable         bool
	collectionID, environmentID, folderID, dataFile   string
	collectionName, collectionHeading                 string
	environmentIDs                                    []string
	environmentNames                                  map[string]string
	folderIDs                                         []string
	folderNames                                       map[string]string
	problem                                           collectionRunProblem
	job                                               *collectionRunJob
	effect                                            *ui.EffectState[runner.Report, *collectionRunJob]
	report                                            runner.Report
	list                                              ui.ListState
	presets                                           collectionRunnerPresets
}

type collectionRunProblem uint8

const (
	collectionRunReady collectionRunProblem = iota
	collectionRunMissingCollection
	collectionRunTargetChanged
	collectionRunInvalidData
	collectionRunIterationLimit
)

type collectionRunJob struct {
	input    runner.Snapshot
	options  runner.Options
	dataFile string

	mu        sync.Mutex
	cancel    context.CancelFunc
	cancelled bool
}

func (job *collectionRunJob) stop() {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.cancelled = true
	if job.cancel != nil {
		job.cancel()
	}
}

func (job *collectionRunJob) run(parent context.Context, sender service.RequestService) (runner.Report, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	job.mu.Lock()
	job.cancel = cancel
	if job.cancelled {
		cancel()
	}
	input := job.input
	job.input = runner.Snapshot{}
	job.mu.Unlock()
	if job.dataFile == "" {
		return input.RunReport(ctx, sender, job.options), nil
	}
	data, err := runner.ReadDataFile(job.dataFile)
	if err != nil {
		return runner.Report{}, err
	}
	return input.RunDataReport(ctx, sender, job.options, data)
}

func (a *App) openCollectionRunner(collectionID string) {
	state := &a.collections.runner
	if state.open || state.job != nil {
		return
	}
	state.open, state.collectionID = true, collectionID
	a.collectionRunnerChoices()
	a.loadCollectionRunnerPresets()
}

func (a *App) collectionRunnerChoices() (model.Collection, model.Environment, bool) {
	state := &a.collections.runner
	state.environmentIDs = []string{""}
	state.environmentNames = make(map[string]string)
	state.folderIDs = []string{""}
	state.folderNames = make(map[string]string)
	collection, ok := a.Svc.Collections.Get(state.collectionID)
	if !ok {
		state.collectionName, state.collectionHeading = "", ""
		state.problem = collectionRunMissingCollection
		return model.Collection{}, model.Environment{}, false
	}
	text := i18n.RunnerFor(a.S.Lang)
	state.collectionName = collection.Name
	state.collectionHeading = fmt.Sprintf(text.CollectionTarget, collection.Name)
	var selected model.Environment
	for _, environment := range a.visibleEnvironments(collection) {
		if environment.ID != "" {
			state.environmentIDs = append(state.environmentIDs, environment.ID)
			state.environmentNames[environment.ID] = environment.Name
			if environment.ID == state.environmentID {
				selected = environment
			}
		}
	}
	var walk func([]model.Item, string)
	walk = func(items []model.Item, parent string) {
		for _, item := range items {
			if item.Type != model.ItemFolder || item.ID == "" {
				continue
			}
			name := item.Name
			if parent != "" {
				name = fmt.Sprintf(text.FolderPath, parent, name)
			}
			state.folderIDs = append(state.folderIDs, item.ID)
			state.folderNames[item.ID] = name
			walk(item.Items, name)
		}
	}
	walk(collection.Items, "")
	_, folderFound := state.folderNames[state.folderID]
	state.folderUnavailable = state.folderUnavailable || (state.folderID != "" && !folderFound)
	state.environmentUnavailable = state.environmentUnavailable || (state.environmentID != "" && selected.ID == "")
	return collection, selected, true
}

func (a *App) chooseCollectionRunData() {
	state, text := &a.collections.runner, i18n.RunnerFor(a.S.Lang)
	if !state.open || state.job != nil || state.report.Version != 0 {
		return
	}
	paths, err := app.OpenFiles(app.OpenFileOptions{Title: text.BrowseData, Filters: []app.FileFilter{
		{Name: "CSV", Pattern: "*.csv"}, {Name: "JSON", Pattern: "*.json"},
	}})
	if errors.Is(err, app.ErrCanceled) {
		return
	}
	if err != nil || len(paths) != 1 || paths[0] == "" {
		state.problem = collectionRunInvalidData
		return
	}
	state.dataFile, state.problem = paths[0], collectionRunReady
}

func (a *App) startCollectionRun() {
	state := &a.collections.runner
	if !state.open || state.job != nil || state.report.Version != 0 || state.presets.writing {
		return
	}
	collection, environment, ready := a.validateCollectionRunTargets()
	if !ready {
		return
	}
	input := runner.NewSnapshot(collection, a.Svc.Settings.Get(), environment.Variables)
	if state.folderID != "" {
		var err error
		input, err = input.WithFolder(state.folderID)
		if err != nil {
			state.folderUnavailable = true
			return
		}
	}
	state.job = &collectionRunJob{
		input: input, dataFile: state.dataFile,
		options: runner.Options{StopOnFailure: state.stopOnFailure, UseCookieSession: state.useCookieSession},
	}
	sender := a.Svc.Requests
	state.effect = ui.NewEffectKeyed(func(ctx context.Context, job *collectionRunJob) (runner.Report, error) {
		return job.run(ctx, sender)
	})
	state.problem = collectionRunReady
}

func (a *App) validateCollectionRunTargets() (model.Collection, model.Environment, bool) {
	state := &a.collections.runner
	collectionName, collectionWasMissing := state.collectionName, state.problem == collectionRunMissingCollection
	folderName, environmentName := state.folderNames[state.folderID], state.environmentNames[state.environmentID]
	state.problem = collectionRunReady
	collection, environment, ok := a.collectionRunnerChoices()
	if !ok || state.folderUnavailable || state.environmentUnavailable {
		return model.Collection{}, model.Environment{}, false
	}
	if collectionWasMissing || collectionName != state.collectionName ||
		folderName != state.folderNames[state.folderID] || environmentName != state.environmentNames[state.environmentID] {
		state.problem = collectionRunTargetChanged
		return model.Collection{}, model.Environment{}, false
	}
	return collection, environment, true
}

func (a *App) declareCollectionRunner(c ui.C) {
	state := &a.collections.runner
	a.declareCollectionRunnerPresets(c)
	if state.job == nil {
		return
	}
	ui.Effect(c, state.effect, state.job)
	if report, err, ok := state.effect.Take(); ok {
		state.job, state.effect, state.cancelling = nil, nil, false
		switch {
		case errors.Is(err, runner.ErrIterationLimit):
			state.problem = collectionRunIterationLimit
		case err != nil:
			state.problem = collectionRunInvalidData
		default:
			*state = collectionRunnerState{open: true, report: report, scopedEmpty: state.folderID != ""}
		}
	}
}

func (a *App) dismissCollectionRunner() {
	state := &a.collections.runner
	if state.presets.writing {
		state.open = true
		return
	}
	if state.job != nil {
		state.open, state.cancelling = true, true
		state.job.stop()
		return
	}
	*state = collectionRunnerState{}
}

func (a *App) collectionRunnerPage(c ui.C) {
	a.requestDownloads(c)
	a.declareSettingsEffects(c)
	ui.Column(c).Fill().Background(a.palette.Surface)
	a.collectionRunnerDialog(c)
}

func (a *App) collectionRunnerDialog(c ui.C) {
	state, text, theme := &a.collections.runner, a.S.Collections, c.Theme()
	spec := ui.ModalSpec{Width: 27 * Space6, Height: 17 * Space6, NoClose: state.job != nil || state.presets.writing}
	if state.job == nil && state.report.Version == 0 {
		spec.Height = 21 * Space6
	}
	dismissed := ui.ModalWith(c, &state.open, text.RunCollection, spec, func() {
		ui.Column(c).Fill().Gap(Space3).Children(func() {
			switch {
			case state.report.Version != 0:
				a.collectionRunnerSummary(c, state)
			case state.job != nil:
				status := text.RunRunning
				if state.cancelling {
					status = text.RunCancelling
				}
				ui.Text(c, status).Bold().Color(theme.Primary)
			default:
				a.collectionRunnerSetup(c)
			}
		})
	}, func() {
		if state.job != nil {
			if ui.Button(c, text.RunCancel).Outline().Icon(icons.X).Disabled(state.cancelling).Clicked() {
				c.After(a.dismissCollectionRunner)
			}
			return
		}
		if ui.Button(c, text.RunClose).Outline().Disabled(state.presets.writing).Clicked() {
			c.After(a.dismissCollectionRunner)
		}
		if state.report.Version == 0 && ui.Button(c, text.RunStart).Icon(icons.Play).Disabled(state.presets.writing).Clicked() {
			c.After(a.startCollectionRun)
		}
	})
	if dismissed {
		c.After(a.dismissCollectionRunner)
	}
}

func (a *App) collectionRunnerSetup(c ui.C) {
	state, text, theme := &a.collections.runner, a.S.Collections, c.Theme()
	labels := i18n.RunnerFor(a.S.Lang)
	ui.Scroll(c).Fill().Scrollbars().Children(func() {
		ui.Column(c).FillWidth().Gap(Space3).Children(func() {
			if state.collectionHeading != "" {
				ui.Wrap(c, state.collectionHeading).Bold()
			}
			ui.Wrap(c, text.RunSetup).Muted()
			ui.Row(c).FillWidth().Gap(Space3).Children(func() {
				ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(Space2).Children(func() {
					ui.Text(c, labels.Scope).Bold()
					control := ui.Select(c, &state.folderID, state.folderIDs, func(id string) string {
						if id == "" {
							return labels.WholeCollection
						}
						return state.folderNames[id]
					}).FillWidth().Label(labels.Scope).Placeholder(labels.UnavailableChoice)
					if _, picked := control.Picked(); picked {
						state.folderUnavailable = false
					}
				})
				ui.Column(c).Basis(0).Grow(1).MinWidth(0).Gap(Space2).Children(func() {
					ui.Text(c, text.RunEnvironment).Bold()
					control := ui.Select(c, &state.environmentID, state.environmentIDs, func(id string) string {
						if id == "" {
							return text.RunNoEnvironment
						}
						return state.environmentNames[id]
					}).FillWidth().Label(text.RunEnvironment).Placeholder(labels.UnavailableChoice)
					if _, picked := control.Picked(); picked {
						state.environmentUnavailable = false
					}
				})
			})
			if state.folderUnavailable {
				ui.Wrap(c, labels.MissingFolder).Color(theme.Danger)
			}
			if state.environmentUnavailable {
				ui.Wrap(c, text.RunMissingEnvironment).Color(theme.Danger)
			}
			ui.Row(c).FillWidth().Gap(Space3).Items(ui.Center).Children(func() {
				if ui.Button(c, labels.BrowseData).Outline().Clicked() {
					c.After(a.chooseCollectionRunData)
				}
				if state.dataFile != "" {
					ui.Text(c, labels.DataSelected).Muted().Grow(1)
					if ui.Button(c, labels.ClearData).Outline().Clicked() {
						state.dataFile, state.problem = "", collectionRunReady
					}
				}
			})
			ui.Row(c).FillWidth().Gap(Space4).Items(ui.Center).Children(func() {
				ui.Checkbox(c, &state.stopOnFailure, text.RunStopOnFailure)
				ui.Checkbox(c, &state.useCookieSession, text.RunUseCookieSession)
			})
			switch state.problem {
			case collectionRunMissingCollection:
				ui.Wrap(c, text.RunMissingCollection).Color(theme.Danger)
			case collectionRunTargetChanged:
				ui.Wrap(c, labels.TargetChanged).Color(a.palette.Warning)
			case collectionRunInvalidData:
				ui.Wrap(c, labels.InvalidData).Color(theme.Danger)
			case collectionRunIterationLimit:
				ui.Wrap(c, labels.IterationLimit).Color(theme.Danger)
			}
			a.collectionRunnerPresetControls(c, labels)
		})
	})
}

func (a *App) collectionRunnerSummary(c ui.C, state *collectionRunnerState) {
	text, theme, report := a.S.Collections, c.Theme(), state.report
	labels := i18n.RunnerFor(a.S.Lang)
	label, color := a.collectionRunOutcome(c, report.Outcome)
	ui.Text(c, label).Bold().Color(color)
	counts := report.Counts
	ui.Wrap(c, fmt.Sprintf(text.RunCounts, counts.Total, counts.Passed, counts.Failed, counts.Cancelled, counts.Skipped))
	ui.Wrap(c, fmt.Sprintf(text.RunAssertions, report.Assertions.Passed, report.Assertions.Failed)).Muted()
	if report.Iterations > 0 {
		ui.Text(c, fmt.Sprintf(labels.Iterations, report.Iterations)).Muted()
	}
	if len(report.Requests) == 0 {
		empty := text.RunEmpty
		if state.scopedEmpty {
			empty = labels.EmptyScope
		}
		ui.Wrap(c, empty).Muted()
		return
	}
	rowsLabel := text.RunRows
	if report.Iterations > 0 {
		rowsLabel = labels.IterationRows
	}
	state.list.Name = func(index int) string {
		entry := report.Requests[index]
		label, status := a.collectionRunOutcomeLabel(entry.Outcome), a.collectionRunStatus(entry)
		name := fmt.Sprintf("%d · %s · %s · %s", entry.Ordinal, status, label, fmt.Sprintf(text.RunAssertions, entry.Assertions.Passed, entry.Assertions.Failed))
		if entry.Iteration > 0 {
			name = fmt.Sprintf("%s %d · %s", labels.Iteration, entry.Iteration, name)
		}
		return name
	}
	ui.Row(c).FillWidth().Shrink(0).Height(theme.Metrics.ControlHeight).PadX(Space2).Gap(Space3).Items(ui.Center).
		Background(a.palette.TableHeader).Children(func() {
		if report.Iterations > 0 {
			ui.Text(c, labels.Iteration).Width(3 * Space6).Muted()
		}
		ui.Text(c, text.RunOrdinal).Width(4 * Space6).Muted()
		ui.Text(c, text.RunStatus).Width(5 * Space6).Muted()
		ui.Text(c, text.RunOutcome).Width(5 * Space6).Muted()
		ui.Text(c, text.RunAssertionColumn).Muted()
	})
	ui.List(c, &state.list, len(report.Requests), func(row ui.ListRow) {
		entry := report.Requests[row.Index]
		label, outcomeColor := a.collectionRunOutcome(c, entry.Outcome)
		status := a.collectionRunStatus(entry)
		statusColor := theme.MutedForeground
		if entry.StatusCode != 0 {
			statusColor = a.palette.StatusColor(theme, entry.StatusCode)
		}
		row.Element.Height(theme.Metrics.ControlHeight).PadX(Space2).Gap(Space3).Items(ui.Center).
			Focus(ui.Look{}.Ring(1, theme.Ring)).Children(func() {
			if report.Iterations > 0 {
				ui.Text(row.C, strconv.Itoa(entry.Iteration)).Width(3 * Space6)
			}
			ui.Text(row.C, strconv.Itoa(entry.Ordinal)).Width(4 * Space6)
			ui.Text(row.C, status).Width(5 * Space6).Color(statusColor)
			ui.Text(row.C, label).Width(5 * Space6).Color(outcomeColor)
			ui.Text(row.C, fmt.Sprintf(text.RunAssertions, entry.Assertions.Passed, entry.Assertions.Failed)).Ellipsis().Grow(1).MinWidth(0)
		})
	}).FillWidth().Grow(1).MinHeight(0).Label(rowsLabel).Scrollbars()
}

func (a *App) collectionRunStatus(entry runner.RequestReport) string {
	if entry.StatusCode != 0 {
		return strconv.Itoa(entry.StatusCode)
	}
	return a.S.Collections.RunNoStatus
}

func (a *App) collectionRunOutcomeLabel(outcome runner.Outcome) string {
	text := a.S.Collections
	switch outcome {
	case runner.Passed:
		return text.RunPassed
	case runner.Cancelled:
		return text.RunCancelled
	case runner.Skipped:
		return text.RunSkipped
	default:
		return text.RunFailed
	}
}

func (a *App) collectionRunOutcome(c ui.C, outcome runner.Outcome) (string, ui.Color) {
	theme := c.Theme()
	color := theme.MutedForeground
	switch outcome {
	case runner.Passed:
		color = theme.Success
	case runner.Failed:
		color = theme.Danger
	case runner.Cancelled:
		color = a.palette.Warning
	}
	return a.collectionRunOutcomeLabel(outcome), color
}
