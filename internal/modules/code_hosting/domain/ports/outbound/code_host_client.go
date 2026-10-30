// Package outbound holds the interfaces the code_hosting use cases need from infrastructure.
package outbound

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/models"
)

// CodeHostClient is the API of one provider (GitLab or GitHub). project is `namespace/project` on
// GitLab and `owner/repo` on GitHub.
type CodeHostClient interface {
	// CurrentUser returns the user the token authenticates; it also checks the connection.
	CurrentUser(ctx context.Context, settings models.CodeHostSettings) (string, error)
	BranchExists(ctx context.Context, settings models.CodeHostSettings, project string, branch string) (bool, error)
	// FindLatestMergeRequest returns the newest request of any state whose source is sourceBranch,
	// preferring an open one. It returns nil and no error when there is none.
	FindLatestMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, sourceBranch string) (*models.MergeRequest, error)
	CreateMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, request models.NewMergeRequest) (models.MergeRequest, error)
	GetMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (models.MergeRequest, error)
	// LatestPipeline returns the latest pipeline with its jobs, or nil when there is none.
	LatestPipeline(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (*models.Pipeline, error)
	MergeMergeRequest(ctx context.Context, settings models.CodeHostSettings, project string, iid uint64) (models.MergeRequest, error)
}
