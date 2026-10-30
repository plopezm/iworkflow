package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
)

// GlobalRoutingPort reads and writes the global routing (models.json).
type GlobalRoutingPort interface {
	// Load returns an empty routing when the file does not exist.
	Load(ctx context.Context) (map[string]models.RoutingEntry, error)
	Save(ctx context.Context, routing map[string]models.RoutingEntry) error
}
