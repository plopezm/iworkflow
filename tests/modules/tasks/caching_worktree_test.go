package tasks_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/git"
)

// countingWorktree is an inner worktree whose diff functions are scripted. It counts the calls, which is
// how the cache is observed: a hit does not call the inner worktree.
type countingWorktree struct {
	outbound.Worktree
	mu          sync.Mutex
	fingerprint func(calls int) (string, error)
	diffCalls   int
	rangeCalls  int
	diffResult  func(calls int) models.TaskDiff
	rangeResult func(head string) models.TaskDiff
	// onDiff runs inside Diff, before it returns: a test uses it to move the branch mid-computation.
	onDiff func()
	// fingerprintCalls counts calls of DiffFingerprint.
	fingerprintCalls int
}

func (w *countingWorktree) DiffFingerprint(context.Context, string, string, string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.fingerprintCalls++
	if w.fingerprint != nil {
		return w.fingerprint(w.fingerprintCalls)
	}
	return "a:b", nil
}

func (w *countingWorktree) Diff(context.Context, string, string, string) (models.TaskDiff, error) {
	w.mu.Lock()
	w.diffCalls++
	calls := w.diffCalls
	onDiff := w.onDiff
	w.mu.Unlock()
	if onDiff != nil {
		onDiff()
	}
	return w.diffResult(calls), nil
}

func (w *countingWorktree) DiffRange(_ context.Context, _ string, diffRange models.DiffRange) (models.TaskDiff, error) {
	w.mu.Lock()
	w.rangeCalls++
	w.mu.Unlock()
	return w.rangeResult(diffRange.HeadSHA), nil
}

// sampleDiff is a diff whose patch is the given text.
func sampleDiff(patch string) models.TaskDiff {
	return models.TaskDiff{Patch: patch}
}

// rangeOf is the range of the cache tests: the base is always "base".
func rangeOf(head string) models.DiffRange {
	return models.DiffRange{BaseSHA: "base", HeadSHA: head}
}

// Rust: caching_worktree::open_diff_hits_when_fingerprint_is_unchanged. The Go cache reads the fingerprint
// once more after computing a diff (to refuse storing a stale one), so a miss reads it twice.
func TestCachingDiffHitsWhenTheFingerprintIsUnchanged(t *testing.T) {
	inner := &countingWorktree{diffResult: func(int) models.TaskDiff { return sampleDiff("one") }}
	cache := git.NewCachingWorktreeWithCapacity(inner, 4)

	first, err := cache.Diff(context.Background(), "/r", "main", "pi/x")
	require.NoError(t, err)
	second, err := cache.Diff(context.Background(), "/r", "main", "pi/x")
	require.NoError(t, err)

	assert.Equal(t, sampleDiff("one"), first)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, inner.diffCalls, "the second call is a hit")
	assert.Equal(t, 3, inner.fingerprintCalls, "miss: before and after; hit: before")
}

// Rust: caching_worktree::open_diff_misses_when_fingerprint_changes
func TestCachingDiffMissesWhenTheFingerprintChanges(t *testing.T) {
	inner := &countingWorktree{
		fingerprint: func(calls int) (string, error) {
			if calls <= 2 {
				return "a:b", nil
			}
			return "a:c", nil
		},
		diffResult: func(calls int) models.TaskDiff {
			if calls == 1 {
				return sampleDiff("old")
			}
			return sampleDiff("new")
		},
	}
	cache := git.NewCachingWorktreeWithCapacity(inner, 4)

	old, err := cache.Diff(context.Background(), "/r", "main", "pi/x")
	require.NoError(t, err)
	fresh, err := cache.Diff(context.Background(), "/r", "main", "pi/x")
	require.NoError(t, err)

	assert.Equal(t, sampleDiff("old"), old)
	assert.Equal(t, sampleDiff("new"), fresh)
	assert.Equal(t, 2, inner.diffCalls)
}

// Rust: caching_worktree::fingerprint_error_is_propagated_without_diffing
func TestCachingFingerprintErrorIsPropagatedWithoutDiffing(t *testing.T) {
	inner := &countingWorktree{
		fingerprint: func(int) (string, error) { return "", werrors.Git("boom") },
	}
	cache := git.NewCachingWorktreeWithCapacity(inner, 4)

	_, err := cache.Diff(context.Background(), "/r", "main", "pi/x")

	assert.ErrorIs(t, err, werrors.Git(""))
	assert.Zero(t, inner.diffCalls)
}

// Go-only: a branch that moves while its diff is computed must not store the newer diff under the older
// fingerprint. Otherwise, when the branch returns to the old tip, the stale entry would be served.
func TestCachingDoesNotStoreADiffComputedAcrossAMove(t *testing.T) {
	// Reads: 1 before call 1 (a:b), 2 after call 1 (a:c: the branch moved), 3 and 4 around call 2 (a:c), 5 before call 3.
	fingerprints := []string{"a:b", "a:c", "a:c", "a:c", "a:b"}
	inner := &countingWorktree{
		fingerprint: func(calls int) (string, error) {
			return fingerprints[min(calls, len(fingerprints))-1], nil
		},
		diffResult: func(calls int) models.TaskDiff {
			return sampleDiff(map[int]string{1: "computed-during-move", 2: "current"}[calls])
		},
	}
	cache := git.NewCachingWorktreeWithCapacity(inner, 4)

	// Call 1: fingerprint a:b, then the diff is computed while the branch moves to a:c. The after read is
	// a:c, so the entry under a:b must not be stored.
	first, err := cache.Diff(context.Background(), "/r", "main", "pi/x")
	require.NoError(t, err)
	assert.Equal(t, "computed-during-move", first.Patch)

	// Call 2: fingerprint a:c, a miss, the diff is computed and stored under a:c.
	second, err := cache.Diff(context.Background(), "/r", "main", "pi/x")
	require.NoError(t, err)
	assert.Equal(t, "current", second.Patch)

	// Call 3: fingerprint a:b again. It must not be a hit on the mid-move diff; a fresh diff is computed.
	third, err := cache.Diff(context.Background(), "/r", "main", "pi/x")
	require.NoError(t, err)
	assert.Equal(t, 3, inner.diffCalls)
	assert.NotEqual(t, "computed-during-move", third.Patch)
}

// Rust: caching_worktree::range_diff_is_cached_by_range_without_fingerprint
func TestCachingRangeDiffIsCachedByRangeWithoutFingerprint(t *testing.T) {
	inner := &countingWorktree{rangeResult: func(string) models.TaskDiff { return sampleDiff("merged") }}
	cache := git.NewCachingWorktreeWithCapacity(inner, 4)

	first, err := cache.DiffRange(context.Background(), "/r", rangeOf("h1"))
	require.NoError(t, err)
	second, err := cache.DiffRange(context.Background(), "/r", rangeOf("h1"))
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Equal(t, 1, inner.rangeCalls)
	assert.Zero(t, inner.fingerprintCalls, "ranges never read the fingerprint")
}

// Rust: caching_worktree::different_ranges_do_not_collide
func TestCachingDifferentRangesDoNotCollide(t *testing.T) {
	inner := &countingWorktree{rangeResult: func(head string) models.TaskDiff { return sampleDiff(head) }}
	cache := git.NewCachingWorktreeWithCapacity(inner, 4)

	a, err := cache.DiffRange(context.Background(), "/r", rangeOf("h1"))
	require.NoError(t, err)
	b, err := cache.DiffRange(context.Background(), "/r", rangeOf("h2"))
	require.NoError(t, err)

	assert.Equal(t, sampleDiff("h1"), a)
	assert.Equal(t, sampleDiff("h2"), b)
}

// Rust: caching_worktree::evicts_least_recently_used_entry
func TestCachingEvictsTheLeastRecentlyUsedEntry(t *testing.T) {
	inner := &countingWorktree{rangeResult: func(head string) models.TaskDiff { return sampleDiff(head) }}
	cache := git.NewCachingWorktreeWithCapacity(inner, 2)
	ctx := context.Background()

	for _, head := range []string{"h1", "h2", "h3"} { // h3 evicts h1
		_, err := cache.DiffRange(ctx, "/r", rangeOf(head))
		require.NoError(t, err)
	}
	_, err := cache.DiffRange(ctx, "/r", rangeOf("h2")) // hit
	require.NoError(t, err)
	_, err = cache.DiffRange(ctx, "/r", rangeOf("h1")) // miss: it was evicted
	require.NoError(t, err)

	assert.Equal(t, 4, inner.rangeCalls, "h1 is computed twice, h2 once")
}

// Rust: caching_worktree::hit_refreshes_recency
func TestCachingHitRefreshesRecency(t *testing.T) {
	inner := &countingWorktree{rangeResult: func(head string) models.TaskDiff { return sampleDiff(head) }}
	cache := git.NewCachingWorktreeWithCapacity(inner, 2)
	ctx := context.Background()

	for _, head := range []string{"h1", "h2", "h1", "h3", "h1", "h2"} {
		_, err := cache.DiffRange(ctx, "/r", rangeOf(head))
		require.NoError(t, err)
	}

	// h1 is hit after h2 becomes the oldest; h3 evicts h2; the last h2 is computed again.
	assert.Equal(t, 4, inner.rangeCalls)
}

// Go-only: a caller that changes the commits it received does not change what the cache serves next.
func TestCachingReturnsCopies(t *testing.T) {
	inner := &countingWorktree{rangeResult: func(string) models.TaskDiff {
		return models.TaskDiff{Patch: "p", Commits: []string{"abc"}}
	}}
	cache := git.NewCachingWorktreeWithCapacity(inner, 4)
	ctx := context.Background()

	first, err := cache.DiffRange(ctx, "/r", rangeOf("h1"))
	require.NoError(t, err)
	first.Commits[0] = "changed"
	second, err := cache.DiffRange(ctx, "/r", rangeOf("h1"))

	require.NoError(t, err)
	assert.Equal(t, []string{"abc"}, second.Commits)
}

// Go-only: concurrent readers of the same range see the same diff, and the cache stays consistent under
// -race.
func TestCachingIsSafeForConcurrentReaders(t *testing.T) {
	inner := &countingWorktree{rangeResult: func(head string) models.TaskDiff { return sampleDiff(head) }}
	cache := git.NewCachingWorktreeWithCapacity(inner, 8)
	var wg sync.WaitGroup
	for index := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			head := "h" + string(rune('0'+index%4))
			diff, err := cache.DiffRange(context.Background(), "/r", rangeOf(head))
			assert.NoError(t, err)
			assert.Equal(t, head, diff.Patch)
		}()
	}
	wg.Wait()
}
