//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// RenameProfile renames a profile. When it was the active one, the active marker follows it.
type RenameProfile struct {
	store outbound.ProfileStorePort
	guard *WriteGuard
}

// NewRenameProfile builds the use case. guard is shared with every use case that mutates profiles.json.
func NewRenameProfile(store outbound.ProfileStorePort, guard *WriteGuard) *RenameProfile {
	return &RenameProfile{store: store, guard: guard}
}

// Execute renames from to to and returns the renamed profile.
func (u *RenameProfile) Execute(ctx context.Context, from models.ProfileName, to models.ProfileName) (models.Profile, error) {
	if err := u.guard.lock(ctx); err != nil {
		return models.Profile{}, err
	}
	defer u.guard.unlock()
	store, err := u.store.Load(ctx)
	if err != nil {
		return models.Profile{}, err
	}
	profile := store.Find(from)
	if profile == nil {
		return models.Profile{}, agenterrors.ProfileNotFound(from.String())
	}
	if store.Contains(to) {
		return models.Profile{}, agenterrors.ProfileAlreadyExists(to.String())
	}
	if store.Active != nil && *store.Active == from {
		renamedActive := to
		store.Active = &renamedActive
	}
	profile.Name = to
	renamed := *profile
	if err := u.store.Save(ctx, store); err != nil {
		return models.Profile{}, err
	}
	return renamed, nil
}
