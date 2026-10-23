package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSplitsNulSeparatedPaths(t *testing.T) {
	require.Equal(t, []string{"src/a.rs", "README.md"}, parseFiles("src/a.rs\x00README.md\x00"))
	require.Empty(t, parseFiles(""))
}

func TestDeletedPathsAreDroppedFromTheListing(t *testing.T) {
	require.Equal(t, []string{"b.rs"}, withoutDeleted([]string{"a.rs", "b.rs"}, []string{"a.rs"}))
}

func TestAFreshListingIsReusedAndAnOldOneIsNot(t *testing.T) {
	const root string = "/work/a"
	start := time.Now()
	cache := newListingCache(3 * time.Second)
	_, found := cache.get(root, start)
	require.False(t, found)

	cache.put(root, []string{"a.rs"}, start)

	files, found := cache.get(root, start.Add(2*time.Second))
	require.True(t, found)
	require.Equal(t, []string{"a.rs"}, files)
	_, found = cache.get(root, start.Add(3*time.Second))
	require.False(t, found)
}

func TestRootsDoNotShareListingsAndExpiredOnesAreDroppedOnPut(t *testing.T) {
	start := time.Now()
	cache := newListingCache(3 * time.Second)
	cache.put("/work/a", []string{"a.rs"}, start)

	later := start.Add(10 * time.Second)
	cache.put("/work/b", []string{"b.rs"}, later)

	_, found := cache.get("/work/a", later)
	require.False(t, found)
	require.Len(t, cache.entries, 1)
	files, found := cache.get("/work/b", later)
	require.True(t, found)
	require.Equal(t, []string{"b.rs"}, files)
}

func TestCachedListingIsACopy(t *testing.T) {
	now := time.Now()
	cache := newListingCache(time.Minute)
	original := []string{"a.rs"}
	cache.put("/work/a", original, now)
	original[0] = "changed.rs"

	files, _ := cache.get("/work/a", now)
	files[0] = "mutated.rs"

	again, _ := cache.get("/work/a", now)
	require.Equal(t, []string{"a.rs"}, again)
}

// runTestGit runs git in dir and fails the test on error.
func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, output)
}

func TestASecondCallWithinTheTTLReusesTheListing(t *testing.T) {
	dir := t.TempDir()
	runTestGit(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.rs"), nil, 0o600))
	adapter := NewFileSearchAdapter()

	first, err := adapter.Files(context.Background(), dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.rs"), nil, 0o600))
	second, err := adapter.Files(context.Background(), dir)
	require.NoError(t, err)

	require.Equal(t, []string{"a.rs"}, first)
	require.Equal(t, first, second, "the new file is hidden until the TTL expires")
}

func TestAnEmptyListingIsNotCached(t *testing.T) {
	dir := t.TempDir()
	adapter := NewFileSearchAdapter()

	files, err := adapter.Files(context.Background(), dir)
	require.NoError(t, err)
	require.Empty(t, files)

	runTestGit(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.rs"), nil, 0o600))

	files, err = adapter.Files(context.Background(), dir)
	require.NoError(t, err)
	require.Equal(t, []string{"a.rs"}, files)
}
