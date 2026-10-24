//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// DeleteLane removes a custom lane. Issues also present in another lane, converted or ignored are kept.
type DeleteLane struct {
	lanes outbound.LaneRepository
}

// NewDeleteLane builds the use case.
func NewDeleteLane(lanes outbound.LaneRepository) *DeleteLane {
	return &DeleteLane{lanes: lanes}
}

// Execute removes the lane. The built-in lane and unknown identifiers are refused.
func (u *DeleteLane) Execute(ctx context.Context, id string) error {
	lane, err := u.lanes.Find(ctx, id)
	if err != nil {
		return err
	}
	if lane == nil {
		return jiraerrors.LaneNotFound(id)
	}
	if lane.IsDefault {
		return jiraerrors.DefaultLaneImmutable()
	}
	return u.lanes.Delete(ctx, id)
}
