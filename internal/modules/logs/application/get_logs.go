// Package application holds the use cases of the logs module.
package application

import (
	"context"
	"fmt"

	logerrors "iworkflow/internal/modules/logs/domain/errors"
	"iworkflow/internal/modules/logs/domain/models"
	"iworkflow/internal/modules/logs/domain/ports/outbound"
)

// MaxLines is how many log lines the view shows at most.
const MaxLines int = 500

// GetLogs reads the most recent lines of the application log.
type GetLogs struct {
	reader outbound.LogReaderPort
}

// NewGetLogs builds the use case over the log reader.
func NewGetLogs(reader outbound.LogReaderPort) *GetLogs {
	return &GetLogs{reader: reader}
}

// Execute returns the tail of the log, at most MaxLines lines.
func (u *GetLogs) Execute(ctx context.Context) (models.LogTail, error) {
	tail, err := u.reader.Tail(ctx, MaxLines)
	if err != nil {
		return models.LogTail{}, fmt.Errorf("%w: %w", logerrors.ErrUnreadable, err)
	}
	return tail, nil
}
