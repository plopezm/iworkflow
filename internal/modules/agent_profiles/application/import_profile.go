//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// ImportProfile adds a profile from an exchange document. A name collision is refused unless the
// caller supplies another name.
type ImportProfile struct {
	store    outbound.ProfileStorePort
	exchange outbound.ProfileExchangePort
	guard    *WriteGuard
}

// NewImportProfile builds the use case. guard is shared with every use case that mutates profiles.json.
func NewImportProfile(store outbound.ProfileStorePort, exchange outbound.ProfileExchangePort, guard *WriteGuard) *ImportProfile {
	return &ImportProfile{store: store, exchange: exchange, guard: guard}
}

// Execute parses raw, optionally renames the profile to renameTo, and stores it.
func (u *ImportProfile) Execute(ctx context.Context, raw string, renameTo *models.ProfileName) (models.Profile, error) {
	profile, err := u.exchange.ParseProfile(ctx, raw)
	if err != nil {
		return models.Profile{}, err
	}
	if renameTo != nil {
		profile.Name = *renameTo
	}
	if err := u.guard.lock(ctx); err != nil {
		return models.Profile{}, err
	}
	defer u.guard.unlock()
	store, err := u.store.Load(ctx)
	if err != nil {
		return models.Profile{}, err
	}
	if store.Contains(profile.Name) {
		return models.Profile{}, agenterrors.ProfileAlreadyExists(profile.Name.String())
	}
	store.Profiles = append(store.Profiles, profile)
	if err := u.store.Save(ctx, store); err != nil {
		return models.Profile{}, err
	}
	return profile, nil
}
