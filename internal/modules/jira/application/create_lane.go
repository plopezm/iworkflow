//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// SavedLane is a lane that was stored, plus the failure of its first sync (the lane itself is kept).
type SavedLane struct {
	Lane      models.Lane
	SyncError *string
}

// CreateLane adds a custom lane and fills it right away.
type CreateLane struct {
	lanes outbound.LaneRepository
	sync  *SyncIssues
}

// NewCreateLane builds the use case.
func NewCreateLane(lanes outbound.LaneRepository, sync *SyncIssues) *CreateLane {
	return &CreateLane{lanes: lanes, sync: sync}
}

// Execute validates the definition, stores the lane and syncs it. A failing sync is reported, not raised.
func (u *CreateLane) Execute(ctx context.Context, name string, jql string) (SavedLane, error) {
	definition, err := models.NewLaneDefinition(name, jql)
	if err != nil {
		return SavedLane{}, err
	}
	lane, err := u.lanes.Create(ctx, definition)
	if err != nil {
		return SavedLane{}, err
	}
	return SavedLane{Lane: lane, SyncError: u.sync.syncLane(ctx, lane.ID)}, nil
}
