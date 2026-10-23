package application

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

// PrepareLaunch reads the global settings, applies the profile pin in repoDir (the worktree of
// the task about to start) and returns the model and reasoning level to use, when set.
type PrepareLaunch struct {
	repository outbound.AgentSettingsRepositoryPort
	pin        outbound.ProfilePinPort
}

// NewPrepareLaunch builds the use case.
func NewPrepareLaunch(repository outbound.AgentSettingsRepositoryPort, pin outbound.ProfilePinPort) *PrepareLaunch {
	return &PrepareLaunch{repository: repository, pin: pin}
}

// Execute prepares the launch of a task whose worktree is repoDir.
func (u *PrepareLaunch) Execute(ctx context.Context, repoDir string) (models.LaunchPreferences, error) {
	settings, err := u.repository.Get(ctx)
	if err != nil {
		return models.LaunchPreferences{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}
	runtime := models.RuntimeOrDefault(settings.Runtime)
	// pi loads the gentle-pi extension, which reads the pin the same way gentle-shell does.
	if err := u.pin.Apply(ctx, repoDir, settings.Profile); err != nil {
		return models.LaunchPreferences{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}
	return models.LaunchPreferences{
		Runtime:         settings.Runtime,
		Model:           settings.Model,
		Thinking:        effectiveThinking(settings.Thinking, runtime),
		MemoryLimitMB:   settings.MemoryLimitMB,
		CPULimitPercent: settings.CPULimitPercent,
		LimitsDisabled:  settings.LimitsDisabled,
	}, nil
}
