package usagelimits_test

import (
	"context"
	"errors"
	"sync"
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

// cancellingProvider answers a good reading but cancels the context while it does.
type cancellingProvider struct {
	scriptedProvider
	cancel context.CancelFunc
}

func (c *cancellingProvider) Fetch(ctx context.Context) (models.ProviderUsage, error) {
	usage, err := c.scriptedProvider.Fetch(ctx)
	c.cancel()
	return usage, err
}

func TestCacheKeepsAGoodReadingEvenIfTheContextIsCancelledAfterIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inner := &cancellingProvider{scriptedProvider: scriptedProvider{results: []result{{usage: reading(t, 42)}}}, cancel: cancel}
	provider := cache.NewProvider(inner, &movingClock{now: testNow}, time.Minute)

	usage, err := provider.Fetch(ctx)

	require.NoError(t, err)
	require.Len(t, usage.Windows(), 1)
	assert.InDelta(t, 42, usage.Windows()[0].UsedPct(), 0.001)
}

func TestCacheStopsServingStaleDataPastTheMaximumAge(t *testing.T) {
	inner := &scriptedProvider{results: []result{{usage: reading(t, 10)}, {err: usageerrors.ErrUnavailable}}}
	clock := &movingClock{now: testNow}
	provider := cache.NewProvider(inner, clock, time.Minute).WithMaxStale(10 * time.Minute)
	_, err := provider.Fetch(context.Background())
	require.NoError(t, err)

	clock.now = testNow.Add(9 * time.Minute)
	stale, err := provider.Fetch(context.Background())
	require.NoError(t, err)
	clock.now = testNow.Add(11 * time.Minute)
	_, err = provider.Fetch(context.Background())

	assert.True(t, stale.Stale())
	require.ErrorIs(t, err, usageerrors.ErrUnavailable)
}

// gatedProvider blocks every read until released and counts them.
type gatedProvider struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
	usage   models.ProviderUsage
}

func (*gatedProvider) ID() models.ProviderID { return models.ProviderClaude }

func (g *gatedProvider) Fetch(context.Context) (models.ProviderUsage, error) {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	g.started <- struct{}{}
	<-g.release
	return g.usage, nil
}

func TestCacheLetsOneCallerReadWhileOthersWaitAndHonourTheirContext(t *testing.T) {
	inner := &gatedProvider{started: make(chan struct{}, 4), release: make(chan struct{}), usage: reading(t, 5)}
	provider := cache.NewProvider(inner, &movingClock{now: testNow}, time.Minute)
	leaderDone := make(chan error, 1)
	go func() {
		_, err := provider.Fetch(context.Background())
		leaderDone <- err
	}()
	<-inner.started

	// A second caller waits behind the read in flight and gives up with its own context.
	waiterCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := provider.Fetch(waiterCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// A third caller waits too, and gets the leader's reading without a second read.
	followerDone := make(chan models.ProviderUsage, 1)
	go func() {
		usage, _ := provider.Fetch(context.Background())
		followerDone <- usage
	}()
	time.Sleep(20 * time.Millisecond)
	close(inner.release)

	require.NoError(t, <-leaderDone)
	follower := <-followerDone
	require.Len(t, follower.Windows(), 1)
	inner.mu.Lock()
	defer inner.mu.Unlock()
	assert.Equal(t, 1, inner.calls)
}
