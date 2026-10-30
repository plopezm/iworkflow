package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
)

// Rust: get_task_events::delegates_cursor_to_repository
func TestGetTaskEventsDelegatesTheCursorToTheRepository(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository()
	id := newTaskIDValue(t)
	seq := int64(7)

	events, err := application.NewGetTaskEvents(repo).Execute(context.Background(), id, &seq)

	require.NoError(t, err)
	assert.Empty(t, events)
	require.NotNil(t, repo.eventsAfter)
	assert.Equal(t, seq, *repo.eventsAfter)
	assert.Equal(t, id, repo.eventsTask)
}
