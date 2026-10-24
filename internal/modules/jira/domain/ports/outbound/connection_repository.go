package outbound

import (
	"context"
	"time"

	"iworkflow/internal/modules/jira/domain/models"
)

// ConnectionRepository stores the single Jira connection.
type ConnectionRepository interface {
	// Get returns nil and no error when there is no connection.
	Get(ctx context.Context) (*models.Connection, error)
	// Save inserts or replaces the single connection. Sync bookkeeping is kept only while the server
	// URL is unchanged.
	Save(ctx context.Context, connection models.Connection) error
	Delete(ctx context.Context) error
	// RecordSync stores the outcome of the last sync; a nil syncError means success.
	RecordSync(ctx context.Context, at time.Time, syncError *string) error
}
