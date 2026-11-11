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
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

func validSaveInput() application.SaveHookInput {
	return application.SaveHookInput{
		Name: "lint", Kind: models.KindScript, Timeout: time.Minute,
		Script: models.ScriptSpec{Command: "make", Args: []string{"lint"}},
	}
}

func TestSaveHookCreatesAHookWithAGeneratedID(t *testing.T) {
	t.Parallel()
	repo := &fakeHooks{}

	hook, err := application.NewSaveHook(repo, &fakeIDs{}).Execute(context.Background(), validSaveInput())

	require.NoError(t, err)
	assert.Equal(t, "run1", hook.ID())
	assert.Equal(t, "lint", hook.Name())
	assert.Len(t, repo.hooks, 1)
}

func TestSaveHookRejectsInvalidInputWithoutSaving(t *testing.T) {
	t.Parallel()
	tests := map[string]func(*application.SaveHookInput){
		"no name":    func(in *application.SaveHookInput) { in.Name = " " },
		"no command": func(in *application.SaveHookInput) { in.Script.Command = "" },
		"no timeout": func(in *application.SaveHookInput) { in.Timeout = 0 },
		"bad kind":   func(in *application.SaveHookInput) { in.Kind = "other" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeHooks{}
			in := validSaveInput()
			mutate(&in)

			_, err := application.NewSaveHook(repo, &fakeIDs{}).Execute(context.Background(), in)

			require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
			assert.Empty(t, repo.hooks)
		})
	}
}

func TestSaveHookEditsAnExistingHookAndReportsAnUnknownID(t *testing.T) {
	t.Parallel()
	repo := &fakeHooks{hooks: []models.Hook{libraryHook(t, "a", models.KindScript)}}
	uc := application.NewSaveHook(repo, &fakeIDs{})
	in := validSaveInput()
	in.ID = "a"

	hook, err := uc.Execute(context.Background(), in)

	require.NoError(t, err)
	assert.Equal(t, "lint", hook.Name())
	require.Len(t, repo.hooks, 1)
	assert.Equal(t, "lint", repo.hooks[0].Name())

	in.ID = "missing"
	_, err = uc.Execute(context.Background(), in)
	require.ErrorIs(t, err, hookerrors.ErrHookNotFound)
}

func TestDeleteHookRemovesTheHookAndReportsAnUnknownID(t *testing.T) {
	t.Parallel()
	repo := &fakeHooks{hooks: []models.Hook{libraryHook(t, "a", models.KindScript)}}
	uc := application.NewDeleteHook(repo)

	require.NoError(t, uc.Execute(context.Background(), "a"))
	assert.Empty(t, repo.hooks)
	require.ErrorIs(t, uc.Execute(context.Background(), "a"), hookerrors.ErrHookNotFound)
}

func TestListHooksShowsAssignmentsAndTheLastFailureOfEachHook(t *testing.T) {
	t.Parallel()
	library := &fakeHooks{hooks: []models.Hook{libraryHook(t, "a", models.KindScript), libraryHook(t, "b", models.KindScript)}}
	assignments := &fakeAssignments{hooks: library, items: []models.Assignment{
		assignment(t, "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureWarn, true),
	}}
	runs := &fakeRuns{saved: []models.HookRun{
		{ID: "r1", HookID: "a", Status: models.RunFailed, Output: "first"},
		{ID: "r2", HookID: "a", Status: models.RunFailed, Output: "second"},
		{ID: "r3", HookID: "b", Status: models.RunOK},
	}}

	items, err := application.NewListHooks(library, assignments, runs).Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Len(t, items[0].Assignments, 1)
	require.NotNil(t, items[0].LastFailure)
	assert.Equal(t, "second", items[0].LastFailure.Output)
	assert.Empty(t, items[1].Assignments)
	assert.Nil(t, items[1].LastFailure, "a hook that never failed has no last failure")
}

func validAssignmentInput() application.SaveAssignmentInput {
	return application.SaveAssignmentInput{
		HookID: "a", WorkspaceID: "w1", Event: models.EventTaskFinish, Phase: models.PhasePost,
		Enabled: true, OnFailure: models.FailureBlock,
	}
}

func TestSaveAssignmentAttachesAHookToAWorkspaceEventAndPhase(t *testing.T) {
	t.Parallel()
	library := &fakeHooks{hooks: []models.Hook{libraryHook(t, "a", models.KindScript)}}
	repo := &fakeAssignments{hooks: library}
	uc := application.NewSaveAssignment(library, repo, &fakeIDs{})

	created, err := uc.Execute(context.Background(), validAssignmentInput())

	require.NoError(t, err)
	assert.Equal(t, "run1", created.ID())
	require.Len(t, repo.items, 1)

	edit := validAssignmentInput()
	edit.ID, edit.OnFailure, edit.Enabled = created.ID(), models.FailureIgnore, false
	edited, err := uc.Execute(context.Background(), edit)
	require.NoError(t, err)
	assert.Equal(t, models.FailureIgnore, edited.OnFailure())
	assert.False(t, edited.Enabled())
	assert.Len(t, repo.items, 1, "an edit does not add a second assignment")
}

func TestSaveAssignmentRefusesWhatItCannotStore(t *testing.T) {
	t.Parallel()
	library := &fakeHooks{hooks: []models.Hook{libraryHook(t, "a", models.KindScript)}}
	tests := map[string]struct {
		mutate  func(*application.SaveAssignmentInput)
		wantErr error
	}{
		"unknown hook":       {func(in *application.SaveAssignmentInput) { in.HookID = "missing" }, hookerrors.ErrHookNotFound},
		"unknown assignment": {func(in *application.SaveAssignmentInput) { in.ID = "missing" }, hookerrors.ErrAssignmentNotFound},
		"no project":         {func(in *application.SaveAssignmentInput) { in.WorkspaceID = "" }, hookerrors.ErrInvalidHook},
		"bad phase":          {func(in *application.SaveAssignmentInput) { in.Phase = "during" }, hookerrors.ErrInvalidHook},
		"bad event":          {func(in *application.SaveAssignmentInput) { in.Event = "task_discard" }, hookerrors.ErrInvalidHook},
		"bad policy":         {func(in *application.SaveAssignmentInput) { in.OnFailure = "explode" }, hookerrors.ErrInvalidHook},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeAssignments{hooks: library}
			in := validAssignmentInput()
			tt.mutate(&in)

			_, err := application.NewSaveAssignment(library, repo, &fakeIDs{}).Execute(context.Background(), in)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Empty(t, repo.items)
		})
	}
}

func TestDeleteAssignmentKeepsTheHook(t *testing.T) {
	t.Parallel()
	repo := assignmentsOf(t, "w1")
	uc := application.NewDeleteAssignment(repo)

	require.NoError(t, uc.Execute(context.Background(), "as-a"))
	assert.Empty(t, repo.items)
	assert.Len(t, repo.hooks.hooks, 1)
	require.ErrorIs(t, uc.Execute(context.Background(), "as-a"), hookerrors.ErrAssignmentNotFound)
}

func TestReorderAssignmentsAcceptsOnlyACompleteListOfTheGroup(t *testing.T) {
	t.Parallel()
	library := &fakeHooks{hooks: []models.Hook{libraryHook(t, "a", models.KindScript), libraryHook(t, "b", models.KindScript)}}
	build := func() *fakeAssignments {
		return &fakeAssignments{hooks: library, items: []models.Assignment{
			assignment(t, "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureWarn, true),
			assignment(t, "b", "w1", models.EventTaskStart, models.PhasePre, models.FailureWarn, true),
		}}
	}
	tests := map[string]struct {
		ids     []string
		wantErr error
	}{
		"complete":  {[]string{"as-b", "as-a"}, nil},
		"partial":   {[]string{"as-a"}, hookerrors.ErrInvalidHook},
		"repeated":  {[]string{"as-a", "as-a"}, hookerrors.ErrInvalidHook},
		"unknown":   {[]string{"as-a", "as-z"}, hookerrors.ErrInvalidHook},
		"extra one": {[]string{"as-a", "as-b", "as-c"}, hookerrors.ErrInvalidHook},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := build()

			err := application.NewReorderAssignments(repo).Execute(context.Background(), "w1", models.EventTaskStart, models.PhasePre, tt.ids)

			if tt.wantErr == nil {
				require.NoError(t, err)
				assert.Equal(t, tt.ids, repo.reorderedTo)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, repo.reorderedTo, "a refused order changes nothing")
		})
	}
}

func TestListRunsPassesTheFilterToTheHistory(t *testing.T) {
	t.Parallel()
	runs := &fakeRuns{saved: []models.HookRun{
		{ID: "r1", HookID: "a", WorkspaceID: "w1", Status: models.RunOK},
		{ID: "r2", HookID: "a", WorkspaceID: "w1", Status: models.RunFailed},
		{ID: "r3", HookID: "b", WorkspaceID: "w2", Status: models.RunErrored},
	}}
	uc := application.NewListRuns(runs)

	failed, err := uc.Execute(context.Background(), outbound.RunFilter{FailedOnly: true})
	require.NoError(t, err)
	require.Len(t, failed, 2)
	assert.Equal(t, "r3", failed[0].ID, "newest first")

	byHook, err := uc.Execute(context.Background(), outbound.RunFilter{HookID: "a"})
	require.NoError(t, err)
	assert.Len(t, byHook, 2)
}
