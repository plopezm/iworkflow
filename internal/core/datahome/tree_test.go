package datahome_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/datahome"
)

func writeFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
	require.NoError(t, os.Chmod(path, mode))
}

func sampleTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "iwf")
	writeFile(t, filepath.Join(root, "workspaces.db"), "db", 0o600)
	writeFile(t, filepath.Join(root, "worktrees", "ws", "task", "main.go"), "package main", 0o644)
	writeFile(t, filepath.Join(root, "worktrees", "ws", "task", "run.sh"), "#!/bin/sh", 0o755)
	writeFile(t, filepath.Join(root, "attachments", "t", "a.png"), "png", 0o644)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "empty-dir"), 0o755))
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Symlink("main.go", filepath.Join(root, "worktrees", "ws", "task", "link")))
		require.NoError(t, os.Symlink("/nonexistent/absolute", filepath.Join(root, "worktrees", "ws", "task", "dangling")))
	}
	return root
}

func TestScanListsEverythingExceptSkipped(t *testing.T) {
	root := sampleTree(t)

	manifest, err := datahome.Scan(root, func(rel string) bool { return rel == "workspaces.db" })

	require.NoError(t, err)
	names := manifest.Names()
	assert.NotContains(t, names, "workspaces.db")
	assert.Contains(t, names, filepath.Join("worktrees", "ws", "task", "main.go"))
	assert.Contains(t, names, "empty-dir")
}

func TestCopyReproducesTheTreeAndVerifyAcceptsIt(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	staging := filepath.Join(t.TempDir(), "staging")

	require.NoError(t, datahome.Copy(context.Background(), root, staging, manifest))

	require.NoError(t, datahome.Verify(staging, manifest))
	content, err := os.ReadFile(filepath.Join(staging, "worktrees", "ws", "task", "main.go"))
	require.NoError(t, err)
	assert.Equal(t, "package main", string(content))
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(staging, "worktrees", "ws", "task", "run.sh"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "the executable bit survives")
		target, err := os.Readlink(filepath.Join(staging, "worktrees", "ws", "task", "dangling"))
		require.NoError(t, err)
		assert.Equal(t, "/nonexistent/absolute", target, "symlinks are copied as links, never followed")
	}
	_, err = os.Stat(filepath.Join(staging, "empty-dir"))
	assert.NoError(t, err, "empty folders are kept")
}

func TestCopyNeverTouchesTheSource(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)

	require.NoError(t, datahome.Copy(context.Background(), root, filepath.Join(t.TempDir(), "s"), manifest))

	require.NoError(t, datahome.Verify(root, manifest))
}

func TestVerifyDetectsMissingTruncatedAndRetargetedEntries(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	staging := filepath.Join(t.TempDir(), "staging")
	require.NoError(t, datahome.Copy(context.Background(), root, staging, manifest))

	require.NoError(t, os.WriteFile(filepath.Join(staging, "attachments", "t", "a.png"), []byte("p"), 0o644))
	require.ErrorIs(t, datahome.Verify(staging, manifest), datahome.ErrVerify, "size differs")
	require.NoError(t, os.WriteFile(filepath.Join(staging, "attachments", "t", "a.png"), []byte("png"), 0o644))
	require.NoError(t, datahome.Verify(staging, manifest))
	require.NoError(t, os.Remove(filepath.Join(staging, "attachments", "t", "a.png")))
	require.ErrorIs(t, datahome.Verify(staging, manifest), datahome.ErrVerify, "missing file")
}

func TestCopyRefusesToOverwriteAnExistingFile(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	staging := filepath.Join(t.TempDir(), "staging")
	writeFile(t, filepath.Join(staging, "attachments", "t", "a.png"), "other", 0o644)

	require.Error(t, datahome.Copy(context.Background(), root, staging, manifest))
}

func TestCopyStopsWhenTheContextIsCancelled(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, datahome.Copy(ctx, root, filepath.Join(t.TempDir(), "s"), manifest), context.Canceled)
}

func TestPruneRemovesExactlyTheManifestAndKeepsForeignFiles(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	writeFile(t, filepath.Join(root, "worktrees", "late.txt"), "created after the scan", 0o644)

	require.NoError(t, datahome.Prune(root, manifest))

	_, err = os.Stat(filepath.Join(root, "worktrees", "ws"))
	assert.ErrorIs(t, err, os.ErrNotExist, "the copied entries are gone")
	_, err = os.Stat(filepath.Join(root, "worktrees", "late.txt"))
	assert.NoError(t, err, "a file the scan never listed survives")
}

func TestPruneRemovesTheRootWhenNothingIsLeft(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)

	require.NoError(t, datahome.Prune(root, manifest))

	_, err = os.Stat(root)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestPruneDoesNotFollowSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	outside := filepath.Join(t.TempDir(), "outside")
	writeFile(t, filepath.Join(outside, "keep.txt"), "keep", 0o644)
	root := filepath.Join(t.TempDir(), "iwf")
	writeFile(t, filepath.Join(root, "workspaces.db"), "db", 0o600)
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link-to-outside")))
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)

	require.NoError(t, datahome.Prune(root, manifest))

	_, err = os.Stat(filepath.Join(outside, "keep.txt"))
	assert.NoError(t, err, "what a link points to is never deleted")
}

func TestCopyProgressReportsBytesAndEndsAtTheTotal(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	var reported [][2]int64

	err = datahome.CopyProgress(context.Background(), root, filepath.Join(t.TempDir(), "s"), manifest, 0, func(copied, total int64) {
		reported = append(reported, [2]int64{copied, total})
	})

	require.NoError(t, err)
	require.NotEmpty(t, reported)
	last := reported[len(reported)-1]
	assert.Equal(t, manifest.TotalSize(), last[0])
	assert.Equal(t, manifest.TotalSize(), last[1])
	for index := 1; index < len(reported); index++ {
		assert.GreaterOrEqual(t, reported[index][0], reported[index-1][0], "progress never goes back")
	}
}

func TestVerifyDetectsACopyWithTheSameSizeButOtherContent(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	staging := filepath.Join(t.TempDir(), "staging")
	require.NoError(t, datahome.Copy(context.Background(), root, staging, manifest))
	require.NoError(t, datahome.Verify(staging, manifest))

	writeFile(t, filepath.Join(staging, "attachments", "t", "a.png"), "pnX", 0o644) // same size as "png"

	require.ErrorIs(t, datahome.Verify(staging, manifest), datahome.ErrVerify)
}

func TestScanFollowsASymlinkedRootButNotTheLinksInside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	real := sampleTree(t)
	link := filepath.Join(t.TempDir(), "link-to-iwf")
	require.NoError(t, os.Symlink(real, link))

	manifest, err := datahome.Scan(link, nil)

	require.NoError(t, err)
	assert.Contains(t, manifest.Names(), "workspaces.db")
}

func TestUnhashedListsTheFilesWhoseContentVerifyCannotCheck(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, manifest.Unhashed(), "a scanned manifest has no checksums yet")

	require.NoError(t, datahome.Copy(context.Background(), root, filepath.Join(t.TempDir(), "s"), manifest))

	assert.Empty(t, manifest.Unhashed(), "Copy records a checksum of every regular file")
}

func TestPruneKeepsAFolderThatStillHoldsUnlistedFiles(t *testing.T) {
	root := sampleTree(t)
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	extra := filepath.Join(root, "attachments", "t", "late.txt")
	require.NoError(t, os.WriteFile(extra, []byte("late"), 0o644))

	require.NoError(t, datahome.Prune(root, manifest), "a non-empty folder is left alone, not reported as a failure")

	assert.FileExists(t, extra)
}
