//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// ListProfiles returns the whole profile store, active marker included.
type ListProfiles struct {
	store outbound.ProfileStorePort
}

// NewListProfiles builds the use case.
func NewListProfiles(store outbound.ProfileStorePort) *ListProfiles {
	return &ListProfiles{store: store}
}

// Execute loads the store as it is on disk.
func (u *ListProfiles) Execute(ctx context.Context) (models.ProfileStore, error) {
	return u.store.Load(ctx)
}
