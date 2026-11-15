package usagelimits_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/infrastructure/adapters/outbound/cache"
)

// scriptedProvider answers the queued results one per call and counts the calls.
type scriptedProvider struct {
	results []result
	calls   int
}

type result struct {
	usage models.ProviderUsage
	err   error
}

func (*scriptedProvider) ID() models.ProviderID { return models.ProviderClaude }

func (s *scriptedProvider) Fetch(context.Context) (models.ProviderUsage, error) {
	next := s.results[min(s.calls, len(s.results)-1)]
	s.calls++
	return next.usage, next.err
}

// movingClock is a clock the test advances.
type movingClock struct{ now time.Time }

func (c *movingClock) Now() time.Time { return c.now }

func reading(t *testing.T, pct float64) models.ProviderUsage {
	t.Helper()
	window, err := models.NewWindow(models.WindowFiveHour, pct, time.Time{})
	require.NoError(t, err)
	usage, err := models.NewProviderUsage(models.ProviderClaude, []models.Window{window}, testNow)
	require.NoError(t, err)
	return usage
}

func TestCacheReusesAReadingWithinTheTTLAndRefreshesAfterIt(t *testing.T) {
	inner := &scriptedProvider{results: []result{{usage: reading(t, 10)}, {usage: reading(t, 20)}}}
	clock := &movingClock{now: testNow}
	provider := cache.NewProvider(inner, clock, time.Minute)

	first, err := provider.Fetch(context.Background())
	require.NoError(t, err)
	clock.now = testNow.Add(59 * time.Second)
	again, err := provider.Fetch(context.Background())
	require.NoError(t, err)
	clock.now = testNow.Add(time.Minute)
	refreshed, err := provider.Fetch(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, inner.calls)
	assert.InDelta(t, 10, first.Windows()[0].UsedPct(), 0.001)
	assert.InDelta(t, 10, again.Windows()[0].UsedPct(), 0.001)
	assert.InDelta(t, 20, refreshed.Windows()[0].UsedPct(), 0.001)
	assert.False(t, refreshed.Stale())
}

func TestCacheServesTheLastGoodReadingAsStaleWhenTheProviderFails(t *testing.T) {
	inner := &scriptedProvider{results: []result{{usage: reading(t, 10)}, {err: usageerrors.ErrUnavailable}}}
	clock := &movingClock{now: testNow}
	provider := cache.NewProvider(inner, clock, time.Minute)
	_, err := provider.Fetch(context.Background())
	require.NoError(t, err)

	clock.now = testNow.Add(2 * time.Minute)
	stale, err := provider.Fetch(context.Background())
	require.NoError(t, err)
	// The failure is remembered for the TTL: no new call.
	clock.now = testNow.Add(2*time.Minute + 30*time.Second)
	_, err = provider.Fetch(context.Background())
	require.NoError(t, err)

	assert.True(t, stale.Stale())
	assert.InDelta(t, 10, stale.Windows()[0].UsedPct(), 0.001)
	assert.Equal(t, 2, inner.calls)
}

func TestCacheReportsTheErrorWhenThereIsNothingToServe(t *testing.T) {
	inner := &scriptedProvider{results: []result{{err: usageerrors.ErrUnavailable}}}
	provider := cache.NewProvider(inner, &movingClock{now: testNow}, time.Minute)

	_, err := provider.Fetch(context.Background())

	require.ErrorIs(t, err, usageerrors.ErrUnavailable)
}

func TestCacheDropsTheReadingWhenTheProviderStopsBeingConfigured(t *testing.T) {
	inner := &scriptedProvider{results: []result{{usage: reading(t, 10)}, {err: usageerrors.ErrNotConfigured}}}
	clock := &movingClock{now: testNow}
	provider := cache.NewProvider(inner, clock, time.Minute)
	_, err := provider.Fetch(context.Background())
	require.NoError(t, err)

	clock.now = testNow.Add(2 * time.Minute)
	_, err = provider.Fetch(context.Background())

	require.ErrorIs(t, err, usageerrors.ErrNotConfigured)
}

func TestCacheDoesNotRememberACancelledCall(t *testing.T) {
	inner := &scriptedProvider{results: []result{{err: errors.New("cancelled")}, {usage: reading(t, 7)}}}
	provider := cache.NewProvider(inner, &movingClock{now: testNow}, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := provider.Fetch(ctx)
	require.Error(t, err)

	usage, err := provider.Fetch(context.Background())

	require.NoError(t, err)
	assert.InDelta(t, 7, usage.Windows()[0].UsedPct(), 0.001)
	assert.Equal(t, 2, inner.calls)
}

func TestCacheExposesTheProviderID(t *testing.T) {
	provider := cache.NewProvider(&scriptedProvider{}, &movingClock{}, 0)

	assert.Equal(t, models.ProviderClaude, provider.ID())
}
