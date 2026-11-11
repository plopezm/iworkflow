package task_hooks_test

import (
	"context"
	"database/sql"
	"errors"
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

func saveScript(t *testing.T, module *di.Module, args ...string) models.Hook {
	t.Helper()
	saved, err := module.SaveHook.Execute(context.Background(), application.SaveHookInput{
		Name: "script", Kind: models.KindScript, Timeout: 30 * time.Second,
		Script: models.ScriptSpec{Command: "sh", Args: args},
	})
	require.NoError(t, err)
	return saved
}

// preTest is the input of a test as a before-start hook of the workspace.
func preTest(workspaceID string, hookID string) application.TestHookInput {
	return application.TestHookInput{WorkspaceID: workspaceID, HookID: hookID, Event: models.EventTaskStart, Phase: models.PhasePre}
}

func countRuns(t *testing.T, db *sql.DB) int {
	t.Helper()
	var runs int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM task_hook_runs`).Scan(&runs))
	return runs
}

// A test runs the hook in the workspace directory with the synthetic task, even when the hook is assigned nowhere, and writes
// no run record: the run history of the tasks is not touched by trying a hook.
func TestTestHookRunsInTheWorkspaceAndRecordsNothing(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("the script uses sh")
	}
	dir := t.TempDir()
	module, db := testModule(t, dir)
	hook := saveScript(t, module, "-c", `pwd > cwd.txt; printf %s "$IWF_TASK_TITLE" > title.txt`)

	outcome, err := module.TestHook.Execute(context.Background(), preTest("w1", hook.ID()))

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
	hook := saveScript(t, module, "-c", "echo broken; exit 3")

	outcome, err := module.TestHook.Execute(context.Background(), preTest("w1", hook.ID()))

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
	hook := saveScript(t, module, "-c", "true")

	_, err := module.TestHook.Execute(context.Background(), preTest("w1", hook.ID()))

	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
	assert.Zero(t, countRuns(t, db))
}

func readFile(t *testing.T, dir string, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return strings.TrimSpace(string(content))
}

// A script that runs past its timeout is an outcome of the test, the same as in a real run: timed out, not an error.
func TestTestHookRecordsATimeoutAsAnOutcome(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("the script uses sh")
	}
	module, db := testModule(t, t.TempDir())
	saved, err := module.SaveHook.Execute(context.Background(), application.SaveHookInput{
		Name: "slow", Kind: models.KindScript, Timeout: time.Second,
		Script: models.ScriptSpec{Command: "sh", Args: []string{"-c", "sleep 30"}},
	})
	require.NoError(t, err)

	outcome, err := module.TestHook.Execute(context.Background(), preTest("w1", saved.ID()))

	require.NoError(t, err)
	assert.Equal(t, models.RunTimedOut, outcome.Status)
	assert.Zero(t, countRuns(t, db))
}

// A script that cannot be started at all (no such executable) is refused as not started, with nothing recorded.
func TestTestHookReportsAScriptThatCannotStartAsNotStarted(t *testing.T) {
	t.Parallel()
	module, db := testModule(t, t.TempDir())
	saved, err := module.SaveHook.Execute(context.Background(), application.SaveHookInput{
		Name: "missing", Kind: models.KindScript, Timeout: 30 * time.Second,
		Script: models.ScriptSpec{Command: "/nonexistent/iwf-missing-command"},
	})
	require.NoError(t, err)

	_, err = module.TestHook.Execute(context.Background(), preTest("w1", saved.ID()))

	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
	assert.Zero(t, countRuns(t, db))
}

// An agent hook without a configured agent session cannot be tried: the refusal says why and nothing runs.
func TestTestHookOfAnAgentWithoutASessionIsNotStarted(t *testing.T) {
	t.Parallel()
	module, db := testModule(t, t.TempDir())
	saved, err := module.SaveHook.Execute(context.Background(), application.SaveHookInput{
		Name: "agent", Kind: models.KindAgent, Timeout: 30 * time.Second,
		Agent: models.AgentSpec{Prompt: "check the branch"},
	})
	require.NoError(t, err)

	_, err = module.TestHook.Execute(context.Background(), preTest("w1", saved.ID()))

	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
	assert.ErrorContains(t, err, "no agent session is configured")
	assert.Zero(t, countRuns(t, db))
}

// An unknown hook id is not found, and a test without event and phase is refused: the test never runs either.
func TestTestHookRefusesUnknownHooksAndMissingPhase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	module, _ := testModule(t, dir)
	saved := saveScript(t, module, "-c", "touch ran.txt")

	_, err := module.TestHook.Execute(context.Background(), preTest("w1", "missing"))
	require.ErrorIs(t, err, hookerrors.ErrHookNotFound)

	_, err = module.TestHook.Execute(context.Background(), application.TestHookInput{WorkspaceID: "w1", HookID: saved.ID()})
	require.ErrorIs(t, err, hookerrors.ErrInvalidHook)

	_, statErr := os.Stat(filepath.Join(dir, "ran.txt"))
	assert.True(t, os.IsNotExist(statErr), "a refused test runs nothing")
}

// failingWorkspaces is a lookup that cannot find the workspace directory.
type failingWorkspaces struct{ err error }

func (f failingWorkspaces) Path(context.Context, string) (string, error) { return "", f.err }

// A workspace lookup that fails stops the test before any run, and the cause is kept in the error chain.
func TestTestHookFailsWhenTheWorkspaceCannotBeFound(t *testing.T) {
	t.Parallel()
	db := dbtest.New(t)
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1', 'one', '/tmp/one', 'main', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	failure := errors.New("workspace removed")
	module := di.NewModule(di.Deps{DB: db, Clock: system.NewClock(), Workspaces: failingWorkspaces{err: failure}})
	saved := saveScript(t, module, "-c", "true")

	_, err = module.TestHook.Execute(context.Background(), preTest("w1", saved.ID()))

	require.ErrorIs(t, err, failure)
	assert.Zero(t, countRuns(t, db))
}
