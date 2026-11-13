package gui

import "github.com/vivaui/viva/ui"

// onboardingEntry keeps the optional guide in the settings heading.
func (a *App) onboardingEntry(c ui.C) {
	s := &a.S.Settings
	label := s.OpenGuide
	if a.settings.current.OnboardingCompleted {
		label = s.ReplayGuide
	}
	ui.Text(c, s.GettingStarted).Muted().FontSize(c.Theme().Metrics.FontSizeS)
	if ui.Button(c, label).XSmall().Outline().Tooltip(s.GettingStartedDescription).Clicked() {
		c.After(func() {
			a.settings.guideOpen = true
			a.settings.guideError = ""
		})
	}
}

func (a *App) onboardingDialog(c ui.C) {
	state := &a.settings
	if !state.guideOpen {
		return
	}
	s := &a.S.Settings
	ui.ModalWith(c, &state.guideOpen, s.GuideTitle, ui.ModalSpec{
		Description: s.GuideIntro,
		Width:       Space6 * 27,
	}, func() {
		ui.Column(c).FillWidth().Gap(Space4).Children(func() {
			onboardingStep(c, s.GuideRequestTitle, s.GuideRequestDescription)
			onboardingStep(c, s.GuideVariablesTitle, s.GuideVariablesDescription)
			onboardingStep(c, s.GuideDataTitle, s.GuideDataDescription)
			if state.guideError != "" {
				ui.Wrap(c, state.guideError).FillWidth().Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, s.GuideSkip).Outline().Clicked() {
			c.After(func() { state.guideOpen = false })
		}
		if ui.Button(c, s.GuideDone).Clicked() {
			c.After(a.completeOnboarding)
		}
	})
}

func onboardingStep(c ui.C, title, description string) {
	ui.Column(c).FillWidth().Gap(Space1).Children(func() {
		ui.Text(c, title).Bold().Color(c.Theme().Primary)
		ui.Wrap(c, description).FillWidth().Muted().FontSize(c.Theme().Metrics.FontSizeS)
	})
}

// completeOnboarding runs after declaration, preserving preferences changed
// since the guide opened. A failed save leaves the guide available for retry.
func (a *App) completeOnboarding() {
	settings := a.Svc.Settings.Get()
	settings.OnboardingCompleted = true
	if err := a.Svc.Settings.Save(settings); err != nil {
		a.settings.guideError = a.S.Settings.GuideSaveFailed
		return
	}
	a.settings.guideOpen = false
	a.settings.guideError = ""
}
