//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// SetGlobalRouting validates and stores the global routing (models.json).
type SetGlobalRouting struct {
	routing outbound.GlobalRoutingPort
}

// NewSetGlobalRouting builds the use case.
func NewSetGlobalRouting(routing outbound.GlobalRoutingPort) *SetGlobalRouting {
	return &SetGlobalRouting{routing: routing}
}

// Execute stores routing when every entry is valid.
func (u *SetGlobalRouting) Execute(ctx context.Context, routing map[string]models.RoutingEntry) error {
	if err := models.ValidateRouting(routing); err != nil {
		return err
	}
	return u.routing.Save(ctx, routing)
}
