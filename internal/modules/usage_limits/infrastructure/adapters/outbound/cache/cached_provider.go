// Package cache holds the decorator that keeps provider reads from hitting a provider too often.
package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
)

const (
	// DefaultTTL is how long a reading, or a failure, is reused.
	DefaultTTL = 60 * time.Second
	// DefaultMaxStale is how long the last good reading may still be served, marked stale, while the provider
	// keeps failing. Past it the provider reads as unavailable instead of showing old numbers.
	DefaultMaxStale = 30 * time.Minute
)

// call is one provider read in flight; followers wait on done.
type call struct{ done chan struct{} }

// Provider decorates a UsageProviderPort with a time-to-live cache. A failing read is also remembered for the
// TTL so a broken provider is not hammered, and the last good reading is served marked as stale for at most the
// maximum stale age. The lock is never held while the provider is read: one caller reads, the others wait for it
// or give up when their own context ends.
type Provider struct {
	inner    outbound.UsageProviderPort
	clock    outbound.ClockPort
	ttl      time.Duration
	maxStale time.Duration

	mu        sync.Mutex
	inflight  *call
	checkedAt time.Time
	goodAt    time.Time
	last      *models.ProviderUsage
	lastErr   error
}

var _ outbound.UsageProviderPort = (*Provider)(nil)

// NewProvider wraps inner. A non-positive ttl means DefaultTTL.
func NewProvider(inner outbound.UsageProviderPort, clock outbound.ClockPort, ttl time.Duration) *Provider {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Provider{inner: inner, clock: clock, ttl: ttl, maxStale: DefaultMaxStale}
}

// WithMaxStale sets how long the last good reading may be served stale; a non-positive value keeps the default.
func (p *Provider) WithMaxStale(maxStale time.Duration) *Provider {
	if maxStale > 0 {
		p.maxStale = maxStale
	}
	return p
}

// ID is the provider of the decorated adapter.
func (p *Provider) ID() models.ProviderID { return p.inner.ID() }

// Fetch returns the cached outcome while it is younger than the TTL; otherwise one caller asks the provider while
// the rest wait. A cancelled read is not remembered, so the next caller asks again.
func (p *Provider) Fetch(ctx context.Context) (models.ProviderUsage, error) {
	for {
		p.mu.Lock()
		now := p.clock.Now()
		if !p.checkedAt.IsZero() && now.Sub(p.checkedAt) < p.ttl {
			usage, err := p.outcome(now)
			p.mu.Unlock()
			return usage, err
		}
		if running := p.inflight; running != nil {
			p.mu.Unlock()
			select {
			case <-running.done:
				continue
			case <-ctx.Done():
				return models.ProviderUsage{}, fmt.Errorf("wait for the usage read: %w", ctx.Err())
			}
		}
		leader := &call{done: make(chan struct{})}
		p.inflight = leader
		p.mu.Unlock()
		return p.lead(ctx, leader)
	}
}

// lead reads the provider without the lock and stores the outcome.
func (p *Provider) lead(ctx context.Context, leader *call) (models.ProviderUsage, error) {
	usage, err := p.inner.Fetch(ctx)

	p.mu.Lock()
	defer p.mu.Unlock()
	defer close(leader.done)
	p.inflight = nil
	if err != nil && ctx.Err() != nil {
		return models.ProviderUsage{}, err //nolint:wrapcheck // the decorated adapter already wraps it
	}
	now := p.clock.Now()
	p.checkedAt = now
	switch {
	case err == nil:
		p.last, p.lastErr, p.goodAt = &usage, nil, now
	case errors.Is(err, usageerrors.ErrNotConfigured):
		p.last, p.lastErr = nil, err
	default:
		p.lastErr = err
	}
	return p.outcome(now)
}

// outcome is the answer for the current state: fresh data, the last good data as stale while it is not too old,
// or the error.
func (p *Provider) outcome(now time.Time) (models.ProviderUsage, error) {
	if p.lastErr == nil && p.last != nil {
		return *p.last, nil
	}
	if p.last != nil && !errors.Is(p.lastErr, usageerrors.ErrNotConfigured) && now.Sub(p.goodAt) <= p.maxStale {
		return p.last.AsStale(), nil
	}
	return models.ProviderUsage{}, p.lastErr //nolint:wrapcheck // the decorated adapter already wraps it
}
