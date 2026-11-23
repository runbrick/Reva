package gui

import (
	"context"
	"errors"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/runner"
	"github.com/vivaui/viva/ui"
)

type collectionRunnerPresets struct {
	choices []runner.Preset
	value   runner.Preset
	name    string
	err     error
	writing bool
	effect  *ui.EffectState[collectionRunnerPresetResult, struct{}]
}

type collectionRunnerPresetResult struct {
	presets    []runner.Preset
	selectedID string
}

func (a *App) loadCollectionRunnerPresets() {
	state := &a.collections.runner
	collectionID := state.collectionID
	state.presets.err = nil
	state.presets.effect = ui.NewEffect[collectionRunnerPresetResult, struct{}](func(ctx context.Context) (collectionRunnerPresetResult, error) {
		if err := ctx.Err(); err != nil {
			return collectionRunnerPresetResult{}, err
		}
		presets, err := runner.ListPresets(collectionID)
		return collectionRunnerPresetResult{presets: presets}, err
	})
}

func (a *App) writeCollectionRunnerPreset(remove bool) {
	state := &a.collections.runner
	panel := &state.presets
	if !state.open || state.job != nil || state.report.Version != 0 || panel.effect != nil || panel.choices == nil {
		return
	}
	if remove {
		if panel.value.ID == "" {
			return
		}
	} else if _, _, ready := a.validateCollectionRunTargets(); !ready {
		return
	}
	collectionID := state.collectionID
	preset := runner.Preset{
		ID: panel.value.ID, Name: panel.name, FolderID: state.folderID, EnvironmentID: state.environmentID,
		StopOnFailure: state.stopOnFailure, UseCookieSession: state.useCookieSession,
	}
	panel.err, panel.writing = nil, true
	panel.effect = ui.NewEffect[collectionRunnerPresetResult, struct{}](func(ctx context.Context) (collectionRunnerPresetResult, error) {
		if err := ctx.Err(); err != nil {
			return collectionRunnerPresetResult{}, err
		}
		if remove {
			presets, err := runner.DeletePreset(collectionID, preset.ID)
			return collectionRunnerPresetResult{presets: presets}, err
		}
		presets, id, err := runner.SavePreset(collectionID, preset)
		return collectionRunnerPresetResult{presets: presets, selectedID: id}, err
	})
}

func (a *App) declareCollectionRunnerPresets(c ui.C) {
	panel := &a.collections.runner.presets
	if panel.effect == nil {
		return
	}
	ui.Effect(c, panel.effect, struct{}{})
	result, err, ready := panel.effect.Take()
	if !ready {
		return
	}
	wasWriting := panel.writing
	panel.effect, panel.writing, panel.err = nil, false, err
	if err != nil {
		if !wasWriting {
			panel.choices = nil
		}
		return
	}
	panel.choices = append([]runner.Preset{{}}, result.presets...)
	panel.value, panel.name = runner.Preset{}, ""
	for _, preset := range result.presets {
		if preset.ID == result.selectedID {
			panel.value, panel.name = preset, preset.Name
			break
		}
	}
}

func (a *App) applyCollectionRunnerPreset() {
	state := &a.collections.runner
	panel := &state.presets
	preset := panel.value
	panel.name, panel.err = preset.Name, nil
	if preset.ID == "" {
		return
	}
	state.folderID, state.environmentID = preset.FolderID, preset.EnvironmentID
	state.stopOnFailure, state.useCookieSession = preset.StopOnFailure, preset.UseCookieSession
	state.folderUnavailable, state.environmentUnavailable = false, false
	state.dataFile, state.problem = "", collectionRunReady
	a.collectionRunnerChoices()
}

func (a *App) collectionRunnerPresetControls(c ui.C, labels i18n.Runner) {
	panel := &a.collections.runner.presets
	switch {
	case panel.writing:
		ui.Text(c, labels.WritingPresets).Muted()
		return
	case panel.effect != nil:
		ui.Text(c, labels.LoadingPresets).Muted()
		return
	case panel.choices == nil:
		a.collectionRunnerPresetRetry(c, labels, labels.PresetLoadError)
		return
	}
	control := ui.Select(c, &panel.value, panel.choices, func(preset runner.Preset) string {
		if preset.ID == "" {
			return labels.CustomPreset
		}
		return preset.Name
	}).FillWidth().Label(labels.Preset)
	if _, picked := control.Picked(); picked {
		c.After(a.applyCollectionRunnerPreset)
	}
	editing := panel.value.ID != ""
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		ui.TextField(c, &panel.name, labels.PresetName, labels.PresetName).Grow(1).MinWidth(0)
		save := labels.SavePreset
		if editing {
			save = labels.UpdatePreset
		}
		if ui.Button(c, save).Outline().Clicked() {
			c.After(func() { a.writeCollectionRunnerPreset(false) })
		}
		if editing && ui.Button(c, labels.DeletePreset).Outline().Clicked() {
			c.After(func() { a.writeCollectionRunnerPreset(true) })
		}
	})
	var message string
	switch {
	case panel.err == nil:
		return
	case errors.Is(panel.err, runner.ErrEmptyPresetName):
		message = labels.EmptyPresetName
	case errors.Is(panel.err, runner.ErrDuplicatePresetName):
		message = labels.DuplicatePresetName
	case errors.Is(panel.err, runner.ErrPresetNotFound):
		a.collectionRunnerPresetRetry(c, labels, labels.PresetUnavailable)
		return
	default:
		message = labels.PresetWriteError
	}
	ui.Wrap(c, message).Color(c.Theme().Danger)
}

func (a *App) collectionRunnerPresetRetry(c ui.C, labels i18n.Runner, message string) {
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		ui.Wrap(c, message).Grow(1).MinWidth(0).Color(c.Theme().Danger)
		if ui.Button(c, labels.RetryPresets).Outline().Clicked() {
			c.After(a.loadCollectionRunnerPresets)
		}
	})
}
