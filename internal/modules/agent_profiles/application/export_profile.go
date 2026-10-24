//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// ExportProfile serializes an existing profile to the exchange format.
type ExportProfile struct {
	store    outbound.ProfileStorePort
	exchange outbound.ProfileExchangePort
}

// NewExportProfile builds the use case.
func NewExportProfile(store outbound.ProfileStorePort, exchange outbound.ProfileExchangePort) *ExportProfile {
	return &ExportProfile{store: store, exchange: exchange}
}

// Execute returns the exchange document of the named profile.
func (u *ExportProfile) Execute(ctx context.Context, name models.ProfileName) (string, error) {
	store, err := u.store.Load(ctx)
	if err != nil {
		return "", err
	}
	profile := store.Find(name)
	if profile == nil {
		return "", agenterrors.ProfileNotFound(name.String())
	}
	return u.exchange.SerializeProfile(ctx, *profile)
}
