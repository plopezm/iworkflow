package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
)

// The runs still marked running when the process starts belong to a process that died: they are closed as errors
// at the time of the start, and the count of closed runs is returned.
func TestReconcileHookRunsClosesStaleRunsAtTheStartTime(t *testing.T) {
	t.Parallel()
	runs := &fakeRuns{stale: 3}
	clock := &fakeClock{now: time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)}

	got, err := application.NewReconcileHookRuns(runs, clock).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(3), got)
	assert.Equal(t, 1, runs.staleCalls)
	assert.Equal(t, time.Date(2026, 11, 7, 9, 0, 1, 0, time.UTC), runs.staleAt, "the clock is read once")
}

func TestReconcileHookRunsWrapsStorageFailures(t *testing.T) {
	t.Parallel()
	runs := &fakeRuns{staleErr: context.DeadlineExceeded}

	_, err := application.NewReconcileHookRuns(runs, &fakeClock{}).Execute(context.Background())

	require.ErrorIs(t, err, context.DeadlineExceeded)
}
