package outbound

import (
	"context"
	"time"

	"iworkflow/internal/modules/jira/domain/models"
)

// IssueRepository stores the imported issues and the membership of each issue in the lanes.
type IssueRepository interface {
	// List returns every imported issue, most recently updated in Jira first.
	List(ctx context.Context) ([]models.Issue, error)
	// Find returns nil and no error when the issue is not imported.
	Find(ctx context.Context, key string) (*models.Issue, error)
	// LaneMemberships returns the issue keys of every lane, keyed by lane id.
	LaneMemberships(ctx context.Context) (map[string][]string, error)
	// ReplaceSynced stores a sync atomically: inserts new issues, refreshes the Jira data of known ones
	// (keeping their local state), replaces the membership of the lanes in batches (other lanes are
	// untouched) and removes New issues that no lane contains any more. It returns how many issues
	// were not stored before.
	ReplaceSynced(ctx context.Context, batches []models.LaneBatch, syncedAt time.Time) (int, error)
	// UpdateRemote refreshes the Jira data of one known issue, keeping its local state.
	UpdateRemote(ctx context.Context, issue models.RemoteIssue, at time.Time) error
	SetLocalState(ctx context.Context, key string, state models.IssueLocalState, taskID *string) error
	DeleteAll(ctx context.Context) error
}
