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
		ID: "h", Name: "hook", Kind: models.KindScript, Timeout: timeout, Script: spec,
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

func TestCommandLineRunsThroughAShellWithoutTheFlag(t *testing.T) {
	cases := map[string]string{
		"words":    "echo hello world",
		"operator": "echo hello && echo world",
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			hook := newHook(t, models.ScriptSpec{Command: command}, time.Minute)
			out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
			require.NoError(t, err)
			assert.Equal(t, models.RunOK, out.Status)
			assert.Contains(t, out.Output, "hello")
		})
	}
}

func TestBareProgramRunsDirectlyWithoutParsingItsArgs(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "printf", Args: []string{"%s", "a b; $HOME"}}, time.Minute)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.NoError(t, err)
	assert.Equal(t, models.RunOK, out.Status)
	assert.Equal(t, "a b; $HOME", strings.TrimSpace(out.Output))
}

func TestStoredShellFlagStillForcesTheShellOnABareCommand(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "echo", Args: []string{"x"}, UseShell: true}, time.Minute)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.NoError(t, err)
	assert.Equal(t, models.RunOK, out.Status)
	assert.Equal(t, "", strings.TrimSpace(out.Output), "echo got no positional arguments: the shell passes them as $1")
}

func TestCommandLineReceivesTheExtraParametersAtTheEnd(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "echo base", Args: []string{"one", "two words; $HOME"}}, time.Minute)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.NoError(t, err)
	assert.Equal(t, models.RunOK, out.Status)
	assert.Equal(t, "base one two words; $HOME", strings.TrimSpace(out.Output))
}

func TestCommandLineThatReadsPositionalsDoesNotGetThemTwice(t *testing.T) {
	cases := map[string]string{
		"dollar one": `echo "$1" | tr a-z A-Z`,
		"dollar at":  `echo "$@" | tr a-z A-Z`,
		"braces":     `echo "${1}" | tr a-z A-Z`,
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			hook := newHook(t, models.ScriptSpec{Command: command, Args: []string{"abc"}}, time.Minute)
			out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
			require.NoError(t, err)
			assert.Equal(t, "ABC", strings.TrimSpace(out.Output))
		})
	}
}

func TestExistingFileWithSpacesRunsDirectly(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "my script.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho \"got:$1\"\n"), 0o700))
	cases := map[string]string{"absolute": script, "relative to the working directory": "./my script.sh"}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			hook := newHook(t, models.ScriptSpec{Command: command, Args: []string{"x y"}}, time.Minute)
			out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(dir))
			require.NoError(t, err)
			assert.Equal(t, models.RunOK, out.Status)
			assert.Equal(t, "got:x y", strings.TrimSpace(out.Output))
		})
	}
}

// A command line that ends in an operator or a comment cannot take the extra parameters at its end without changing
// what it runs, so it is refused before anything starts, and nothing of it runs.
func TestCommandLineEndingInATerminatorIsRefusedWithParameters(t *testing.T) {
	cases := map[string]string{
		"background": "touch %s &",
		"semicolon":  "touch %s;",
		"pipe":       "touch %s |",
		"and":        "touch %s &&",
		"comment":    "touch %s # note",
		"newline":    "touch %s\necho done",
		"backslash":  "touch %s \\",
	}
	for name, template := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "marker")
			command := strings.Replace(template, "%s", marker, 1)
			hook := newHook(t, models.ScriptSpec{Command: command, Args: []string{"extra"}}, time.Minute)

			_, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(dir))

			require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
			assert.Contains(t, err.Error(), `"$@"`)
			assert.NoFileExists(t, marker)
		})
	}
}

// Without parameters nothing is appended, so the same command lines run as written.
func TestCommandLineEndingInATerminatorRunsWithoutParameters(t *testing.T) {
	cases := map[string]string{
		"comment":    "echo hi # note",
		"background": "echo hi &",
		"semicolon":  "echo hi;",
	}
	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			hook := newHook(t, models.ScriptSpec{Command: command}, time.Minute)
			out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
			require.NoError(t, err)
			assert.Equal(t, models.RunOK, out.Status)
			assert.Contains(t, out.Output, "hi")
		})
	}
}

// A line that places the parameters itself is never refused, whatever it ends with.
func TestCommandLineThatPlacesTheParametersItselfIsNeverRefused(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: `echo "$@" # note`, Args: []string{"abc"}}, time.Minute)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.NoError(t, err)
	assert.Equal(t, "abc", strings.TrimSpace(out.Output))
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

func TestRunsInTheWorkingDirectoryOfTheHook(t *testing.T) {
	worktree := t.TempDir()
	sub := filepath.Join(worktree, "sub")
	require.NoError(t, os.Mkdir(sub, 0o750))
	other := t.TempDir()
	tests := map[string]struct{ dir, want string }{
		"relative to the worktree": {dir: "sub", want: sub},
		"absolute":                 {dir: other, want: other},
		"empty is the worktree":    {dir: "", want: worktree},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hook := newHook(t, models.ScriptSpec{Command: "pwd", WorkingDir: tc.dir}, time.Minute)
			out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(worktree))
			require.NoError(t, err)
			want, err := filepath.EvalSymlinks(tc.want)
			require.NoError(t, err)
			assert.Equal(t, want, strings.TrimSpace(out.Output))
		})
	}
}

func TestMissingWorkingDirectoryDoesNotStartTheRun(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "pwd", WorkingDir: "nope"}, time.Minute)
	_, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(t.TempDir()))
	require.ErrorIs(t, err, hookerrors.ErrRunNotStarted)
}

// A run whose owner is cancelled while the process runs is reported as cancelled, not as "could not start": the
// process did start, and it is stopped with its owner.
func TestCancelledRunIsReportedAsCancelledNotAsNotStarted(t *testing.T) {
	hook := newHook(t, models.ScriptSpec{Command: "sleep", Args: []string{"30"}}, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)

	started := time.Now()
	_, err := process.NewScriptRunner().Run(ctx, hook, runCtx(t.TempDir()))

	require.ErrorIs(t, err, hookerrors.ErrRunCancelled)
	assert.NotErrorIs(t, err, hookerrors.ErrRunNotStarted)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(started), 10*time.Second, "the process was stopped")
}
