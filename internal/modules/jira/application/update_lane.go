//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// UpdateLane renames or re-queries a custom lane; a changed query is synced right away.
type UpdateLane struct {
	lanes outbound.LaneRepository
	sync  *SyncIssues
}

// NewUpdateLane builds the use case.
func NewUpdateLane(lanes outbound.LaneRepository, sync *SyncIssues) *UpdateLane {
	return &UpdateLane{lanes: lanes, sync: sync}
}

// Execute updates the lane. Only a changed query triggers a sync.
func (u *UpdateLane) Execute(ctx context.Context, id string, name string, jql string) (SavedLane, error) {
	definition, err := models.NewLaneDefinition(name, jql)
	if err != nil {
		return SavedLane{}, err
	}
	current, err := u.lanes.Find(ctx, id)
	if err != nil {
		return SavedLane{}, err
	}
	if current == nil {
		return SavedLane{}, jiraerrors.LaneNotFound(id)
	}
	if current.IsDefault {
		return SavedLane{}, jiraerrors.DefaultLaneImmutable()
	}
	lane, err := u.lanes.Update(ctx, id, definition)
	if err != nil {
		return SavedLane{}, err
	}
	var syncError *string
	if lane.JQL != current.JQL {
		syncError = u.sync.syncLane(ctx, lane.ID)
	}
	return SavedLane{Lane: lane, SyncError: syncError}, nil
}
