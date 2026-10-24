//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// SetProfileRouting replaces the routing (model and thinking level per agent) of a profile.
type SetProfileRouting struct {
	store outbound.ProfileStorePort
	guard *WriteGuard
}

// NewSetProfileRouting builds the use case. guard is shared with every use case that mutates profiles.json.
func NewSetProfileRouting(store outbound.ProfileStorePort, guard *WriteGuard) *SetProfileRouting {
	return &SetProfileRouting{store: store, guard: guard}
}

// Execute validates the routing, stores it under name and returns the updated profile.
func (u *SetProfileRouting) Execute(ctx context.Context, name models.ProfileName, routing map[string]models.RoutingEntry) (models.Profile, error) {
	if err := models.ValidateRouting(routing); err != nil {
		return models.Profile{}, err
	}
	if err := u.guard.lock(ctx); err != nil {
		return models.Profile{}, err
	}
	defer u.guard.unlock()
	store, err := u.store.Load(ctx)
	if err != nil {
		return models.Profile{}, err
	}
	profile := store.Find(name)
	if profile == nil {
		return models.Profile{}, agenterrors.ProfileNotFound(name.String())
	}
	profile.Routing = models.CloneRouting(routing)
	updated := *profile
	if err := u.store.Save(ctx, store); err != nil {
		return models.Profile{}, err
	}
	return updated, nil
}
