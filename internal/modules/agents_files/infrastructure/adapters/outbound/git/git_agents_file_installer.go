//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
)

const (
	targetFileName    = "AGENTS.md"
	commitMessage     = "docs: add AGENTS.md"
	fallbackUserName  = "iworkflow"
	fallbackUserEmail = "iworkflow@localhost"
)

// AgentsFileInstaller writes AGENTS.md in a folder and, when that folder is inside a git
// repository, commits only that file. Other changes in the repository are left untouched.
type AgentsFileInstaller struct {
	logger *slog.Logger
}

var _ outbound.AgentsFileInstaller = (*AgentsFileInstaller)(nil)

// NewAgentsFileInstaller builds the installer. Commit failures are logged, not returned.
func NewAgentsFileInstaller(logger *slog.Logger) *AgentsFileInstaller {
	return &AgentsFileInstaller{logger: logger}
}

// Install writes the content to <dir>/AGENTS.md. A failed commit does not fail the install: the
// file is written and the outcome reports Committed=false.
func (a *AgentsFileInstaller) Install(ctx context.Context, content []byte, dir string, overwrite bool) (models.InstallOutcome, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return models.InstallOutcome{}, agentserrors.Install("la carpeta no existe: " + dir)
	}
	target := filepath.Join(dir, targetFileName)
	exists, err := pathExists(target)
	if err != nil {
		return models.InstallOutcome{}, agentserrors.Install(err.Error())
	}
	if exists && !overwrite {
		return models.InstallOutcome{}, agentserrors.TargetExists(target)
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return models.InstallOutcome{}, agentserrors.Install(fmt.Sprintf("no se pudo escribir %s: %v", target, err))
	}
	if !a.isInsideRepository(ctx, dir) {
		return models.InstallOutcome{Committed: false}, nil
	}
	committed, err := a.commitTarget(ctx, dir)
	if err != nil {
		a.logger.WarnContext(ctx, "AGENTS.md written but not committed", "dir", dir, "error", err)
		return models.InstallOutcome{Committed: false}, nil
	}
	return models.InstallOutcome{Committed: committed}, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// isInsideRepository reports whether git sees dir inside a work tree. Failing to start git counts as "no".
func (a *AgentsFileInstaller) isInsideRepository(ctx context.Context, dir string) bool {
	output, err := runGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		a.logger.WarnContext(ctx, "could not run git", "dir", dir, "error", err)
		return false
	}
	return output.success
}

// commitTarget stages and commits only AGENTS.md. It returns false, without error, when the file
// has no changes to commit.
func (a *AgentsFileInstaller) commitTarget(ctx context.Context, dir string) (bool, error) {
	add, err := runGit(ctx, dir, "add", "--", targetFileName)
	if err != nil {
		return false, agentserrors.Install(err.Error())
	}
	if !add.success {
		return false, agentserrors.Install(add.stderr)
	}
	staged, err := runGit(ctx, dir, "diff", "--cached", "--quiet", "--", targetFileName)
	if err != nil {
		return false, agentserrors.Install(err.Error())
	}
	if staged.success {
		return false, nil
	}
	identity, err := identityArgs(ctx, dir)
	if err != nil {
		return false, agentserrors.Install(err.Error())
	}
	args := append(identity, "commit", "-q", "--only", "-m", commitMessage, "--", targetFileName)
	commit, err := runGit(ctx, dir, args...)
	if err != nil {
		return false, agentserrors.Install(err.Error())
	}
	if !commit.success {
		return false, agentserrors.Install(commit.stderr)
	}
	return true, nil
}

// identityArgs returns the `-c` flags that supply a commit identity when the repository has none.
func identityArgs(ctx context.Context, dir string) ([]string, error) {
	hasName, err := gitConfigIsSet(ctx, dir, "user.name")
	if err != nil {
		return nil, err
	}
	if hasName {
		hasEmail, err := gitConfigIsSet(ctx, dir, "user.email")
		if err != nil {
			return nil, err
		}
		if hasEmail {
			return []string{}, nil
		}
	}
	return []string{
		"-c", "user.name=" + fallbackUserName,
		"-c", "user.email=" + fallbackUserEmail,
	}, nil
}

func gitConfigIsSet(ctx context.Context, dir string, key string) (bool, error) {
	output, err := runGit(ctx, dir, "config", key)
	if err != nil {
		return false, err
	}
	return output.success && strings.TrimSpace(output.stdout) != "", nil
}
