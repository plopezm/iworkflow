package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// fakeClock moves forward only when fakeSleep is called, so elapsed time is exact and no test waits.
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	c.now = c.now.Add(d)
	return nil
}

// flakyFind is an inner repository whose Find fails with a busy error the first failures times.
// Its other methods are not expected to be called (the embedded nil interface panics if they are).
type flakyFind struct {
	outbound.TaskRepository
	failures  int
	calls     int
	permanent error
}

func (r *flakyFind) Find(_ context.Context, _ models.TaskID) (*models.Task, error) {
	r.calls++
	if r.permanent != nil {
		return nil, r.permanent
	}
	if r.calls <= r.failures {
		return nil, werrors.Busy("database is locked")
	}
	return nil, nil
}

// recordingAppend fails the first failures appends and records every payload it receives.
type recordingAppend struct {
	outbound.TaskRepository
	failures int
	payloads []models.TaskEventKind
}

func (r *recordingAppend) AppendEvent(_ context.Context, id models.TaskID, kind models.TaskEventKind) (models.TaskEvent, error) {
	r.payloads = append(r.payloads, kind)
	if len(r.payloads) <= r.failures {
		return models.TaskEvent{}, werrors.Busy("database is locked")
	}
	seq := int64(1)
	return models.TaskEvent{TaskID: id, Seq: &seq, Kind: kind, CreatedAt: time.Now()}, nil
}

func newDecorated(inner outbound.TaskRepository, policy RetryPolicy, clock *fakeClock) *RetryingTaskRepository {
	return NewRetryingTaskRepository(inner, policy, clock.Now, clock.Sleep)
}

func TestRetryingSucceedsOnFirstCallWithoutRetry(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: fixedInstant}
	inner := &flakyFind{}
	repo := newDecorated(inner, BoundedRetryPolicy(), clock)

	task, err := repo.Find(context.Background(), newID(t))
	require.NoError(t, err)
	assert.Nil(t, task)
	assert.Equal(t, 1, inner.calls)
	assert.Equal(t, fixedInstant, clock.now)
}

func TestRetryingRetriesBusyUntilOK(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: fixedInstant}
	inner := &flakyFind{failures: 2}
	repo := newDecorated(inner, BoundedRetryPolicy(), clock)

	_, err := repo.Find(context.Background(), newID(t))
	require.NoError(t, err)
	assert.Equal(t, 3, inner.calls)
}

func TestRetryingReturnsNonTransientErrorImmediately(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: fixedInstant}
	inner := &flakyFind{permanent: werrors.Repository("boom")}
	repo := newDecorated(inner, PersistentRetryPolicy(), clock)

	_, err := repo.Find(context.Background(), newID(t))
	assert.ErrorIs(t, err, werrors.Repository("boom"))
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingBoundedPolicyGivesUpAfterMaxElapsed(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: fixedInstant}
	inner := &flakyFind{failures: 1 << 30}
	repo := newDecorated(inner, BoundedRetryPolicy(), clock)

	_, err := repo.Find(context.Background(), newID(t))
	assert.ErrorIs(t, err, werrors.Busy(""))
	elapsed := clock.now.Sub(fixedInstant)
	assert.GreaterOrEqual(t, elapsed, BoundedMaxElapsed)
	assert.Less(t, elapsed, BoundedMaxElapsed+MaxRetryDelay)
}

func TestRetryingPersistentPolicyRetriesPastTheBoundedLimit(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: fixedInstant}
	const failures = 60
	inner := &flakyFind{failures: failures}
	repo := newDecorated(inner, PersistentRetryPolicy(), clock)

	_, err := repo.Find(context.Background(), newID(t))
	require.NoError(t, err)
	assert.Equal(t, failures+1, inner.calls)
	assert.Greater(t, clock.now.Sub(fixedInstant), BoundedMaxElapsed)
}

func TestRetryingAppendEventIsRetriedWithTheSamePayload(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: fixedInstant}
	inner := &recordingAppend{failures: 2}
	repo := newDecorated(inner, BoundedRetryPolicy(), clock)
	kind := models.StatusKind(models.StatusBacklog, nil)

	event, err := repo.AppendEvent(context.Background(), newID(t), kind)
	require.NoError(t, err)
	assert.Equal(t, kind, event.Kind)
	require.Len(t, inner.payloads, 3)
	for _, payload := range inner.payloads {
		assert.Equal(t, kind, payload)
	}
}

func TestRetryingDelaysGrowExponentiallyAndCapAtMaxDelay(t *testing.T) {
	t.Parallel()
	clock := &fakeClock{now: fixedInstant}
	// Eight failures wait 25+50+100+200+400+800+1000+1000 ms.
	inner := &flakyFind{failures: 8}
	repo := newDecorated(inner, PersistentRetryPolicy(), clock)

	_, err := repo.Find(context.Background(), newID(t))
	require.NoError(t, err)
	assert.Equal(t, 3575*time.Millisecond, clock.now.Sub(fixedInstant))
}

var fixedInstant = time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)

func newID(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return id
}
