// Package git installs AGENTS.md into project folders and commits it when they are git repositories.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitOutput is the raw result of one git command. A non-zero exit is a normal result here,
// not an error: callers decide what a failed command means.
type gitOutput struct {
	success bool
	stdout  string
	stderr  string
}

// runGit runs `git -C dir args...`. It returns an error only when git could not be started or
// the context was cancelled; a non-zero exit status is reported through gitOutput.success.
func runGit(ctx context.Context, dir string, args ...string) (gitOutput, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr != nil && ctx.Err() != nil {
		return gitOutput{}, fmt.Errorf("git cancelled: %w", ctx.Err())
	}
	var exitErr *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exitErr) {
		return gitOutput{}, fmt.Errorf("no se pudo ejecutar git: %w", runErr)
	}
	return gitOutput{
		success: runErr == nil,
		stdout:  stdout.String(),
		stderr:  strings.TrimSpace(stderr.String()),
	}, nil
}
