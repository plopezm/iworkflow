//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"
	"time"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/persistence/gen"
)

// LaneRepository stores the lanes of the Jira page in jira_lanes.
type LaneRepository struct {
	db      *sql.DB
	queries *gen.Queries
	now     func() time.Time
	newID   func() string
}

var _ outbound.LaneRepository = (*LaneRepository)(nil)

// NewLaneRepository builds the repository on an open database. now stamps created_at and newID
// identifies each new custom lane.
func NewLaneRepository(db *sql.DB, now func() time.Time, newID func() string) *LaneRepository {
	return &LaneRepository{db: db, queries: gen.New(db), now: now, newID: newID}
}

// List returns every lane in display order: the default lane first, then custom ones by creation.
func (r *LaneRepository) List(ctx context.Context) ([]models.Lane, error) {
	rows, err := r.queries.ListJiraLanes(ctx)
	if err != nil {
		return nil, dbError(err)
	}
	lanes := make([]models.Lane, 0, len(rows))
	for _, row := range rows {
		lanes = append(lanes, laneFromRow(row))
	}
	return lanes, nil
}

// Find returns nil and no error when there is no such lane.
func (r *LaneRepository) Find(ctx context.Context, id string) (*models.Lane, error) {
	row, err := r.queries.GetJiraLane(ctx, id)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, dbError(err)
	}
	lane := laneFromRow(row)
	return &lane, nil
}

// Create appends a custom lane. Its position is computed by the insert itself, so two creations
// cannot take the same slot.
func (r *LaneRepository) Create(ctx context.Context, definition models.LaneDefinition) (models.Lane, error) {
	id := r.newID()
	if _, err := r.queries.InsertJiraLaneAtEnd(ctx, gen.InsertJiraLaneAtEndParams{
		ID:        id,
		Name:      definition.Name(),
		Jql:       definition.JQL(),
		CreatedAt: formatTimestamp(r.now()),
	}); err != nil {
		return models.Lane{}, dbError(err)
	}
	created, err := r.queries.GetJiraLane(ctx, id)
	if err != nil {
		return models.Lane{}, dbError(err)
	}
	return laneFromRow(created), nil
}

// Update renames or re-queries a custom lane. The default lane is refused and an unknown id is reported.
func (r *LaneRepository) Update(ctx context.Context, id string, definition models.LaneDefinition) (models.Lane, error) {
	updated, err := r.queries.UpdateJiraLaneDefinition(ctx, gen.UpdateJiraLaneDefinitionParams{
		Name: definition.Name(),
		Jql:  definition.JQL(),
		ID:   id,
	})
	if err != nil {
		return models.Lane{}, dbError(err)
	}
	if updated == 0 {
		return models.Lane{}, r.refusal(ctx, id)
	}
	row, err := r.queries.GetJiraLane(ctx, id)
	if err != nil {
		return models.Lane{}, dbError(err)
	}
	return laneFromRow(row), nil
}

// Delete removes a custom lane with its membership, then the New issues no other lane contains. The
// three statements run in one transaction.
func (r *LaneRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return dbError(err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := r.queries.WithTx(tx)
	// The membership goes first: it is the write that takes the database lock of the transaction.
	if err := queries.DeleteJiraLaneMemberships(ctx, id); err != nil {
		return dbError(err)
	}
	deleted, err := queries.DeleteCustomJiraLane(ctx, id)
	if err != nil {
		return dbError(err)
	}
	if deleted == 0 {
		// Nothing was removed, so the rollback restores the membership.
		return r.refusal(ctx, id)
	}
	if err := queries.DeleteRetiredNewJiraIssues(ctx, models.IssueLocalStateNew.String()); err != nil {
		return dbError(err)
	}
	if err := tx.Commit(); err != nil {
		return dbError(err)
	}
	return nil
}

// SetCollapsed stores whether a lane is folded away, the default one included.
func (r *LaneRepository) SetCollapsed(ctx context.Context, id string, collapsed bool) error {
	updated, err := r.queries.SetJiraLaneCollapsed(ctx, gen.SetJiraLaneCollapsedParams{
		Collapsed: boolToInt(collapsed),
		ID:        id,
	})
	if err != nil {
		return dbError(err)
	}
	if updated == 0 {
		return jiraerrors.LaneNotFound(id)
	}
	return nil
}

// refusal explains why a write that touched no row was refused: the lane is unknown, or it is the
// built-in one.
func (r *LaneRepository) refusal(ctx context.Context, id string) error {
	row, err := r.queries.GetJiraLane(ctx, id)
	if isNoRows(err) {
		return jiraerrors.LaneNotFound(id)
	}
	if err != nil {
		return dbError(err)
	}
	if row.IsDefault != 0 {
		return jiraerrors.DefaultLaneImmutable()
	}
	return jiraerrors.LaneNotFound(id)
}

func laneFromRow(row gen.JiraLane) models.Lane {
	return models.Lane{
		ID:        row.ID,
		Name:      row.Name,
		JQL:       row.Jql,
		IsDefault: row.IsDefault != 0,
		Collapsed: row.Collapsed != 0,
	}
}
