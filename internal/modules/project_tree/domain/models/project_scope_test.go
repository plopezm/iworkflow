package models_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestBareUUIDIsAWorkspaceScope(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	scope, ok := models.ParseProjectScope(id.String())

	require.True(t, ok)
	require.Equal(t, models.WorkspaceScope(id), scope)
	require.False(t, scope.IsTask())
	require.Equal(t, id, scope.ID())
}

func TestPrefixedUUIDIsATaskScope(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	scope, ok := models.ParseProjectScope("task:" + id.String())

	require.True(t, ok)
	require.Equal(t, models.TaskScope(id), scope)
	require.True(t, scope.IsTask())
}

func TestGarbageIsRejected(t *testing.T) {
	for _, value := range []string{"nope", "task:nope", "", "task:"} {
		_, ok := models.ParseProjectScope(value)
		require.False(t, ok, "value %q must be rejected", value)
	}
}
