package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
)

// fakeWorkspaces answers the workspace path of the test, or fails when err is set.
type fakeWorkspaces struct {
	path string
	err  error
}

func (f *fakeWorkspaces) Path(context.Context, string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.path, nil
}

// capturingRunner records the context of every run and answers with a fixed outcome or error.
type capturingRunner struct {
	runs    []models.RunContext
	outcome models.RunOutcome
	err     error
}

func (c *capturingRunner) Run(_ context.Context, _ models.Hook, run models.RunContext) (models.RunOutcome, error) {
	c.runs = append(c.runs, run)
	return c.outcome, c.err
}

func savedHook(t *testing.T, id string, kind models.Kind) models.Hook {
	t.Helper()
	return libraryHook(t, id, kind)
}

// The test runs one hook through the same runner a real run uses, but with a synthetic task: the workspace path is
// the working directory, and the run is not recorded anywhere (the use case has no run repository at all).
func TestTestHookRunsOneHookWithASyntheticTaskInTheWorkspace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		kind       models.Kind
		phase      models.Phase
		event      models.Event
		wantResult models.TaskResult
	}{
		{name: "script before start", kind: models.KindScript, phase: models.PhasePre, event: models.EventTaskStart, wantResult: models.TaskResultNone},
		{name: "script after finish", kind: models.KindScript, phase: models.PhasePost, event: models.EventTaskFinish, wantResult: models.TaskResultOK},
		{name: "agent before start", kind: models.KindAgent, phase: models.PhasePre, event: models.EventTaskStart, wantResult: models.TaskResultNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hook := savedHook(t, "h1", tt.kind)
			hooks := &fakeHooks{hooks: []models.Hook{hook}}
			scripts := &capturingRunner{outcome: models.RunOutcome{Status: models.RunOK, Output: "hello\n"}}
			agents := &capturingRunner{outcome: models.RunOutcome{Status: models.RunFailed, ExitCode: -1}}
			useCase := application.NewTestHook(hooks, &fakeWorkspaces{path: "/work/ws"}, scripts, agents)

			outcome, err := useCase.Execute(context.Background(), application.TestHookInput{WorkspaceID: "w1", HookID: "h1", Event: tt.event, Phase: tt.phase})

			require.NoError(t, err)
			runner := scripts
			if tt.kind == models.KindAgent {
				runner = agents
			}
			require.Len(t, runner.runs, 1)
			assert.Equal(t, models.RunContext{
				TaskID: "test", TaskTitle: "test", Branch: "test",
				WorkspacePath: "/work/ws", WorktreePath: "/work/ws",
				Phase: tt.phase, Event: tt.event, Result: tt.wantResult,
			}, runner.runs[0])
			assert.Equal(t, runner.outcome, outcome)
		})
	}
}

// A hook that fails is still an outcome: the test shows it, it is not an error of the use case.
func TestTestHookReturnsAFailedRunAsAnOutcome(t *testing.T) {
	t.Parallel()
	hooks := &fakeHooks{hooks: []models.Hook{savedHook(t, "h1", models.KindScript)}}
	scripts := &capturingRunner{outcome: models.RunOutcome{Status: models.RunTimedOut, ExitCode: -1, Output: "slow"}}
	useCase := application.NewTestHook(hooks, &fakeWorkspaces{path: "/work/ws"}, scripts, &capturingRunner{})

	outcome, err := useCase.Execute(context.Background(), pre("w1", "h1"))

	require.NoError(t, err)
	assert.Equal(t, models.RunTimedOut, outcome.Status)
	assert.Equal(t, "slow", outcome.Output)
}

func TestTestHookRefusesWhatItCannotRun(t *testing.T) {
	t.Parallel()
	hook := savedHook(t, "h1", models.KindScript)
	lookupErr := errors.New("workspace gone")
	tests := []struct {
		name       string
		hooks      []models.Hook
		workspaces *fakeWorkspaces
		input      application.TestHookInput
		wantErr    error
		wantRuns   int
	}{
		{
			name: "unknown hook", hooks: []models.Hook{hook}, workspaces: &fakeWorkspaces{path: "/work/ws"},
			input: pre("w1", "missing"), wantErr: hookerrors.ErrHookNotFound,
		},
		{
			name: "no phase", hooks: []models.Hook{hook}, workspaces: &fakeWorkspaces{path: "/work/ws"},
			input: application.TestHookInput{WorkspaceID: "w1", HookID: "h1"}, wantErr: hookerrors.ErrInvalidHook,
		},
		{
			name: "workspace lookup fails", hooks: []models.Hook{hook}, workspaces: &fakeWorkspaces{err: lookupErr},
			input: pre("w1", "h1"), wantErr: lookupErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			scripts := &capturingRunner{}
			useCase := application.NewTestHook(&fakeHooks{hooks: tt.hooks}, tt.workspaces, scripts, &capturingRunner{})

			_, err := useCase.Execute(context.Background(), tt.input)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, scripts.runs, "a refused test must not run anything")
		})
	}
}

// A run that could not even be attempted is an error, so the screen can say why nothing ran.
func TestTestHookSurfacesARunThatCouldNotStart(t *testing.T) {
	t.Parallel()
	hook := savedHook(t, "h1", models.KindScript)
	startErr := fmt.Errorf("%w: missing directory", hookerrors.ErrRunNotStarted)
	scripts := &capturingRunner{err: startErr}
	useCase := application.NewTestHook(&fakeHooks{hooks: []models.Hook{hook}}, &fakeWorkspaces{path: "/gone"}, scripts, &capturingRunner{})

	_, err := useCase.Execute(context.Background(), pre("w1", "h1"))

	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
}

// pre is the input of a test as a before-start hook of the workspace.
func pre(workspaceID string, hookID string) application.TestHookInput {
	return application.TestHookInput{WorkspaceID: workspaceID, HookID: hookID, Event: models.EventTaskStart, Phase: models.PhasePre}
}
