package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/config"
	"iworkflow/internal/core/datahome"
	"iworkflow/internal/core/db"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// dataFolderWithDatabase creates <root>/old with an empty migrated database.
func dataFolderWithDatabase(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "old")
	handle, err := db.Open(context.Background(), filepath.Join(dir, "workspaces.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), handle))
	require.NoError(t, handle.Close())
	return dir
}

func moveEnv(root string, extra map[string]string) func(string) string {
	values := map[string]string{"HOME": root, "USERPROFILE": root, "XDG_CONFIG_HOME": filepath.Join(root, "cfg"), "APPDATA": filepath.Join(root, "cfg")}
	for name, value := range extra {
		values[name] = value
	}
	return func(name string) string { return values[name] }
}

func TestApplyPendingMoveSwitchesTheDataFolderBeforeTheServerReadsIt(t *testing.T) {
	root := t.TempDir()
	from := dataFolderWithDatabase(t, root)
	to := filepath.Join(root, "moved")
	getenv := moveEnv(root, map[string]string{"IWF_HOME": ""})
	// The default folder is <home>/.iworkflow; use it as the source.
	require.NoError(t, os.Rename(from, filepath.Join(root, ".iworkflow")))
	from = filepath.Join(root, ".iworkflow")
	mover, err := datahome.NewMover(runtime.GOOS, getenv, time.Now, func(string, ...any) {})
	require.NoError(t, err)
	require.NoError(t, mover.Request(from, to))

	require.NoError(t, applyPendingMove(context.Background(), getenv, quietLogger()))

	dir, err := config.DataDir(getenv)
	require.NoError(t, err)
	assert.Equal(t, to, dir, "the server resolves the new folder")
	assert.FileExists(t, filepath.Join(to, "workspaces.db"))
	assert.NoDirExists(t, from)
}

func TestApplyPendingMoveWithNothingPendingChangesNothing(t *testing.T) {
	root := t.TempDir()
	from := dataFolderWithDatabase(t, root)
	getenv := moveEnv(root, map[string]string{"IWF_HOME": from})

	require.NoError(t, applyPendingMove(context.Background(), getenv, quietLogger()))

	assert.FileExists(t, filepath.Join(from, "workspaces.db"))
}

func TestApplyPendingMoveSurvivesAMissingConfigFolder(t *testing.T) {
	getenv := func(string) string { return "" }

	require.NoError(t, applyPendingMove(context.Background(), getenv, quietLogger()))
}

func TestApplyPendingMoveReportsAnInterruptionAndKeepsTheMovePending(t *testing.T) {
	root := t.TempDir()
	from := dataFolderWithDatabase(t, root)
	getenv := moveEnv(root, map[string]string{"IWF_HOME": from})
	mover, err := datahome.NewMover(runtime.GOOS, getenv, time.Now, func(string, ...any) {})
	require.NoError(t, err)
	require.NoError(t, mover.Request(from, filepath.Join(root, "moved")))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, applyPendingMove(ctx, getenv, quietLogger()), context.Canceled)

	pending, err := mover.Pending()
	require.NoError(t, err)
	assert.NotNil(t, pending)
}
