package task_hooks_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/system"
	"iworkflow/internal/modules/task_hooks/infrastructure/di"
)

// workspaceAt is the workspace lookup of the test: every workspace lives in the directory.
type workspaceAt string

func (w workspaceAt) Path(context.Context, string) (string, error) { return string(w), nil }

// testModule seeds one workspace in dir and wires the module over it, with the database to inspect afterwards.
func testModule(t *testing.T, dir string) (*di.Module, *sql.DB) {
	t.Helper()
	db := dbtest.New(t)
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1', 'one', ?, 'main', '2026-01-01T00:00:00Z')`, dir)
	require.NoError(t, err)
	return di.NewModule(di.Deps{DB: db, Clock: system.NewClock(), Workspaces: workspaceAt(dir)}), db
}

func saveScript(t *testing.T, module *di.Module, enabled bool, args ...string) models.Hook {
	t.Helper()
	saved, err := module.SaveHook.Execute(context.Background(), application.SaveHookInput{
		WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindScript,
		Enabled: enabled, OnFailure: models.FailureBlock, Timeout: 30 * time.Second,
		Script: models.ScriptSpec{Command: "sh", Args: args},
	})
	require.NoError(t, err)
	return saved
}

func countRuns(t *testing.T, db *sql.DB) int {
	t.Helper()
	var runs int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM task_hook_runs`).Scan(&runs))
	return runs
}

// A test runs the hook in the workspace directory with the synthetic task, even when the hook is disabled, and writes
// no run record: the run history of the tasks is not touched by trying a hook.
func TestTestHookRunsInTheWorkspaceAndRecordsNothing(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("the script uses sh")
	}
	dir := t.TempDir()
	module, db := testModule(t, dir)
	hook := saveScript(t, module, false, "-c", `pwd > cwd.txt; printf %s "$IWF_TASK_TITLE" > title.txt`)

	outcome, err := module.TestHook.Execute(context.Background(), application.TestHookInput{WorkspaceID: "w1", HookID: hook.ID()})

	require.NoError(t, err)
	assert.Equal(t, models.RunOK, outcome.Status)
	wantDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	gotDir, err := filepath.EvalSymlinks(readFile(t, dir, "cwd.txt"))
	require.NoError(t, err)
	assert.Equal(t, wantDir, gotDir)
	assert.Equal(t, "test", readFile(t, dir, "title.txt"))
	assert.Zero(t, countRuns(t, db), "a test writes no run record")
}

// A failing script is an outcome the test shows, with its exit code, and it is still not recorded.
func TestTestHookReportsAFailingScriptAsAnOutcome(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("the script uses sh")
	}
	module, db := testModule(t, t.TempDir())
	hook := saveScript(t, module, true, "-c", "echo broken; exit 3")

	outcome, err := module.TestHook.Execute(context.Background(), application.TestHookInput{WorkspaceID: "w1", HookID: hook.ID()})

	require.NoError(t, err)
	assert.Equal(t, models.RunFailed, outcome.Status)
	assert.Equal(t, 3, outcome.ExitCode)
	assert.Contains(t, outcome.Output, "broken")
	assert.Zero(t, countRuns(t, db))
}

// A directory that is gone cannot run anything: the test says so instead of running somewhere else.
func TestTestHookRefusesAWorkspaceDirectoryThatIsGone(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("the script uses sh")
	}
	gone := filepath.Join(t.TempDir(), "removed")
	module, db := testModule(t, gone)
	hook := saveScript(t, module, true, "-c", "true")

	_, err := module.TestHook.Execute(context.Background(), application.TestHookInput{WorkspaceID: "w1", HookID: hook.ID()})

	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
	assert.Zero(t, countRuns(t, db))
}

func readFile(t *testing.T, dir string, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return strings.TrimSpace(string(content))
}
