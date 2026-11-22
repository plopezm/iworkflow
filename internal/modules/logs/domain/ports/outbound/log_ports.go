// Package outbound holds the ports the logs module needs from the infrastructure.
package outbound

import (
	"context"

	"iworkflow/internal/modules/logs/domain/models"
)

// LogReaderPort reads the end of the application log.
type LogReaderPort interface {
	// Tail returns at most maxLines of the most recent lines. A log that does not exist yet is an
	// empty tail, not an error.
	Tail(ctx context.Context, maxLines int) (models.LogTail, error)
}
