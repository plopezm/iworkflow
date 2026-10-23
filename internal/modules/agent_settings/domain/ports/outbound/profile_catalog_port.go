package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

// ProfileCatalogPort lists the agent profiles known to gentle-pi.
type ProfileCatalogPort interface {
	List(ctx context.Context) (models.ProfileCatalog, error)
}
