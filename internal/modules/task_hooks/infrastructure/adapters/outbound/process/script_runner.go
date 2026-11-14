// Package process runs the script hooks as child processes of the server.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

const (
	// maxOutput bounds the captured output of one run.
	maxOutput = 1 << 20
	// killGrace is how long the pipes may stay open after the process was killed on timeout.
	killGrace = 2 * time.Second
)

// ScriptRunner runs a script hook with exec (no shell) unless the hook asks for one, in the worktree of the
// task, with the IWF_* variables and the hook variables added to the server environment.
type ScriptRunner struct{}

var _ outbound.ScriptRunner = (*ScriptRunner)(nil)

// NewScriptRunner builds the runner.
func NewScriptRunner() *ScriptRunner { return &ScriptRunner{} }

// Run expands the argument templates, starts the process and waits for it. The run is bounded by the hook
// timeout and by ctx; a timeout is reported as RunTimedOut, a non-zero exit as RunFailed.
func (r *ScriptRunner) Run(ctx context.Context, hook models.Hook, run models.RunContext) (models.RunOutcome, error) {
	script := hook.Script()
	if script.Command == "" {
		return models.RunOutcome{}, fmt.Errorf("%w: hook %s has no command", hookerrors.ErrRunNotStarted, hook.ID())
	}
	dir := workingDir(script, run)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return models.RunOutcome{}, fmt.Errorf("%w: the directory %s does not exist", hookerrors.ErrRunNotStarted, dir)
	}
	args, err := expandArgs(script.Args, run)
	if err != nil {
		return models.RunOutcome{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, hook.Timeout())
	defer cancel()
	cmd, err := buildCommand(runCtx, script, args, dir)
	if err != nil {
		return models.RunOutcome{}, err
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), environment(script, run)...)
	ownProcessGroup(cmd)
	cmd.WaitDelay = killGrace
	output := &boundedBuffer{limit: maxOutput}
	cmd.Stdout = output
	cmd.Stderr = output

	err = cmd.Run()
	outcome := models.RunOutcome{Status: models.RunOK, Output: output.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		outcome.Status, outcome.ExitCode = models.RunTimedOut, -1
	case ctx.Err() != nil:
		// The process was started and then stopped with its owner: it is cancelled, not "not started".
		return models.RunOutcome{}, fmt.Errorf("%w: %w", hookerrors.ErrRunCancelled, ctx.Err())
	case errors.As(err, &exitErr):
		outcome.Status, outcome.ExitCode = models.RunFailed, exitErr.ExitCode()
	default:
		return models.RunOutcome{}, fmt.Errorf("%w: %w", hookerrors.ErrRunNotStarted, err)
	}
	return outcome, nil
}

// workingDir is the directory of the run: the one the hook names (relative to the worktree) or the worktree.
func workingDir(script models.ScriptSpec, run models.RunContext) string {
	switch {
	case script.WorkingDir == "":
		return run.WorktreePath
	case filepath.IsAbs(script.WorkingDir):
		return script.WorkingDir
	default:
		return filepath.Join(run.WorktreePath, script.WorkingDir)
	}
}

// shellSyntax are the characters that only a shell understands: whitespace, quoting, expansion, pipes and lists.
const shellSyntax = " \t\n|&;<>()$`\"'*?[]#~{}!\\"

// isFile reports whether the command names an existing file, relative paths being taken from the working directory.
// A path of that kind runs directly even when it holds spaces or other characters a shell would parse.
func isFile(command, dir string) bool {
	if !strings.ContainsRune(command, filepath.Separator) {
		return false
	}
	path := command
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// needsShell reports whether the command is a command line rather than the name or path of one program. A stored
// UseShell flag from an older hook keeps forcing the shell.
func needsShell(script models.ScriptSpec, dir string) bool {
	if script.UseShell {
		return true
	}
	return strings.ContainsAny(script.Command, shellSyntax) && !isFile(script.Command, dir)
}

// readsPositionals reports whether the command line already uses its arguments by itself.
func readsPositionals(command string) bool {
	return readsAny(command, positionals)
}

// readsAny reports whether the command line contains any of the markers a shell uses to read its arguments.
func readsAny(command string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(command, marker) {
			return true
		}
	}
	return false
}

// canAppendParameters reports whether text can be added at the end of the command line without changing what it
// runs: no comment, no second line and no trailing operator or escape that would take the added text with it.
func canAppendParameters(command string) bool {
	trimmed := strings.TrimSpace(command)
	return trimmed != "" && !strings.ContainsAny(trimmed, "#\n") && !strings.ContainsAny(trimmed[len(trimmed)-1:], "&;|\\(")
}

// buildCommand runs a bare program directly, or a command line through the platform shell (sh -c, PowerShell on Windows). A hook saved with the old shell flag
// gets the arguments as positional parameters only. A command line detected here also receives them at its end,
// unless it reads them itself, so the extra parameters are never dropped. The shell never parses them. When the
// parameters cannot be added without changing the line, the run does not start and the error says where to put them.
func buildCommand(ctx context.Context, script models.ScriptSpec, args []string, dir string) (*exec.Cmd, error) {
	if !needsShell(script, dir) {
		return exec.CommandContext(ctx, script.Command, args...), nil //nolint:gosec // the command is configured by the user on purpose
	}
	command := script.Command
	if !script.UseShell && len(args) > 0 && !readsPositionals(command) {
		if !canAppendParameters(command) {
			return nil, fmt.Errorf("%w: the command line ends in an operator, a comment or has several lines, so the extra parameters cannot be added at its end: write %s where they go", hookerrors.ErrRunNotStarted, parametersToken)
		}
		command += appendedParameters
	}
	return shellCommand(ctx, command, args), nil
}

func expandArgs(args []string, run models.RunContext) ([]string, error) {
	expanded := make([]string, 0, len(args))
	for _, arg := range args {
		value, err := models.ExpandTemplate(arg, run)
		if err != nil {
			return nil, fmt.Errorf("%w: argument %q: %w", hookerrors.ErrRunNotStarted, arg, err)
		}
		expanded = append(expanded, value)
	}
	return expanded, nil
}

// environment returns the standard IWF_* variables followed by the hook variables, which win on a clash.
func environment(script models.ScriptSpec, run models.RunContext) []string {
	env := []string{
		"IWF_TASK_ID=" + run.TaskID,
		"IWF_TASK_TITLE=" + run.TaskTitle,
		"IWF_TASK_STATUS=" + run.TaskStatus,
		"IWF_WORKSPACE_PATH=" + run.WorkspacePath,
		"IWF_WORKTREE_PATH=" + run.WorktreePath,
		"IWF_BRANCH=" + run.Branch,
		"IWF_PHASE=" + string(run.Phase),
		"IWF_EVENT=" + string(run.Event),
	}
	if run.Phase == models.PhasePost {
		env = append(env, "IWF_TASK_RESULT="+string(run.Result))
	}
	for _, variable := range script.Env {
		env = append(env, variable.Name+"="+variable.Value)
	}
	return env
}

// boundedBuffer keeps the first limit bytes and drops the rest, so a chatty script cannot exhaust memory.
type boundedBuffer struct {
	buf     bytes.Buffer
	limit   int
	dropped int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.buf.Len()
	if room > len(p) {
		room = len(p)
	}
	if room > 0 {
		b.buf.Write(p[:room])
	}
	b.dropped += len(p) - max(room, 0)
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	if b.dropped == 0 {
		return b.buf.String()
	}
	return b.buf.String() + "\n[output truncated: " + strconv.Itoa(b.dropped) + " bytes dropped]"
}
