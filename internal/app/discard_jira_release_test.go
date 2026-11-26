package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
)

type fakeIssueReleaser struct {
	released []string
	err      error
	failures int // the first calls that fail before one works
}

func (f *fakeIssueReleaser) Execute(_ context.Context, taskID string) error {
	f.released = append(f.released, taskID)
	if f.failures > 0 {
		f.failures--
		return errors.New("transient")
	}
	return f.err
}

func TestDiscardedTaskReleasesItsJiraIssueAndSurvivesAFailure(t *testing.T) {
	id, err := tasksmodels.NewTaskID()
	require.NoError(t, err)
	releaser := &fakeIssueReleaser{}
	bridge := &taskDiscardJiraRelease{inner: releaser, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	bridge.Discarded(context.Background(), id)
	assert.Equal(t, []string{id.String()}, releaser.released)

	releaser.err = errors.New("db down")
	assert.NotPanics(t, func() { bridge.Discarded(context.Background(), id) })

	unbound := &taskDiscardJiraRelease{}
	assert.NotPanics(t, func() { unbound.Discarded(context.Background(), id) }, "before the jira module is bound nothing happens")
}

func TestDiscardedTaskRetriesTheReleaseAndGivesUpAfterTheAttempts(t *testing.T) {
	id, err := tasksmodels.NewTaskID()
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	flaky := &fakeIssueReleaser{failures: 2}
	(&taskDiscardJiraRelease{inner: flaky, logger: logger}).Discarded(context.Background(), id)
	assert.Len(t, flaky.released, 3, "a transient failure is retried until it works")

	broken := &fakeIssueReleaser{err: errors.New("db down")}
	(&taskDiscardJiraRelease{inner: broken, logger: logger}).Discarded(context.Background(), id)
	assert.Len(t, broken.released, discardReleaseAttempts)
}
