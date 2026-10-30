//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"strings"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// CommitProjectChanges commits what is staged in a project with the given message.
type CommitProjectChanges struct {
	roots outbound.WorkspaceRootPort
	git   outbound.GitStatusPort
}

// NewCommitProjectChanges builds the use case.
func NewCommitProjectChanges(roots outbound.WorkspaceRootPort, git outbound.GitStatusPort) *CommitProjectChanges {
	return &CommitProjectChanges{roots: roots, git: git}
}

// Execute trims the message, rejects a blank one before touching git, then commits.
func (u *CommitProjectChanges) Execute(ctx context.Context, scope models.ProjectScope, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return treeerrors.EmptyCommitMessage()
	}
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return err
	}
	return u.git.Commit(ctx, root.Path, message)
}
