// Package git lists and deletes the branches of a workspace with the git command line. Every command runs under
// its own timeout, prompts are disabled, and every name is validated before git sees it.
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

	"iworkflow/internal/modules/workspace_branches/domain/models"
	"iworkflow/internal/modules/workspace_branches/domain/ports/outbound"
)

const (
	// listTimeout bounds the reads of the branch list.
	listTimeout = 30 * time.Second
	// deleteLocalTimeout bounds `git branch -D`.
	deleteLocalTimeout = 30 * time.Second
	// deleteRemoteTimeout bounds `git push --delete`, which talks to the network.
	deleteRemoteTimeout = 60 * time.Second
	// waitDelay bounds the wait for the output pipes after git exits, in case a credential helper keeps them open.
	waitDelay = 5 * time.Second
)

// Adapter implements outbound.BranchGit with the git CLI.
type Adapter struct{}

var _ outbound.BranchGit = (*Adapter)(nil)

// NewAdapter builds the adapter.
func NewAdapter() *Adapter {
	return &Adapter{}
}

// ListBranches reads the local and remote-tracking refs, the remotes, the current branch and the worktrees.
func (a *Adapter) ListBranches(ctx context.Context, repoPath string) ([]models.Branch, error) {
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	refs, err := run(ctx, repoPath, "for-each-ref", "--format=%(refname)", "refs/heads/", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	remotes, err := a.ListRemotes(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	// Empty with a detached HEAD: then no branch is the current one.
	current, err := run(ctx, repoPath, "branch", "--show-current")
	if err != nil {
		return nil, err
	}
	worktrees, err := run(ctx, repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseBranches(refs, remotes, current, checkedOutBranches(worktrees)), nil
}

// ListRemotes returns the names of the configured remotes, as `git remote` prints them.
func (a *Adapter) ListRemotes(ctx context.Context, repoPath string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	out, err := run(ctx, repoPath, "remote")
	if err != nil {
		return nil, err
	}
	var remotes []string
	for _, line := range strings.Split(out, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			remotes = append(remotes, name)
		}
	}
	return remotes, nil
}

// DeleteLocal force-deletes a local branch. The name is validated first; "--" keeps it out of the options.
func (a *Adapter) DeleteLocal(ctx context.Context, repoPath string, name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, deleteLocalTimeout)
	defer cancel()
	_, err := run(ctx, repoPath, "branch", "-D", "--", name)
	return err
}

// DeleteRemote deletes a branch on a remote. The remote and the branch are validated first. The branch is sent
// as its full ref, refs/heads/<name>: a bare name such as "tags/x" can match a tag too and git refuses it as
// ambiguous. A full ref that does not exist on the remote is not an error for git push, so the remote is asked
// first and a missing branch fails without deleting anything.
func (a *Adapter) DeleteRemote(ctx context.Context, repoPath string, remote string, name string) error {
	if err := checkName(remote); err != nil {
		return err
	}
	if err := checkName(name); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, deleteRemoteTimeout)
	defer cancel()
	ref := localRefPrefix + name
	listed, err := run(ctx, repoPath, "ls-remote", "--heads", remote, ref)
	if err != nil {
		return err
	}
	if !listsRef(listed, ref) {
		return fmt.Errorf("%s on remote %s: %w", ref, remote, errRemoteBranchMissing)
	}
	_, err = run(ctx, repoPath, "push", remote, "--delete", "--", ref)
	return err
}

// listsRef reports whether the `git ls-remote` output lists ref exactly, as its last field of a line.
func listsRef(listed string, ref string) bool {
	for _, line := range strings.Split(listed, "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[1] == ref {
			return true
		}
	}
	return false
}

// checkName rejects a value git would read as an option, or that is empty or contains NUL.
func checkName(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsRune(name, 0) {
		return fmt.Errorf("refusing to pass %q to git: %w", name, errInvalidName)
	}
	return nil
}

var (
	errInvalidName         = errors.New("invalid branch name")
	errRemoteBranchMissing = errors.New("remote branch does not exist")
)

// run executes `git -C dir args...` and returns its trimmed standard output. A non-zero exit is an error that
// carries git's stderr. A cancelled or timed-out context is reported as such, not as a git failure.
func run(ctx context.Context, dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = waitDelay
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), ctxErr)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return "", fmt.Errorf("start git: %w", err)
		}
		message := strings.TrimSpace(strings.ToValidUTF8(stderr.String(), "�"))
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), message)
	}
	return strings.TrimSpace(strings.ToValidUTF8(stdout.String(), "�")), nil
}
