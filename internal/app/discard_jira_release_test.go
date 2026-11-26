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
}

func (f *fakeIssueReleaser) Execute(_ context.Context, taskID string) error {
	f.released = append(f.released, taskID)
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
