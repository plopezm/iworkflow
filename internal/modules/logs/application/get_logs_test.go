package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/logs/application"
	logerrors "iworkflow/internal/modules/logs/domain/errors"
	"iworkflow/internal/modules/logs/domain/models"
)

type readerStub struct {
	tail models.LogTail
	err  error
	max  int
}

func (r *readerStub) Tail(_ context.Context, maxLines int) (models.LogTail, error) {
	r.max = maxLines
	return r.tail, r.err
}

func TestGetLogsAsksForAtMostMaxLinesAndReturnsTheTail(t *testing.T) {
	reader := &readerStub{tail: models.LogTail{Lines: []string{"a", "b"}, Truncated: true}}
	tail, err := application.NewGetLogs(reader).Execute(context.Background())
	require.NoError(t, err)
	require.Equal(t, application.MaxLines, reader.max)
	require.Equal(t, reader.tail, tail)
}

func TestGetLogsWrapsAReadFailureAsUnreadable(t *testing.T) {
	_, err := application.NewGetLogs(&readerStub{err: errors.New("disk")}).Execute(context.Background())
	require.ErrorIs(t, err, logerrors.ErrUnreadable)
}
