package process_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/process"
)

func newHook(t *testing.T, spec models.ScriptSpec, timeout time.Duration) models.Hook {
	t.Helper()
	hook, err := models.NewHook(models.HookParams{
		ID: "h", WorkspaceID: "w", Phase: models.PhasePost, Event: models.EventTaskFinish, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureBlock, Timeout: timeout, Script: spec,
	})
	require.NoError(t, err)
	return hook
}

func runCtx(dir string) models.RunContext {
	return models.RunContext{
		TaskID: "t1", TaskTitle: "a; touch pwned", TaskStatus: "done", WorkspacePath: "/ws", WorktreePath: dir,
		Branch: "feat/x", Phase: models.PhasePost, Event: models.EventTaskFinish, Result: models.TaskResultOK,
	}
}

func TestRunsInWorktreeWithEnvAndTemplates(t *testing.T) {
	dir := t.TempDir()
	hook := newHook(t, models.ScriptSpec{
		Command: "sh", Args: []string{"-c", `echo "$0|$1|$IWF_TASK_ID|$IWF_BRANCH|$IWF_PHASE|$IWF_EVENT|$IWF_TASK_RESULT|$CUSTOM|$(pwd)"`, "{{.Branch}}", "{{.TaskTitle}}"},
		Env: []models.EnvVar{{Name: "CUSTOM", Value: "yes"}},
	}, time.Minute)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(dir))
	require.NoError(t, err)
	assert.Equal(t, models.RunOK, out.Status)
	real, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	assert.Equal(t, "feat/x|a; touch pwned|t1|feat/x|post|task_finish|ok|yes|"+real, strings.TrimSpace(out.Output))
	_, statErr := os.Stat(filepath.Join(dir, "pwned"))
	assert.True(t, os.IsNotExist(statErr), "the title must not be run by a shell")
}

func TestPreHookHasNoTaskResult(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "sh", Args: []string{"-c", `echo "[${IWF_TASK_RESULT-unset}]"`}}, time.Minute)
	rc := runCtx(t.TempDir())
	rc.Phase, rc.Result = models.PhasePre, models.TaskResultNone
	out, err := process.NewScriptRunner().Run(context.Background(), hook, rc)
	require.NoError(t, err)
	assert.Equal(t, "[unset]", strings.TrimSpace(out.Output))
}

func TestNonZeroExitIsFailedOutcome(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "sh", Args: []string{"-c", "echo boom >&2; exit 3"}}, time.Minute)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.NoError(t, err)
	assert.Equal(t, models.RunFailed, out.Status)
	assert.Equal(t, 3, out.ExitCode)
	assert.True(t, out.Failed())
	assert.Contains(t, out.Output, "boom")
}

func TestTimeoutIsTimedOutOutcome(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "sleep", Args: []string{"30"}}, 150*time.Millisecond)
	start := time.Now()
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.NoError(t, err)
	assert.Equal(t, models.RunTimedOut, out.Status)
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestUseShellPassesArgsAsPositionals(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: `echo "$1" | tr a-z A-Z`, Args: []string{"{{.TaskTitle}}"}, UseShell: true}, time.Minute)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.NoError(t, err)
	assert.Equal(t, "A; TOUCH PWNED", strings.TrimSpace(out.Output))
}

func TestNotStartedErrors(t *testing.T) {
	runner := process.NewScriptRunner()
	cases := map[string]struct {
		spec models.ScriptSpec
		dir  string
	}{
		"bad template":   {models.ScriptSpec{Command: "echo", Args: []string{"{{.Nope}}"}}, t.TempDir()},
		"missing dir":    {models.ScriptSpec{Command: "echo"}, filepath.Join(t.TempDir(), "gone")},
		"no executable":  {models.ScriptSpec{Command: "definitely-not-a-command-xyz"}, t.TempDir()},
		"unclosed brace": {models.ScriptSpec{Command: "echo", Args: []string{"{{.Branch"}}, t.TempDir()},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runner.Run(context.Background(), newHook(t, tc.spec, time.Minute), runCtx(tc.dir))
			assert.True(t, errors.Is(err, hookerrors.ErrRunNotStarted), "got %v", err)
		})
	}
}

func TestOutputIsBounded(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "sh", Args: []string{"-c", "head -c 3000000 /dev/zero | tr '\\0' x"}}, time.Minute)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.NoError(t, err)
	assert.Less(t, len(out.Output), 1<<20+100)
	assert.Contains(t, out.Output, "output truncated")
}
