//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// ListProjectGitChanges lists the paths of a project that differ from git HEAD.
type ListProjectGitChanges struct {
	roots outbound.WorkspaceRootPort
	git   outbound.GitStatusPort
}

// NewListProjectGitChanges builds the use case.
func NewListProjectGitChanges(roots outbound.WorkspaceRootPort, git outbound.GitStatusPort) *ListProjectGitChanges {
	return &ListProjectGitChanges{roots: roots, git: git}
}

// Execute resolves the project root, then asks git for its changes.
func (u *ListProjectGitChanges) Execute(ctx context.Context, scope models.ProjectScope) ([]models.GitChange, error) {
	root, err := u.roots.Resolve(ctx, scope)
	if err != nil {
		return nil, err
	}
	return u.git.Changes(ctx, root.Path)
}
