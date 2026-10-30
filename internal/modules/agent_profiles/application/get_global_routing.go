//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// GetGlobalRouting reads the global routing (models.json).
type GetGlobalRouting struct {
	routing outbound.GlobalRoutingPort
}

// NewGetGlobalRouting builds the use case.
func NewGetGlobalRouting(routing outbound.GlobalRoutingPort) *GetGlobalRouting {
	return &GetGlobalRouting{routing: routing}
}

// Execute returns the global routing as stored.
func (u *GetGlobalRouting) Execute(ctx context.Context) (map[string]models.RoutingEntry, error) {
	return u.routing.Load(ctx)
}
