package state_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/logs/domain/models"
	"iworkflow/internal/modules/logs/presentation/state"
)

func TestLogsCardListsTheNewestLineFirstAndLeavesTheTailUntouched(t *testing.T) {
	tail := models.LogTail{Lines: []string{"old", "new"}, Truncated: true}
	props := state.LogsCard(context.Background(), tail, false)
	require.Equal(t, []string{"new", "old"}, props.Lines)
	require.Equal(t, []string{"old", "new"}, tail.Lines)
	require.NotEmpty(t, props.TruncatedNote)
	require.Empty(t, props.Error)
}

func TestLogsCardReportsAFailedReadAsAnError(t *testing.T) {
	props := state.LogsCard(context.Background(), models.LogTail{}, true)
	require.NotEmpty(t, props.Error)
	require.Empty(t, props.Lines)
}
