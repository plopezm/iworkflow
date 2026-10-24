package outbound

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/models"
)

// GitRemote reads and publishes branches through the `origin` remote of a repository.
type GitRemote interface {
	// OriginURL returns the URL of `origin`; it fails with errors.NoOrigin when there is none.
	OriginURL(ctx context.Context, repo string) (string, error)
	// PublishBranch pushes the local branch to the project of the provider, authenticating with the
	// token of settings and not with the system git credentials. The branch must exist in repo.
	PublishBranch(ctx context.Context, repo string, branch string, settings models.CodeHostSettings, project string) error
	// EnsureCommitsAhead makes sure branch has at least one commit ahead of base. When it has none, it
	// adds an empty commit without touching the working tree, because providers reject requests with
	// no commits. It reports whether it created that commit.
	EnsureCommitsAhead(ctx context.Context, repo string, branch string, base string) (bool, error)
}
