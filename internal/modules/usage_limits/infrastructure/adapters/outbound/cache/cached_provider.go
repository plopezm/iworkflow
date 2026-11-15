// Package cache holds the decorator that keeps provider reads from hitting a provider too often.
package cache

import (
	"context"
	"errors"
	"sync"
	"time"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
)

// DefaultTTL is how long a reading, or a failure, is reused.
const DefaultTTL = 60 * time.Second

// Provider decorates a UsageProviderPort with a time-to-live cache. A failing read is also remembered for the
// TTL so a broken provider is not hammered, and the last good reading is served marked as stale meanwhile.
type Provider struct {
	inner outbound.UsageProviderPort
	clock outbound.ClockPort
	ttl   time.Duration

	mu        sync.Mutex
	checkedAt time.Time
	last      *models.ProviderUsage
	lastErr   error
}

var _ outbound.UsageProviderPort = (*Provider)(nil)

// NewProvider wraps inner. A non-positive ttl means DefaultTTL.
func NewProvider(inner outbound.UsageProviderPort, clock outbound.ClockPort, ttl time.Duration) *Provider {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Provider{inner: inner, clock: clock, ttl: ttl}
}

// ID is the provider of the decorated adapter.
func (p *Provider) ID() models.ProviderID { return p.inner.ID() }

// Fetch returns the cached outcome while it is younger than the TTL; otherwise it asks the provider. A cancelled
// call is not remembered, so the next caller asks again.
func (p *Provider) Fetch(ctx context.Context) (models.ProviderUsage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.clock.Now()
	if !p.checkedAt.IsZero() && now.Sub(p.checkedAt) < p.ttl {
		return p.outcome()
	}
	usage, err := p.inner.Fetch(ctx)
	if ctx.Err() != nil {
		return models.ProviderUsage{}, err //nolint:wrapcheck // the decorated adapter already wraps it
	}
	p.checkedAt = now
	switch {
	case err == nil:
		p.last, p.lastErr = &usage, nil
	case errors.Is(err, usageerrors.ErrNotConfigured):
		p.last, p.lastErr = nil, err
	default:
		p.lastErr = err
	}
	return p.outcome()
}

// outcome is the answer for the current state: fresh data, the last good data as stale, or the error.
func (p *Provider) outcome() (models.ProviderUsage, error) {
	if p.lastErr == nil && p.last != nil {
		return *p.last, nil
	}
	if p.last != nil && !errors.Is(p.lastErr, usageerrors.ErrNotConfigured) {
		return p.last.AsStale(), nil
	}
	return models.ProviderUsage{}, p.lastErr //nolint:wrapcheck // the decorated adapter already wraps it
}
