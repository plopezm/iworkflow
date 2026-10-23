package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

// ModelCatalogPort lists the models available in the executable of a runtime.
type ModelCatalogPort interface {
	List(ctx context.Context, runtime models.AgentRuntime) ([]models.ModelRef, error)
}
