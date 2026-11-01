package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

// RuntimeAvailabilityPort tells whether the executable of a runtime exists on this machine.
type RuntimeAvailabilityPort interface {
	Installed(ctx context.Context, runtime models.AgentRuntime) bool
}
