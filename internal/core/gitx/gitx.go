// Package gitx runs the git CLI without any domain knowledge.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// FailedError reports a git command that exited with a failure status.
type FailedError struct {
	// Args are the git arguments joined by spaces.
	Args string
	// Code is the exit status, or nil when it is not known.
	Code *int
	// Stderr is the trimmed standard error.
	Stderr string
}

// Error keeps the wording of the original implementation.
func (e *FailedError) Error() string {
	return fmt.Sprintf("git %s falló (%s): %s", e.Args, codeText(e.Code), e.Stderr)
}

// codeText renders the exit code the way Rust's Debug printed Option<i32>.
func codeText(code *int) string {
	if code == nil {
		return "None"
	}
	return "Some(" + strconv.Itoa(*code) + ")"
}

// ErrSpawn is wrapped when git itself cannot be started or fed.
var ErrSpawn = errors.New("no se pudo ejecutar git")

// Output is the raw result of a git command, for cases where a non-zero exit is expected.
type Output struct {
	Success bool
	Stdout  string
	Stderr  string
}

// baseEnv returns the inherited environment with prompts disabled, so git never blocks on input.
func baseEnv(extra []string) []string {
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return append(env, extra...)
}

// RunRaw runs `git -C dir args...` and returns the output without treating a failure as an error.
func RunRaw(ctx context.Context, dir string, args ...string) (Output, error) {
	return RunRawEnv(ctx, dir, nil, args...)
}

// RunRawEnv is RunRaw with extra environment entries ("KEY=VALUE") for this process only. It passes
// sensitive configuration (GIT_CONFIG_*) without putting it on the command line.
func RunRawEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (Output, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", gitArgs(dir, args)...)
	cmd.Env = baseEnv(extraEnv)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return Output{}, fmt.Errorf("%w: %w", ErrSpawn, err)
		}
	}
	return Output{
		Success: err == nil,
		Stdout:  validUTF8(stdout.Bytes()),
		Stderr:  strings.TrimSpace(validUTF8(stderr.Bytes())),
	}, nil
}

// Run runs `git -C dir args...` and returns the trimmed stdout. A non-zero exit is a *FailedError.
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := RunRaw(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	if !out.Success {
		return "", &FailedError{Args: strings.Join(args, " "), Code: nil, Stderr: out.Stderr}
	}
	return strings.TrimSpace(out.Stdout), nil
}

// RunWithStdin runs `git -C dir args...` feeding stdin to the process. It is needed by plumbing
// commands such as `hash-object --stdin` that do not touch the index.
func RunWithStdin(ctx context.Context, dir string, stdin []byte, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", gitArgs(dir, args)...)
	cmd.Env = baseEnv(nil)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return strings.TrimSpace(validUTF8(stdout.Bytes())), nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return "", fmt.Errorf("%w: %w", ErrSpawn, err)
	}
	code := exitErr.ExitCode()
	return "", &FailedError{
		Args:   strings.Join(args, " "),
		Code:   &code,
		Stderr: strings.TrimSpace(validUTF8(stderr.Bytes())),
	}
}

// longPathsArgs enable long paths (over 260 characters) on Windows, where worktrees fail without it.
func longPathsArgs() []string {
	return []string{"config", "--global", "core.longpaths", "true"}
}

// EnableLongPaths sets core.longpaths in the global git configuration. It only applies on Windows;
// elsewhere it does nothing.
func EnableLongPaths(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	return runLongPathsConfig(ctx, "")
}

// runLongPathsConfig runs the configuration. A non-empty globalConfig redirects the global file
// through GIT_CONFIG_GLOBAL.
func runLongPathsConfig(ctx context.Context, globalConfig string) error {
	args := longPathsArgs()
	cmd := exec.CommandContext(ctx, "git", args...)
	var extra []string
	if globalConfig != "" {
		extra = append(extra, "GIT_CONFIG_GLOBAL="+globalConfig)
	}
	cmd.Env = baseEnv(extra)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return fmt.Errorf("%w: %w", ErrSpawn, err)
		}
		code := exitErr.ExitCode()
		return &FailedError{
			Args:   strings.Join(args, " "),
			Code:   &code,
			Stderr: strings.TrimSpace(validUTF8(stderr.Bytes())),
		}
	}
	return nil
}

// gitArgs prefixes the arguments with `-C dir`.
func gitArgs(dir string, args []string) []string {
	return append([]string{"-C", dir}, args...)
}

// validUTF8 decodes bytes replacing invalid sequences, like String::from_utf8_lossy.
func validUTF8(raw []byte) string {
	return strings.ToValidUTF8(string(raw), "�")
}
