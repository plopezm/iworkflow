// Package outbound holds the ports of the usage_limits module towards the infrastructure.
package outbound

import (
	"context"
	"time"

	"iworkflow/internal/modules/usage_limits/domain/models"
)

// UsageProviderPort reads the subscription usage of one provider. Each adapter is a facade: it hides where the
// data comes from (an HTTP endpoint, session files) behind this contract.
type UsageProviderPort interface {
	// ID is the provider this adapter reads.
	ID() models.ProviderID
	// Fetch returns the current usage. It returns errors.ErrNotConfigured when the provider has no credentials
	// or data on this machine, and errors.ErrUnavailable when it could not be read now.
	Fetch(ctx context.Context) (models.ProviderUsage, error)
}

// ClockPort gives the current time.
type ClockPort interface {
	Now() time.Time
}
