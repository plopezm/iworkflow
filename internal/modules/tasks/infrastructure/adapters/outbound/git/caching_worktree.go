//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"context"
	"sync"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// DiffCacheCapacity is the number of diffs kept in memory.
const DiffCacheCapacity = 32

// CachingWorktree is a worktree decorator that avoids recomputing diffs that did not change.
//
//   - DiffRange: a range (base, head) is immutable, so the range itself is the key.
//   - Diff: the key includes the fingerprint (the tips of both branches). The fingerprint is read on every
//     call, so a moved branch misses the cache. The diff is stored only when the fingerprint is the same
//     before and after it was computed: a branch that moves in between would otherwise store the newer
//     diff under the older key, and that diff would be served again if the branch returned to that tip.
//   - DiffLive is never cached: it depends on uncommitted files, which no cheap fingerprint covers.
//
// Every other method is delegated unchanged.
type CachingWorktree struct {
	inner outbound.Worktree
	mu    sync.Mutex
	// entries is least recently used first; the most recent entry is last.
	entries  []cacheEntry
	capacity int
}

type cacheEntry struct {
	key  string
	diff models.TaskDiff
}

var _ outbound.Worktree = (*CachingWorktree)(nil)

// NewCachingWorktree decorates inner with the default capacity.
func NewCachingWorktree(inner outbound.Worktree) *CachingWorktree {
	return NewCachingWorktreeWithCapacity(inner, DiffCacheCapacity)
}

// NewCachingWorktreeWithCapacity decorates inner with a cache of the given size (at least 1).
func NewCachingWorktreeWithCapacity(inner outbound.Worktree, capacity int) *CachingWorktree {
	return &CachingWorktree{inner: inner, capacity: max(capacity, 1)}
}

// Diff returns the cached diff while the fingerprint of the branches is unchanged.
func (c *CachingWorktree) Diff(ctx context.Context, repo string, baseBranch string, branch string) (models.TaskDiff, error) {
	before, err := c.inner.DiffFingerprint(ctx, repo, baseBranch, branch)
	if err != nil {
		return models.TaskDiff{}, err
	}
	key := "diff|" + repo + "|" + before
	if hit, ok := c.lookup(key); ok {
		return hit, nil
	}
	diff, err := c.inner.Diff(ctx, repo, baseBranch, branch)
	if err != nil {
		return models.TaskDiff{}, err
	}
	after, err := c.inner.DiffFingerprint(ctx, repo, baseBranch, branch)
	if err == nil && after == before {
		c.store(key, diff)
	}
	return diff, nil
}

// DiffRange returns the cached diff of a range.
func (c *CachingWorktree) DiffRange(ctx context.Context, repo string, diffRange models.DiffRange) (models.TaskDiff, error) {
	key := "range|" + repo + "|" + diffRange.BaseSHA + ".." + diffRange.HeadSHA
	if hit, ok := c.lookup(key); ok {
		return hit, nil
	}
	diff, err := c.inner.DiffRange(ctx, repo, diffRange)
	if err != nil {
		return models.TaskDiff{}, err
	}
	c.store(key, diff)
	return diff, nil
}

// lookup returns a copy of the cached diff and marks it as the most recently used.
func (c *CachingWorktree) lookup(key string) (models.TaskDiff, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for index, entry := range c.entries {
		if entry.key == key {
			c.entries = append(append(c.entries[:index:index], c.entries[index+1:]...), entry)
			return copyDiff(entry.diff), true
		}
	}
	return models.TaskDiff{}, false
}

// store adds a copy of the diff, replacing an entry with the same key and evicting the least recently
// used entry when the cache is full.
func (c *CachingWorktree) store(key string, diff models.TaskDiff) {
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.entries[:0]
	for _, entry := range c.entries {
		if entry.key != key {
			kept = append(kept, entry)
		}
	}
	c.entries = kept
	if len(c.entries) >= c.capacity {
		c.entries = c.entries[1:]
	}
	c.entries = append(c.entries, cacheEntry{key: key, diff: copyDiff(diff)})
}

// copyDiff clones the commit list, so a caller cannot change the cached value through its result.
func copyDiff(diff models.TaskDiff) models.TaskDiff {
	diff.Commits = append([]string(nil), diff.Commits...)
	return diff
}

// The remaining methods are not cached.

// WorktreePath delegates to the inner worktree.
func (c *CachingWorktree) WorktreePath(workspaceName string, dirName string) string {
	return c.inner.WorktreePath(workspaceName, dirName)
}

// ListBranches delegates to the inner worktree.
func (c *CachingWorktree) ListBranches(ctx context.Context, repo string) ([]models.BranchOption, error) {
	return c.inner.ListBranches(ctx, repo)
}

// Create delegates to the inner worktree.
func (c *CachingWorktree) Create(ctx context.Context, repo string, baseBranch string, branch string, path string) error {
	return c.inner.Create(ctx, repo, baseBranch, branch, path)
}

// CommitAll delegates to the inner worktree.
func (c *CachingWorktree) CommitAll(ctx context.Context, path string, message string) (bool, error) {
	return c.inner.CommitAll(ctx, path, message)
}

// DiffLive delegates to the inner worktree: uncommitted changes are not cached.
func (c *CachingWorktree) DiffLive(ctx context.Context, worktree string, baseBranch string) (models.TaskDiff, error) {
	return c.inner.DiffLive(ctx, worktree, baseBranch)
}

// DiffCommitted delegates to the inner worktree: the tip of the worktree moves with every commit of the agent.
func (c *CachingWorktree) DiffCommitted(ctx context.Context, worktree string, baseBranch string) (models.TaskDiff, error) {
	return c.inner.DiffCommitted(ctx, worktree, baseBranch)
}

// DiffFingerprint delegates to the inner worktree.
func (c *CachingWorktree) DiffFingerprint(ctx context.Context, repo string, baseBranch string, branch string) (string, error) {
	return c.inner.DiffFingerprint(ctx, repo, baseBranch, branch)
}

// PinRange delegates to the inner worktree.
func (c *CachingWorktree) PinRange(ctx context.Context, repo string, baseBranch string, branch string, pinRef string) (models.DiffRange, error) {
	return c.inner.PinRange(ctx, repo, baseBranch, branch, pinRef)
}

// Merge delegates to the inner worktree.
func (c *CachingWorktree) Merge(ctx context.Context, repo string, baseBranch string, branch string, message string) (models.MergeOutcome, error) {
	return c.inner.Merge(ctx, repo, baseBranch, branch, message)
}

// Exists delegates to the inner worktree.
func (c *CachingWorktree) Exists(ctx context.Context, path string) bool {
	return c.inner.Exists(ctx, path)
}

// Remove delegates to the inner worktree.
func (c *CachingWorktree) Remove(ctx context.Context, repo string, path string, branch string) error {
	return c.inner.Remove(ctx, repo, path, branch)
}

// SweepOrphans delegates to the inner worktree.
func (c *CachingWorktree) SweepOrphans(ctx context.Context, keep []string) (int, error) {
	return c.inner.SweepOrphans(ctx, keep)
}

// WaitForCleanup delegates to the inner worktree.
func (c *CachingWorktree) WaitForCleanup(ctx context.Context) {
	c.inner.WaitForCleanup(ctx)
}
