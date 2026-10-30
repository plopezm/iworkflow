//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// CreateProfile adds an empty profile (no routing) and leaves the active marker untouched.
type CreateProfile struct {
	store outbound.ProfileStorePort
	guard *WriteGuard
}

// NewCreateProfile builds the use case. guard is shared with every use case that mutates profiles.json.
func NewCreateProfile(store outbound.ProfileStorePort, guard *WriteGuard) *CreateProfile {
	return &CreateProfile{store: store, guard: guard}
}

// Execute creates the profile and returns it. It fails when the name is already taken.
func (u *CreateProfile) Execute(ctx context.Context, name models.ProfileName) (models.Profile, error) {
	if err := u.guard.lock(ctx); err != nil {
		return models.Profile{}, err
	}
	defer u.guard.unlock()
	store, err := u.store.Load(ctx)
	if err != nil {
		return models.Profile{}, err
	}
	if store.Contains(name) {
		return models.Profile{}, agenterrors.ProfileAlreadyExists(name.String())
	}
	profile := models.NewProfile(name, nil)
	store.Profiles = append(store.Profiles, profile)
	if err := u.store.Save(ctx, store); err != nil {
		return models.Profile{}, err
	}
	return profile, nil
}
