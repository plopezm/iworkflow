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
	"slices"
	"strings"
	"time"

	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	"iworkflow/internal/modules/workspace_branches/domain/models"
	"iworkflow/internal/modules/workspace_branches/domain/ports/outbound"
)

const (
	// listTimeout bounds each single git command of the branch list read, not the whole read.
	listTimeout = 30 * time.Second
	// deleteLocalTimeout bounds the single `git branch -D` command.
	deleteLocalTimeout = 30 * time.Second
	// deleteRemoteTimeout bounds each single git command of a remote deletion (up to three, so the whole deletion can take longer); they talk to the network.
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
	return listBranches(ctx, repoPath, listTimeout)
}

// listBranches is ListBranches with the budget of each git command given by limit.
func listBranches(ctx context.Context, repoPath string, limit time.Duration) ([]models.Branch, error) {
	refs, err := bounded(ctx, limit, repoPath, "for-each-ref", "--format=%(refname)", "refs/heads/", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	remotes, err := remotesWithin(ctx, repoPath, limit)
	if err != nil {
		return nil, err
	}
	// Empty with a detached HEAD: then no branch is the current one.
	current, err := bounded(ctx, limit, repoPath, "branch", "--show-current")
	if err != nil {
		return nil, err
	}
	worktrees, err := bounded(ctx, limit, repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseBranches(refs, remotes, current, checkedOutBranches(worktrees)), nil
}

// ListRemotes returns the names of the configured remotes, as `git remote` prints them.
func (a *Adapter) ListRemotes(ctx context.Context, repoPath string) ([]string, error) {
	return remotesWithin(ctx, repoPath, listTimeout)
}

// remotesWithin is ListRemotes with its git command bounded by limit.
func remotesWithin(ctx context.Context, repoPath string, limit time.Duration) ([]string, error) {
	out, err := bounded(ctx, limit, repoPath, "remote")
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
	_, err := bounded(ctx, deleteLocalTimeout, repoPath, "branch", "-D", "--", name)
	return err
}

// DeleteRemote deletes a branch on a remote, in this order:
//  1. The branch is validated and the remote has to be one of the configured remotes, so a URL or a path never
//     receives the push.
//  2. The remote is asked whether it lists the branch; a missing branch fails with werrors.ErrBranchNotFound without
//     deleting anything. The branch is sent as its full ref, refs/heads/<name>: a bare name such as "tags/x" can
//     match a tag too and git refuses it as ambiguous, and a full ref that does not exist on the remote is not an
//     error for git push, which is why step 2 exists.
//  3. The push deletes the ref.
//  4. The remote is asked again: a ref that is still listed means the push did not delete it, and that is an error.
func (a *Adapter) DeleteRemote(ctx context.Context, repoPath string, remote string, name string) error {
	if err := checkName(remote); err != nil {
		return err
	}
	if err := checkName(name); err != nil {
		return err
	}
	configured, err := remotesWithin(ctx, repoPath, listTimeout)
	if err != nil {
		return err
	}
	if !slices.Contains(configured, remote) {
		return fmt.Errorf("%q is not a configured remote of the repository: %w", remote, werrors.ErrInvalidBranch)
	}
	ref := localRefPrefix + name
	listed, err := bounded(ctx, deleteRemoteTimeout, repoPath, "ls-remote", "--heads", remote, ref)
	if err != nil {
		return err
	}
	if !listsRef(listed, ref) {
		return fmt.Errorf("%s on remote %s: %w", ref, remote, werrors.ErrBranchNotFound)
	}
	if _, err := bounded(ctx, deleteRemoteTimeout, repoPath, "push", remote, "--delete", "--", ref); err != nil {
		return err
	}
	// Between the two commands the branch may have vanished; git push then can exit 0 with a localized warning.
	// The remote decides whether the ref is gone, not the exit status of the push.
	after, err := bounded(ctx, deleteRemoteTimeout, repoPath, "ls-remote", "--heads", remote, ref)
	if err != nil {
		return err
	}
	if listsRef(after, ref) {
		return fmt.Errorf("push to remote %s reported success but %s is still listed", remote, ref)
	}
	return nil
}

// listsRef reports whether the `git ls-remote` output lists ref exactly. Each line is "<sha>\t<ref>": the split is
// on the tab, so a ref name containing whitespace (legal in git) is compared whole.
func listsRef(listed string, ref string) bool {
	for _, line := range strings.Split(listed, "\n") {
		if _, name, found := strings.Cut(line, "\t"); found && name == ref {
			return true
		}
	}
	return false
}

// checkName rejects a value git would read as an option, or that is empty or contains NUL.
func checkName(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.ContainsRune(name, 0) {
		return fmt.Errorf("refusing to pass %q to git: %w", name, werrors.ErrInvalidBranch)
	}
	return nil
}

// bounded runs one git command under its own timeout, derived from ctx. Each command gets its own budget, so
// a slow read does not leave less time for the next one.
func bounded(ctx context.Context, limit time.Duration, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	return run(ctx, dir, args...)
}

// run executes `git -C dir args...` and returns its trimmed standard output. A non-zero exit is an error that
// carries git's stderr. A cancelled or timed-out context is reported as such, not as a git failure.
func run(ctx context.Context, dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = waitDelay
	runErr := finished(cmd.Run(), cmd.ProcessState)
	return outcome(ctx, runErr, stdout.String(), stderr.String(), args)
}

// finished drops exec.ErrWaitDelay when the process itself exited 0: git succeeded and only a helper it started kept
// the output pipes open past WaitDelay, which is not a git failure. Any other error is returned unchanged.
func finished(runErr error, state *os.ProcessState) error {
	if errors.Is(runErr, exec.ErrWaitDelay) && state != nil && state.Success() {
		return nil
	}
	return runErr
}

// outcome turns the end of one git command into the answer of run. A command that exited 0 is a success even when
// the context expired right after it; a context error is reported only for a command that failed.
func outcome(ctx context.Context, runErr error, stdout string, stderr string, args []string) (string, error) {
	if runErr == nil {
		return strings.TrimSpace(strings.ToValidUTF8(stdout, "�")), nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), ctxErr)
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return "", fmt.Errorf("start git: %w", runErr)
	}
	message := strings.TrimSpace(strings.ToValidUTF8(stderr, "�"))
	return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), message)
}
