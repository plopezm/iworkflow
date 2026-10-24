//nolint:wrapcheck // Port errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"

	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// SetLaneCollapsed remembers whether a lane (any, including the default one) is folded away.
type SetLaneCollapsed struct {
	lanes outbound.LaneRepository
}

// NewSetLaneCollapsed builds the use case.
func NewSetLaneCollapsed(lanes outbound.LaneRepository) *SetLaneCollapsed {
	return &SetLaneCollapsed{lanes: lanes}
}

// Execute stores the flag.
func (u *SetLaneCollapsed) Execute(ctx context.Context, id string, collapsed bool) error {
	return u.lanes.SetCollapsed(ctx, id, collapsed)
}
