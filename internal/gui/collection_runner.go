package gui

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/runbrick/reva/internal/gui/icons"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service"
	"github.com/vivaui/viva/ui"
)

type collectionRunnerState struct {
	open, stopOnFailure, cancelling bool
	collectionID, environmentID     string
	environmentIDs                  []string
	environmentNames                map[string]string
	problem                         collectionRunProblem
	job                             *collectionRunJob
	effect                          *ui.EffectState[runner.Report, *collectionRunJob]
	report                          runner.Report
	list                            ui.ListState
}

type collectionRunProblem uint8

const (
	collectionRunReady collectionRunProblem = iota
	collectionRunMissingCollection
	collectionRunMissingEnvironment
)

type collectionRunJob struct {
	input   runner.Snapshot
	options runner.Options

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

func (job *collectionRunJob) run(parent context.Context, sender service.RequestService) runner.Report {
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
	return input.RunReport(ctx, sender, job.options)
}

func (a *App) openCollectionRunner(collectionID string) {
	state := &a.collections.runner
	if state.open || state.job != nil {
		return
	}
	state.open, state.collectionID = true, collectionID
	a.collectionRunnerEnvironments()
}

func (a *App) collectionRunnerEnvironments() {
	state := &a.collections.runner
	state.environmentIDs = []string{""}
	state.environmentNames = make(map[string]string)
	collection, ok := a.Svc.Collections.Get(state.collectionID)
	if !ok {
		return
	}
	for _, environment := range a.visibleEnvironments(collection) {
		if environment.ID != "" {
			state.environmentIDs = append(state.environmentIDs, environment.ID)
			state.environmentNames[environment.ID] = environment.Name
		}
	}
}

func (a *App) startCollectionRun() {
	state := &a.collections.runner
	if !state.open || state.job != nil || state.report.Version != 0 {
		return
	}
	collection, ok := a.Svc.Collections.Get(state.collectionID)
	if !ok {
		state.problem = collectionRunMissingCollection
		return
	}
	var values []model.KV
	if state.environmentID != "" {
		environment, ok := a.environmentFor(collection, state.environmentID)
		if !ok {
			state.problem = collectionRunMissingEnvironment
			state.environmentID = ""
			a.collectionRunnerEnvironments()
			return
		}
		values = environment.Variables
	}
	state.job = &collectionRunJob{
		input:   runner.NewSnapshot(collection, a.Svc.Settings.Get(), values),
		options: runner.Options{StopOnFailure: state.stopOnFailure},
	}
	sender := a.Svc.Requests
	state.effect = ui.NewEffectKeyed(func(ctx context.Context, job *collectionRunJob) (runner.Report, error) {
		return job.run(ctx, sender), nil
	})
	state.collectionID, state.environmentID = "", ""
	state.environmentIDs, state.environmentNames = nil, nil
	state.problem = collectionRunReady
}

func (a *App) declareCollectionRunner(c ui.C) {
	state := &a.collections.runner
	if state.job == nil {
		return
	}
	ui.Effect(c, state.effect, state.job)
	if report, _, ok := state.effect.Take(); ok {
		state.job, state.effect, state.cancelling, state.report = nil, nil, false, report
	}
}

func (a *App) dismissCollectionRunner() {
	state := &a.collections.runner
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
	spec := ui.ModalSpec{Width: 27 * Space6, Height: 17 * Space6, NoClose: state.job != nil}
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
				ui.Wrap(c, text.RunSetup).Muted()
				ui.Text(c, text.RunEnvironment).Bold()
				ui.Select(c, &state.environmentID, state.environmentIDs, func(id string) string {
					if id == "" {
						return text.RunNoEnvironment
					}
					return state.environmentNames[id]
				}).FillWidth().Label(text.RunEnvironment)
				ui.Checkbox(c, &state.stopOnFailure, text.RunStopOnFailure)
				switch state.problem {
				case collectionRunMissingCollection:
					ui.Wrap(c, text.RunMissingCollection).Color(theme.Danger)
				case collectionRunMissingEnvironment:
					ui.Wrap(c, text.RunMissingEnvironment).Color(theme.Danger)
				}
			}
		})
	}, func() {
		if state.job != nil {
			if ui.Button(c, text.RunCancel).Outline().Icon(icons.X).Disabled(state.cancelling).Clicked() {
				c.After(a.dismissCollectionRunner)
			}
			return
		}
		if ui.Button(c, text.RunClose).Outline().Clicked() {
			c.After(a.dismissCollectionRunner)
		}
		if state.report.Version == 0 && ui.Button(c, text.RunStart).Icon(icons.Play).Clicked() {
			c.After(a.startCollectionRun)
		}
	})
	if dismissed {
		c.After(a.dismissCollectionRunner)
	}
}

func (a *App) collectionRunnerSummary(c ui.C, state *collectionRunnerState) {
	text, theme, report := a.S.Collections, c.Theme(), state.report
	label, color := a.collectionRunOutcome(c, report.Outcome)
	ui.Text(c, label).Bold().Color(color)
	counts := report.Counts
	ui.Wrap(c, fmt.Sprintf(text.RunCounts, counts.Total, counts.Passed, counts.Failed, counts.Cancelled, counts.Skipped))
	ui.Wrap(c, fmt.Sprintf(text.RunAssertions, report.Assertions.Passed, report.Assertions.Failed)).Muted()
	if len(report.Requests) == 0 {
		ui.Wrap(c, text.RunEmpty).Muted()
		return
	}
	state.list.Name = func(index int) string {
		entry := report.Requests[index]
		label, status := a.collectionRunOutcomeLabel(entry.Outcome), a.collectionRunStatus(entry)
		return fmt.Sprintf("%d · %s · %s · %s", entry.Ordinal, status, label, fmt.Sprintf(text.RunAssertions, entry.Assertions.Passed, entry.Assertions.Failed))
	}
	ui.Row(c).FillWidth().Shrink(0).Height(theme.Metrics.ControlHeight).PadX(Space2).Gap(Space3).Items(ui.Center).
		Background(a.palette.TableHeader).Children(func() {
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
			ui.Text(row.C, strconv.Itoa(entry.Ordinal)).Width(4 * Space6)
			ui.Text(row.C, status).Width(5 * Space6).Color(statusColor)
			ui.Text(row.C, label).Width(5 * Space6).Color(outcomeColor)
			ui.Text(row.C, fmt.Sprintf(text.RunAssertions, entry.Assertions.Passed, entry.Assertions.Failed)).Ellipsis().Grow(1).MinWidth(0)
		})
	}).FillWidth().Grow(1).MinHeight(0).Label(text.RunRows).Scrollbars()
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
