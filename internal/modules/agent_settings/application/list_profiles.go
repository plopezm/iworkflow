package application

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

// ListProfiles lists the agent profiles (`profiles.json`) and which one is active.
type ListProfiles struct {
	catalog outbound.ProfileCatalogPort
}

// NewListProfiles builds the use case.
func NewListProfiles(catalog outbound.ProfileCatalogPort) *ListProfiles {
	return &ListProfiles{catalog: catalog}
}

// Execute reads the profile catalog.
func (u *ListProfiles) Execute(ctx context.Context) (models.ProfileCatalog, error) {
	catalog, err := u.catalog.List(ctx)
	return catalog, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
