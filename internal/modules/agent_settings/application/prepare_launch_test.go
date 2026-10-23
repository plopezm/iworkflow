package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/application"
	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

const testWorktree = "/worktree"

func profileName(t *testing.T, raw string) *models.ProfileName {
	t.Helper()
	name, err := models.ParseProfileName(raw)
	require.NoError(t, err)
	return &name
}

func runtimePointer(runtime models.AgentRuntime) *models.AgentRuntime {
	return &runtime
}

func thinkingPointer(level models.ThinkingLevel) *models.ThinkingLevel {
	return &level
}

func TestPrepareLaunchAppliesPinAndReturnsModelAndThinking(t *testing.T) {
	model := models.ModelRef{Provider: "anthropic", Model: "claude-5"}
	repository := &fakeRepository{settings: models.AgentSettings{
		Model:    &model,
		Profile:  profileName(t, "Claude"),
		Thinking: thinkingPointer(models.ThinkingHigh),
	}}
	pin := &fakePin{}
	useCase := application.NewPrepareLaunch(repository, pin)

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.NoError(t, err)
	require.Equal(t, []pinCall{{repoDir: testWorktree, profile: profileName(t, "Claude")}}, pin.calls)
	require.Equal(t, models.LaunchPreferences{
		Model:    &model,
		Thinking: thinkingPointer(models.ThinkingHigh),
	}, result)
}

func TestPrepareLaunchReturnsLimitsAndTheDisabledSwitch(t *testing.T) {
	memory := uint32(2048)
	cpu := uint32(100)
	repository := &fakeRepository{settings: models.AgentSettings{
		MemoryLimitMB:   &memory,
		CPULimitPercent: &cpu,
		LimitsDisabled:  true,
	}}
	useCase := application.NewPrepareLaunch(repository, &fakePin{})

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.NoError(t, err)
	require.Equal(t, &memory, result.MemoryLimitMB)
	require.Equal(t, &cpu, result.CPULimitPercent)
	require.True(t, result.LimitsDisabled)
}

func TestPrepareLaunchRemovesPinAndReturnsDefaultsForDefaultSettings(t *testing.T) {
	pin := &fakePin{}
	useCase := application.NewPrepareLaunch(&fakeRepository{}, pin)

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.NoError(t, err)
	require.Equal(t, []pinCall{{repoDir: testWorktree, profile: nil}}, pin.calls)
	require.Equal(t, models.LaunchPreferences{}, result)
}

func TestPrepareLaunchKeepsRuntimeAndProfilePinForPi(t *testing.T) {
	repository := &fakeRepository{settings: models.AgentSettings{
		Runtime: runtimePointer(models.RuntimePi),
		Profile: profileName(t, "Claude"),
	}}
	pin := &fakePin{}
	useCase := application.NewPrepareLaunch(repository, pin)

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.NoError(t, err)
	require.Equal(t, []pinCall{{repoDir: testWorktree, profile: profileName(t, "Claude")}}, pin.calls)
	require.Equal(t, runtimePointer(models.RuntimePi), result.Runtime)
}

func TestPrepareLaunchDropsSavedThinkingIncompatibleWithTheRuntime(t *testing.T) {
	repository := &fakeRepository{settings: models.AgentSettings{
		Runtime:  runtimePointer(models.RuntimeClaude),
		Thinking: thinkingPointer(models.ThinkingOff),
	}}
	useCase := application.NewPrepareLaunch(repository, &fakePin{})

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.NoError(t, err)
	require.Equal(t, runtimePointer(models.RuntimeClaude), result.Runtime)
	require.Nil(t, result.Thinking)
}

func TestPrepareLaunchKeepsThinkingCompatibleWithTheRuntime(t *testing.T) {
	repository := &fakeRepository{settings: models.AgentSettings{
		Runtime:  runtimePointer(models.RuntimeClaude),
		Thinking: thinkingPointer(models.ThinkingMax),
	}}
	useCase := application.NewPrepareLaunch(repository, &fakePin{})

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.NoError(t, err)
	require.Equal(t, thinkingPointer(models.ThinkingMax), result.Thinking)
}

func TestPrepareLaunchKeepsOffThinkingForPi(t *testing.T) {
	repository := &fakeRepository{settings: models.AgentSettings{
		Runtime:  runtimePointer(models.RuntimePi),
		Thinking: thinkingPointer(models.ThinkingOff),
	}}
	useCase := application.NewPrepareLaunch(repository, &fakePin{})

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.NoError(t, err)
	require.Equal(t, thinkingPointer(models.ThinkingOff), result.Thinking)
}

func TestPrepareLaunchDropsCursorThinkingBecauseCursorHasNoLevels(t *testing.T) {
	repository := &fakeRepository{settings: models.AgentSettings{
		Runtime:  runtimePointer(models.RuntimeCursor),
		Thinking: thinkingPointer(models.ThinkingLow),
	}}
	useCase := application.NewPrepareLaunch(repository, &fakePin{})

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.NoError(t, err)
	require.Nil(t, result.Thinking)
}

func TestPrepareLaunchStopsWhenThePinFails(t *testing.T) {
	pin := &fakePin{err: agenterrors.IO(errors.New("read-only"))}
	useCase := application.NewPrepareLaunch(&fakeRepository{}, pin)

	result, err := useCase.Execute(context.Background(), testWorktree)

	require.ErrorIs(t, err, agenterrors.ErrIO)
	require.Equal(t, models.LaunchPreferences{}, result)
}
