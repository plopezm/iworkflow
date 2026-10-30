//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// DeleteProfile removes a profile. The active profile cannot be deleted.
type DeleteProfile struct {
	store outbound.ProfileStorePort
	guard *WriteGuard
}

// NewDeleteProfile builds the use case. guard is shared with every use case that mutates profiles.json.
func NewDeleteProfile(store outbound.ProfileStorePort, guard *WriteGuard) *DeleteProfile {
	return &DeleteProfile{store: store, guard: guard}
}

// Execute deletes the named profile.
func (u *DeleteProfile) Execute(ctx context.Context, name models.ProfileName) error {
	if err := u.guard.lock(ctx); err != nil {
		return err
	}
	defer u.guard.unlock()
	store, err := u.store.Load(ctx)
	if err != nil {
		return err
	}
	if !store.Contains(name) {
		return agenterrors.ProfileNotFound(name.String())
	}
	if store.Active != nil && *store.Active == name {
		return agenterrors.CannotDeleteActiveProfile(name.String())
	}
	kept := make([]models.Profile, 0, len(store.Profiles))
	for _, profile := range store.Profiles {
		if profile.Name != name {
			kept = append(kept, profile)
		}
	}
	store.Profiles = kept
	return u.store.Save(ctx, store)
}
