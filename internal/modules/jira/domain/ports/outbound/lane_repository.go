package outbound

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
)

// LaneRepository stores the lanes of the Jira page.
type LaneRepository interface {
	// List returns every lane in display order: the default lane first, then custom ones by creation.
	List(ctx context.Context) ([]models.Lane, error)
	// Find returns nil and no error when there is no such lane.
	Find(ctx context.Context, id string) (*models.Lane, error)
	// Create appends a custom lane (expanded) and returns it.
	Create(ctx context.Context, definition models.LaneDefinition) (models.Lane, error)
	// Update renames or re-queries a custom lane; the default lane is not editable.
	Update(ctx context.Context, id string, definition models.LaneDefinition) (models.Lane, error)
	// Delete removes a custom lane with its membership and the New issues no other lane contains.
	Delete(ctx context.Context, id string) error
	SetCollapsed(ctx context.Context, id string, collapsed bool) error
}
