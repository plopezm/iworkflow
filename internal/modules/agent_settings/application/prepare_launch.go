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
	return u.ExecuteWithOverrides(ctx, repoDir, models.LaunchOverrides{})
}

// ExecuteWithOverrides prepares the launch of a task whose worktree is repoDir with task-specific overrides.
func (u *PrepareLaunch) ExecuteWithOverrides(ctx context.Context, repoDir string, overrides models.LaunchOverrides) (models.LaunchPreferences, error) {
	settings, err := u.repository.Get(ctx)
	if err != nil {
		return models.LaunchPreferences{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}
	runtime := settings.Runtime
	if overrides.Runtime != nil {
		runtime = overrides.Runtime
	}
	effectiveRuntime := models.RuntimeOrDefault(runtime)

	profile := settings.Profile
	if overrides.Profile != nil {
		profile = overrides.Profile
	}
	if err := u.pin.Apply(ctx, repoDir, profile); err != nil {
		return models.LaunchPreferences{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}

	model := settings.Model
	if overrides.Model != nil {
		model = overrides.Model
	} else if overrides.Runtime != nil && settings.Runtime != nil && *overrides.Runtime != *settings.Runtime {
		model = nil
	}

	thinking := settings.Thinking
	if overrides.Thinking != nil {
		thinking = overrides.Thinking
	}

	return models.LaunchPreferences{
		Runtime:         runtime,
		Model:           model,
		Thinking:        effectiveThinking(thinking, effectiveRuntime),
		MemoryLimitMB:   settings.MemoryLimitMB,
		CPULimitPercent: settings.CPULimitPercent,
		LimitsDisabled:  settings.LimitsDisabled,
	}, nil
}
