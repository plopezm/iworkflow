// Package git manages the worktrees of the tasks with the git command line: creation, diffs, merges,
// removal and the cleanup of leftovers. Every path it touches is confined to the worktrees root.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"iworkflow/internal/core/gitx"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// waitDelay bounds the wait for the output pipes after git exits. A child that inherited the pipes
// (a credential helper, for instance) would otherwise keep the command waiting after it was killed.
const waitDelay = 5 * time.Second

// result is the outcome of one git command. A failed exit is a result, not an error: callers decide.
type result struct {
	success bool
	stdout  string
	stderr  string
}

// runRaw runs `git -C dir args...`. It returns an error only when git cannot be started or when the
// context ends; a non-zero exit is reported in the result. Cancelling ctx kills git and every process
// it started.
func runRaw(ctx context.Context, dir string, args ...string) (result, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = waitDelay
	killProcessTree(cmd)
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result{}, fmt.Errorf("git %s cancelled: %w", strings.Join(args, " "), ctxErr)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return result{}, fmt.Errorf("%w: %w", gitx.ErrSpawn, err)
		}
	}
	return result{
		success: err == nil,
		stdout:  strings.ToValidUTF8(stdout.String(), "\uFFFD"),
		stderr:  strings.TrimSpace(strings.ToValidUTF8(stderr.String(), "\uFFFD")),
	}, nil
}

// git runs a command that must succeed and returns its trimmed standard output, as the Rust run_git did.
// A failure is a domain Git error carrying the same text as the Rust version.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := runRaw(ctx, dir, args...)
	if err != nil {
		if ctx.Err() != nil {
			// A cancelled context is not a git failure: callers must still be able to see it.
			return "", err
		}
		return "", werrors.Git(err.Error())
	}
	if !out.success {
		failure := &gitx.FailedError{Args: strings.Join(args, " "), Stderr: out.stderr}
		return "", werrors.Git(failure.Error())
	}
	return strings.TrimSpace(out.stdout), nil
}

// refExists reports whether ref resolves in dir.
func refExists(ctx context.Context, dir string, ref string) (bool, error) {
	out, err := runRaw(ctx, dir, "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		if ctx.Err() != nil {
			return false, err
		}
		return false, werrors.Git(err.Error())
	}
	return out.success, nil
}

// splitLines returns the lines of a command output; an empty output has no lines.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
