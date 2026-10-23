package git

import (
	"context"
	"slices"
	"sync"
	"time"

	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// listingTTL is how long a listing is reused. Autocomplete asks once per keystroke; a few seconds
// absorb the burst while new files still show up almost immediately.
const listingTTL time.Duration = 3 * time.Second

// cachedListing is the last listing of one project root and when it was stored.
type cachedListing struct {
	storedAt time.Time
	files    []string
}

// listingCache remembers the last listing per project root for ttl. Time is passed in, so the
// expiry rules are testable without sleeping.
type listingCache struct {
	ttl     time.Duration
	entries map[string]cachedListing
}

func newListingCache(ttl time.Duration) *listingCache {
	return &listingCache{ttl: ttl, entries: make(map[string]cachedListing)}
}

// get returns the cached listing of root when it is younger than the TTL.
func (c *listingCache) get(root string, now time.Time) ([]string, bool) {
	entry, ok := c.entries[root]
	if !ok || now.Sub(entry.storedAt) >= c.ttl {
		return nil, false
	}
	return slices.Clone(entry.files), true
}

// put stores a listing and drops the expired ones, so the cache cannot grow without bound.
func (c *listingCache) put(root string, files []string, now time.Time) {
	for key, entry := range c.entries {
		if now.Sub(entry.storedAt) >= c.ttl {
			delete(c.entries, key)
		}
	}
	c.entries[root] = cachedListing{storedAt: now, files: slices.Clone(files)}
}

// FileSearchAdapter lists project files with `git ls-files`, so ignored files and `.git` never
// show up. Listings are reused for listingTTL.
type FileSearchAdapter struct {
	mu    sync.Mutex
	cache *listingCache
	now   func() time.Time
}

var _ outbound.FileSearchPort = (*FileSearchAdapter)(nil)

// NewFileSearchAdapter builds the adapter with an empty cache.
func NewFileSearchAdapter() *FileSearchAdapter {
	return &FileSearchAdapter{cache: newListingCache(listingTTL), now: time.Now}
}

// Files returns the files of root that git does not ignore. It is empty when root is not a git
// repository.
func (a *FileSearchAdapter) Files(ctx context.Context, root string) ([]string, error) {
	if files, ok := a.cached(root); ok {
		return files, nil
	}
	files, err := listFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	// An empty listing is what a failed or not-yet-initialised repository looks like: it is cheap to
	// recompute and must not hide the files that appear right after.
	if len(files) > 0 {
		a.remember(root, files)
	}
	return files, nil
}

// cached reads the cache under the lock.
func (a *FileSearchAdapter) cached(root string) ([]string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cache.get(root, a.now())
}

// remember stores a listing under the lock.
func (a *FileSearchAdapter) remember(root string, files []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache.put(root, files, a.now())
}

// listFiles runs git to list the files of root; empty when it is not a git repository.
func listFiles(ctx context.Context, root string) ([]string, error) {
	listed, err := runGit(ctx, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	if !listed.Success {
		return []string{}, nil
	}
	// --cached also lists tracked files that were deleted from the worktree.
	deleted, err := runGit(ctx, root, "ls-files", "-z", "--deleted")
	if err != nil {
		return nil, err
	}
	var gone []string
	if deleted.Success {
		gone = parseFiles(deleted.Stdout)
	}
	return withoutDeleted(parseFiles(listed.Stdout), gone), nil
}

// parseFiles splits `git ls-files -z` output into paths (pure).
func parseFiles(output string) []string {
	return nonEmptyFields(output)
}

// withoutDeleted drops the listed paths that were deleted from the worktree (pure).
func withoutDeleted(listed []string, deleted []string) []string {
	gone := make(map[string]struct{}, len(deleted))
	for _, path := range deleted {
		gone[path] = struct{}{}
	}
	kept := make([]string, 0, len(listed))
	for _, path := range listed {
		if _, removed := gone[path]; !removed {
			kept = append(kept, path)
		}
	}
	return kept
}
