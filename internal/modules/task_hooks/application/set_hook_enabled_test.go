package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
)

// Toggling changes only the enabled flag: the command, the position and the rest of the hook are kept as they were.
func TestSetHookEnabledChangesOnlyTheFlag(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	before := savedHook(t, "h1", "w1", models.KindScript, models.PhasePre, models.EventTaskStart)
	hooks := &fakeHooks{hooks: []models.Hook{before}}
	useCase := application.NewSetHookEnabled(hooks)

	require.NoError(t, useCase.Execute(ctx, application.SetHookEnabledInput{WorkspaceID: "w1", HookID: "h1", Enabled: false}))

	after, err := hooks.Get(ctx, "h1")
	require.NoError(t, err)
	assert.False(t, after.Enabled())
	assert.Equal(t, before.Script(), after.Script())
	assert.Equal(t, before.Timeout(), after.Timeout())
	assert.Equal(t, before.OnFailure(), after.OnFailure())
	assert.Equal(t, before.Phase(), after.Phase())
}

func TestSetHookEnabledRefusesWhatItCannotChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tests := []struct {
		name    string
		input   application.SetHookEnabledInput
		wantErr error
	}{
		{name: "unknown hook", input: application.SetHookEnabledInput{WorkspaceID: "w1", HookID: "missing"}, wantErr: hookerrors.ErrHookNotFound},
		{name: "hook of another workspace", input: application.SetHookEnabledInput{WorkspaceID: "w2", HookID: "h1"}, wantErr: hookerrors.ErrInvalidHook},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hooks := &fakeHooks{hooks: []models.Hook{savedHook(t, "h1", "w1", models.KindScript, models.PhasePre, models.EventTaskStart)}}

			err := application.NewSetHookEnabled(hooks).Execute(ctx, tt.input)

			require.ErrorIs(t, err, tt.wantErr)
			stored, getErr := hooks.Get(ctx, "h1")
			require.NoError(t, getErr)
			assert.True(t, stored.Enabled(), "a refused toggle must leave the hook as it was")
		})
	}
}
