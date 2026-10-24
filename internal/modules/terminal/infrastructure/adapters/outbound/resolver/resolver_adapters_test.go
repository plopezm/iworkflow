package resolver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/outbound/resolver"
)

func TestWorkspacePathAdapterReturnsTheFoundTarget(t *testing.T) {
	target := models.WorkspaceTarget{Name: "demo", Path: "/tmp/x"}
	adapter := resolver.NewWorkspacePathAdapter(func(context.Context, uuid.UUID) (*models.WorkspaceTarget, error) {
		return &target, nil
	})

	got, err := adapter.Resolve(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Equal(t, target, got)
}

func TestWorkspacePathAdapterMapsAMissingWorkspaceToNotFound(t *testing.T) {
	adapter := resolver.NewWorkspacePathAdapter(func(context.Context, uuid.UUID) (*models.WorkspaceTarget, error) {
		return nil, nil
	})

	_, err := adapter.Resolve(context.Background(), uuid.New())

	require.ErrorIs(t, err, terminalerrors.ErrWorkspaceNotFound)
}

func TestWorkspacePathAdapterMapsALookupFailureToAPtyError(t *testing.T) {
	adapter := resolver.NewWorkspacePathAdapter(func(context.Context, uuid.UUID) (*models.WorkspaceTarget, error) {
		return nil, errors.New("boom")
	})

	_, err := adapter.Resolve(context.Background(), uuid.New())

	require.ErrorIs(t, err, terminalerrors.ErrPty)
	require.EqualError(t, err, "error del pseudo-terminal: boom")
}

func TestTaskPathAdapterReturnsTheFoundTarget(t *testing.T) {
	target := models.TaskTarget{WorkspaceID: uuid.New(), Name: "t", Path: "/tmp/wt"}
	adapter := resolver.NewTaskPathAdapter(func(context.Context, uuid.UUID) (*models.TaskTarget, error) {
		return &target, nil
	})

	got, err := adapter.Resolve(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Equal(t, target, got)
}

func TestTaskPathAdapterMapsAMissingTaskToNotFound(t *testing.T) {
	adapter := resolver.NewTaskPathAdapter(func(context.Context, uuid.UUID) (*models.TaskTarget, error) {
		return nil, nil
	})

	_, err := adapter.Resolve(context.Background(), uuid.New())

	require.ErrorIs(t, err, terminalerrors.ErrTaskNotFound)
}

func TestTaskPathAdapterMapsALookupFailureToAPtyError(t *testing.T) {
	adapter := resolver.NewTaskPathAdapter(func(context.Context, uuid.UUID) (*models.TaskTarget, error) {
		return nil, errors.New("boom")
	})

	_, err := adapter.Resolve(context.Background(), uuid.New())

	require.EqualError(t, err, "error del pseudo-terminal: boom")
}
