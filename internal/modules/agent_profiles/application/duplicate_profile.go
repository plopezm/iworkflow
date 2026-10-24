//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// DuplicateProfile copies the routing of an existing profile into a new one.
type DuplicateProfile struct {
	store outbound.ProfileStorePort
	guard *WriteGuard
}

// NewDuplicateProfile builds the use case. guard is shared with every use case that mutates profiles.json.
func NewDuplicateProfile(store outbound.ProfileStorePort, guard *WriteGuard) *DuplicateProfile {
	return &DuplicateProfile{store: store, guard: guard}
}

// Execute copies the routing of source into target and returns the new profile.
func (u *DuplicateProfile) Execute(ctx context.Context, source models.ProfileName, target models.ProfileName) (models.Profile, error) {
	if err := u.guard.lock(ctx); err != nil {
		return models.Profile{}, err
	}
	defer u.guard.unlock()
	store, err := u.store.Load(ctx)
	if err != nil {
		return models.Profile{}, err
	}
	existing := store.Find(source)
	if existing == nil {
		return models.Profile{}, agenterrors.ProfileNotFound(source.String())
	}
	routing := models.CloneRouting(existing.Routing)
	if store.Contains(target) {
		return models.Profile{}, agenterrors.ProfileAlreadyExists(target.String())
	}
	profile := models.NewProfile(target, routing)
	store.Profiles = append(store.Profiles, profile)
	if err := u.store.Save(ctx, store); err != nil {
		return models.Profile{}, err
	}
	return profile, nil
}
