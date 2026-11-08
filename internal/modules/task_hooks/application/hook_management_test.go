package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
)

// workspaceHook builds a valid script hook of a workspace: pre task_start, warn on failure.
func workspaceHook(t *testing.T, id string, workspaceID string, command string) models.Hook {
	t.Helper()
	h, err := models.NewHook(models.HookParams{
		ID: id, WorkspaceID: workspaceID, Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureWarn, Timeout: time.Minute,
		Script: models.ScriptSpec{Command: command},
	})
	require.NoError(t, err)
	return h
}

func validSaveInput(workspaceID string) application.SaveHookInput {
	return application.SaveHookInput{
		WorkspaceID: workspaceID, Phase: models.PhasePost, Event: models.EventTaskFinish, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureBlock, Timeout: time.Minute,
		Script: models.ScriptSpec{Command: "make", Args: []string{"lint"}},
	}
}

func ids(hooks []models.Hook) []string {
	out := make([]string, 0, len(hooks))
	for _, h := range hooks {
		out = append(out, h.ID())
	}
	return out
}

func TestListHooksReturnsOnlyTheWorkspaceHooksInPositionOrder(t *testing.T) {
	t.Parallel()
	repo := &fakeHooks{hooks: []models.Hook{
		workspaceHook(t, "a", "w1", "x"), workspaceHook(t, "b", "w2", "x"), workspaceHook(t, "c", "w1", "x"),
	}}

	got, err := application.NewListHooks(repo).Execute(context.Background(), "w1")

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "c"}, ids(got))
}

func TestSaveHookCreatesAHookWithAGeneratedID(t *testing.T) {
	t.Parallel()
	repo := &fakeHooks{}
	uc := application.NewSaveHook(repo, &fakeIDs{})

	saved, err := uc.Execute(context.Background(), validSaveInput("w1"))

	require.NoError(t, err)
	assert.Equal(t, "run1", saved.ID())
	assert.Equal(t, "w1", saved.WorkspaceID())
	assert.Equal(t, models.FailureBlock, saved.OnFailure())
	require.Len(t, repo.hooks, 1)
	assert.Equal(t, "make", repo.hooks[0].Script().Command)
}

func TestSaveHookRejectsInvalidInputWithoutSaving(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(in *application.SaveHookInput)
	}{
		{name: "script without command", mutate: func(in *application.SaveHookInput) { in.Script.Command = " " }},
		{name: "missing timeout", mutate: func(in *application.SaveHookInput) { in.Timeout = 0 }},
		{name: "unknown event", mutate: func(in *application.SaveHookInput) { in.Event = "task_discard" }},
		{name: "missing workspace", mutate: func(in *application.SaveHookInput) { in.WorkspaceID = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeHooks{}
			in := validSaveInput("w1")
			tt.mutate(&in)

			_, err := application.NewSaveHook(repo, &fakeIDs{}).Execute(context.Background(), in)

			require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
			assert.Empty(t, repo.hooks)
		})
	}
}

func TestSaveHookUpdatesAnExistingHookOfItsWorkspace(t *testing.T) {
	t.Parallel()
	repo := &fakeHooks{hooks: []models.Hook{workspaceHook(t, "a", "w1", "old"), workspaceHook(t, "b", "w1", "other")}}
	in := validSaveInput("w1")
	in.ID = "a"

	saved, err := application.NewSaveHook(repo, &fakeIDs{}).Execute(context.Background(), in)

	require.NoError(t, err)
	assert.Equal(t, "a", saved.ID())
	assert.Equal(t, []string{"a", "b"}, ids(repo.hooks))
	assert.Equal(t, "make", repo.hooks[0].Script().Command)
}

// Review advisory (a): the SQL upsert silently ignores a hook of another workspace, so the use case must refuse it.
func TestSaveHookRefusesAHookOfAnotherWorkspace(t *testing.T) {
	t.Parallel()
	repo := &fakeHooks{hooks: []models.Hook{workspaceHook(t, "a", "w2", "original")}}
	in := validSaveInput("w1")
	in.ID = "a"

	_, err := application.NewSaveHook(repo, &fakeIDs{}).Execute(context.Background(), in)

	require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
	require.Len(t, repo.hooks, 1)
	assert.Equal(t, "w2", repo.hooks[0].WorkspaceID())
	assert.Equal(t, "original", repo.hooks[0].Script().Command)
}

func TestSaveHookReportsAnUnknownID(t *testing.T) {
	t.Parallel()
	in := validSaveInput("w1")
	in.ID = "missing"

	_, err := application.NewSaveHook(&fakeHooks{}, &fakeIDs{}).Execute(context.Background(), in)

	require.ErrorIs(t, err, hookerrors.ErrHookNotFound)
}

func TestDeleteHookRemovesOnlyAHookOfTheWorkspace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		workspaceID string
		hookID      string
		wantErr     error
		wantLeft    []string
	}{
		{name: "own hook", workspaceID: "w1", hookID: "a", wantLeft: []string{"b", "c"}},
		{name: "hook of another workspace", workspaceID: "w1", hookID: "c", wantErr: hookerrors.ErrInvalidHook, wantLeft: []string{"a", "b", "c"}},
		{name: "unknown hook", workspaceID: "w1", hookID: "zzz", wantErr: hookerrors.ErrHookNotFound, wantLeft: []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeHooks{hooks: []models.Hook{
				workspaceHook(t, "a", "w1", "x"), workspaceHook(t, "b", "w1", "x"), workspaceHook(t, "c", "w2", "x"),
			}}

			err := application.NewDeleteHook(repo).Execute(context.Background(), tt.workspaceID, tt.hookID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantLeft, ids(repo.hooks))
		})
	}
}

func TestReorderHooksAcceptsOnlyACompleteListOfTheWorkspace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		order   []string
		wantErr bool
	}{
		{name: "complete list", order: []string{"c", "a", "b"}},
		{name: "incomplete list", order: []string{"c", "a"}, wantErr: true},
		{name: "hook of another workspace", order: []string{"a", "b", "d"}, wantErr: true},
		{name: "duplicate id", order: []string{"a", "a", "b"}, wantErr: true},
		{name: "unknown id", order: []string{"a", "b", "zzz"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeHooks{hooks: []models.Hook{
				workspaceHook(t, "a", "w1", "x"), workspaceHook(t, "b", "w1", "x"), workspaceHook(t, "c", "w1", "x"),
				workspaceHook(t, "d", "w2", "x"),
			}}

			err := application.NewReorderHooks(repo).Execute(context.Background(), "w1", tt.order)

			if tt.wantErr {
				require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
				assert.Equal(t, []string{"a", "b", "c", "d"}, ids(repo.hooks), "a rejected order changes nothing")
				return
			}
			require.NoError(t, err)
			order, listErr := repo.List(context.Background(), "w1")
			require.NoError(t, listErr)
			assert.Equal(t, []string{"c", "a", "b"}, ids(order))
		})
	}
}
