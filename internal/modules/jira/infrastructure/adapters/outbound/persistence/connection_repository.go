// Package persistence implements the jira repository ports on SQLite through sqlc.
package persistence

import (
	"context"
	"database/sql"
	"time"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/persistence/gen"
)

// connectionRowID is the fixed primary key of the single connection row.
const connectionRowID int64 = 1

// ConnectionRepository stores the single Jira connection in the jira_connection table.
type ConnectionRepository struct {
	queries *gen.Queries
	now     func() time.Time
}

var _ outbound.ConnectionRepository = (*ConnectionRepository)(nil)

// NewConnectionRepository builds the repository on an open database. now stamps updated_at.
func NewConnectionRepository(db *sql.DB, now func() time.Time) *ConnectionRepository {
	return &ConnectionRepository{queries: gen.New(db), now: now}
}

// Get returns nil and no error when no connection is stored.
func (r *ConnectionRepository) Get(ctx context.Context) (*models.Connection, error) {
	row, err := r.queries.GetJiraConnection(ctx, connectionRowID)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, dbError(err)
	}
	lastSyncAt, err := parseOptionalTimestamp(row.LastSyncAt)
	if err != nil {
		return nil, err
	}
	connection := models.RestoreConnection(
		row.BaseUrl,
		row.Token,
		row.AcceptInvalidCerts != 0,
		lastSyncAt,
		optionalString(row.LastSyncError),
	)
	return &connection, nil
}

// Save stores the connection. The statement is a single upsert, so it is atomic. The sync bookkeeping
// of the stored row is kept only while the server URL does not change (see UpsertJiraConnection).
func (r *ConnectionRepository) Save(ctx context.Context, connection models.Connection) error {
	var lastSyncAt *string
	if at := connection.LastSyncAt(); at != nil {
		formatted := formatTimestamp(*at)
		lastSyncAt = &formatted
	}
	err := r.queries.UpsertJiraConnection(ctx, gen.UpsertJiraConnectionParams{
		ID:                 connectionRowID,
		BaseUrl:            connection.BaseURL(),
		Token:              connection.Token(),
		AcceptInvalidCerts: boolToInt(connection.AcceptInvalidCerts()),
		LastSyncAt:         lastSyncAt,
		LastSyncError:      nullString(connection.LastSyncError()),
		UpdatedAt:          formatTimestamp(r.now()),
	})
	if err != nil {
		return dbError(err)
	}
	return nil
}

// Delete removes the connection.
func (r *ConnectionRepository) Delete(ctx context.Context) error {
	if err := r.queries.DeleteJiraConnection(ctx); err != nil {
		return dbError(err)
	}
	return nil
}

// RecordSync stores the outcome of the last sync. A nil syncError means success.
func (r *ConnectionRepository) RecordSync(ctx context.Context, at time.Time, syncError *string) error {
	formatted := formatTimestamp(at)
	_, err := r.queries.RecordJiraSync(ctx, gen.RecordJiraSyncParams{
		ID:            connectionRowID,
		LastSyncAt:    &formatted,
		LastSyncError: nullString(syncError),
	})
	if err != nil {
		return dbError(err)
	}
	return nil
}

func boolToInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
