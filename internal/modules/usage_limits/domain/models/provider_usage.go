package models

import (
	"fmt"
	"slices"
	"time"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
)

// ProviderID identifies a subscription provider.
type ProviderID string

// Known providers.
const (
	ProviderClaude ProviderID = "claude"
	ProviderCodex  ProviderID = "codex"
)

// ProviderUsage is what one provider reports at one moment. It is immutable.
type ProviderUsage struct {
	provider  ProviderID
	windows   []Window
	fetchedAt time.Time
	stale     bool
}

// NewProviderUsage validates the data and builds the report. It needs a provider and at least one window.
func NewProviderUsage(provider ProviderID, windows []Window, fetchedAt time.Time) (ProviderUsage, error) {
	if provider == "" {
		return ProviderUsage{}, fmt.Errorf("provider is empty: %w", usageerrors.ErrInvalidUsage)
	}
	if len(windows) == 0 {
		return ProviderUsage{}, fmt.Errorf("provider %q reports no window: %w", provider, usageerrors.ErrInvalidUsage)
	}
	return ProviderUsage{provider: provider, windows: slices.Clone(windows), fetchedAt: fetchedAt}, nil
}

// Provider is the provider identifier.
func (u ProviderUsage) Provider() ProviderID { return u.provider }

// Windows returns a copy of the windows.
func (u ProviderUsage) Windows() []Window { return slices.Clone(u.windows) }

// FetchedAt is when the data was read from the provider.
func (u ProviderUsage) FetchedAt() time.Time { return u.fetchedAt }

// Stale reports that the data is the last good reading, served because the provider failed now.
func (u ProviderUsage) Stale() bool { return u.stale }

// AsStale returns a copy marked as stale.
func (u ProviderUsage) AsStale() ProviderUsage {
	u.stale = true
	return u
}

// Report is the outcome of asking every provider.
type Report struct {
	// Usages are the providers that answered, in provider order.
	Usages []ProviderUsage
	// Unavailable are the configured providers that failed and have no earlier reading.
	Unavailable []ProviderID
}
