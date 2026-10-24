//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
	"iworkflow/internal/modules/jira/infrastructure/adapters/outbound/persistence/gen"
)

// IssueRepository stores the imported issues and their lane memberships in jira_issues and
// jira_lane_issues.
//
// Writes come first in every transaction: a deferred SQLite transaction that reads and then writes
// can fail with SQLITE_BUSY on the upgrade, so the sync checks for existing keys with
// INSERT ... ON CONFLICT instead of reading them up front.
type IssueRepository struct {
	db      *sql.DB
	queries *gen.Queries
}

var _ outbound.IssueRepository = (*IssueRepository)(nil)

// NewIssueRepository builds the repository on an open database.
func NewIssueRepository(db *sql.DB) *IssueRepository {
	return &IssueRepository{db: db, queries: gen.New(db)}
}

// List returns every imported issue, most recently updated in Jira first.
func (r *IssueRepository) List(ctx context.Context) ([]models.Issue, error) {
	rows, err := r.queries.ListJiraIssues(ctx)
	if err != nil {
		return nil, dbError(err)
	}
	issues := make([]models.Issue, 0, len(rows))
	for _, row := range rows {
		issue, err := issueFromRow(row)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// Find returns nil and no error when the issue is not imported.
func (r *IssueRepository) Find(ctx context.Context, key string) (*models.Issue, error) {
	row, err := r.queries.GetJiraIssue(ctx, key)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, dbError(err)
	}
	issue, err := issueFromRow(row)
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

// LaneMemberships returns the issue keys of every lane, keyed by lane id.
func (r *IssueRepository) LaneMemberships(ctx context.Context) (map[string][]string, error) {
	rows, err := r.queries.ListJiraLaneMemberships(ctx)
	if err != nil {
		return nil, dbError(err)
	}
	memberships := make(map[string][]string)
	for _, row := range rows {
		memberships[row.LaneID] = append(memberships[row.LaneID], row.IssueKey)
	}
	return memberships, nil
}

// ReplaceSynced stores a sync in one transaction. A key repeated within the batches is stored once
// and each membership once, so a duplicated key cannot abort the whole sync.
func (r *IssueRepository) ReplaceSynced(ctx context.Context, batches []models.LaneBatch, syncedAt time.Time) (int, error) {
	issues := distinctIssues(batches)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, dbError(err)
	}
	defer func() { _ = tx.Rollback() }()
	queries := r.queries.WithTx(tx)
	synced := formatTimestamp(syncedAt)

	added := 0
	for _, issue := range issues {
		inserted, err := queries.InsertJiraIssueIfAbsent(ctx, insertParams(issue, synced))
		if err != nil {
			return 0, dbError(err)
		}
		if inserted > 0 {
			added++
			continue
		}
		if _, err := queries.RefreshJiraIssueRemote(ctx, refreshParams(issue, synced)); err != nil {
			return 0, dbError(err)
		}
	}
	for _, batch := range batches {
		if err := queries.DeleteJiraLaneMemberships(ctx, batch.LaneID); err != nil {
			return 0, dbError(err)
		}
		for _, key := range distinctKeys(batch.Issues) {
			if err := queries.InsertJiraLaneMembership(ctx, gen.InsertJiraLaneMembershipParams{
				LaneID:   batch.LaneID,
				IssueKey: key,
			}); err != nil {
				return 0, dbError(err)
			}
		}
	}
	// Untouched, converted or ignored issues are kept; plain New ones that no lane lists any more are retired.
	if err := queries.DeleteRetiredNewJiraIssues(ctx, models.IssueLocalStateNew.String()); err != nil {
		return 0, dbError(err)
	}
	if err := tx.Commit(); err != nil {
		return 0, dbError(err)
	}
	return added, nil
}

// UpdateRemote refreshes the Jira data of one known issue, keeping its local state.
func (r *IssueRepository) UpdateRemote(ctx context.Context, issue models.RemoteIssue, at time.Time) error {
	updated, err := r.queries.RefreshJiraIssueRemote(ctx, refreshParams(issue, formatTimestamp(at)))
	if err != nil {
		return dbError(err)
	}
	if updated == 0 {
		return jiraerrors.IssueNotFound(issue.Key)
	}
	return nil
}

// SetLocalState changes what iworkflow has done with the issue; taskID links or unlinks its task.
func (r *IssueRepository) SetLocalState(ctx context.Context, key string, state models.IssueLocalState, taskID *string) error {
	updated, err := r.queries.SetJiraIssueLocalState(ctx, gen.SetJiraIssueLocalStateParams{
		LocalState: state.String(),
		TaskID:     nullString(taskID),
		Key:        key,
	})
	if err != nil {
		return dbError(err)
	}
	if updated == 0 {
		return jiraerrors.IssueNotFound(key)
	}
	return nil
}

// DeleteAll removes every imported issue; their lane memberships go with them.
func (r *IssueRepository) DeleteAll(ctx context.Context) error {
	if err := r.queries.DeleteAllJiraIssues(ctx); err != nil {
		return dbError(err)
	}
	return nil
}

// distinctIssues returns each key once, keeping the first occurrence.
func distinctIssues(batches []models.LaneBatch) []models.RemoteIssue {
	seen := make(map[string]struct{})
	var issues []models.RemoteIssue
	for _, batch := range batches {
		for _, issue := range batch.Issues {
			if _, duplicate := seen[issue.Key]; duplicate {
				continue
			}
			seen[issue.Key] = struct{}{}
			issues = append(issues, issue)
		}
	}
	return issues
}

// distinctKeys returns the keys of one lane, each once, in order.
func distinctKeys(issues []models.RemoteIssue) []string {
	seen := make(map[string]struct{}, len(issues))
	keys := make([]string, 0, len(issues))
	for _, issue := range issues {
		if _, duplicate := seen[issue.Key]; duplicate {
			continue
		}
		seen[issue.Key] = struct{}{}
		keys = append(keys, issue.Key)
	}
	return keys
}

func insertParams(issue models.RemoteIssue, synced string) gen.InsertJiraIssueIfAbsentParams {
	return gen.InsertJiraIssueIfAbsentParams{
		Key:           issue.Key,
		Summary:       issue.Summary,
		Description:   issue.Description,
		Status:        issue.Status,
		IssueType:     issue.IssueType,
		Priority:      nullString(issue.Priority),
		Assignee:      nullString(issue.Assignee),
		Reporter:      nullString(issue.Reporter),
		Labels:        encodeLabels(issue.Labels),
		Url:           issue.URL,
		JiraUpdatedAt: formatTimestamp(issue.JiraUpdatedAt),
		LocalState:    models.IssueLocalStateNew.String(),
		SyncedAt:      synced,
	}
}

func refreshParams(issue models.RemoteIssue, synced string) gen.RefreshJiraIssueRemoteParams {
	return gen.RefreshJiraIssueRemoteParams{
		Summary:       issue.Summary,
		Description:   issue.Description,
		Status:        issue.Status,
		IssueType:     issue.IssueType,
		Priority:      nullString(issue.Priority),
		Assignee:      nullString(issue.Assignee),
		Reporter:      nullString(issue.Reporter),
		Labels:        encodeLabels(issue.Labels),
		Url:           issue.URL,
		JiraUpdatedAt: formatTimestamp(issue.JiraUpdatedAt),
		SyncedAt:      synced,
		Key:           issue.Key,
	}
}

// encodeLabels stores the labels as a JSON array; no labels is "[]", as the Rust version wrote it.
func encodeLabels(labels []string) string {
	if labels == nil {
		labels = []string{}
	}
	encoded, err := json.Marshal(labels)
	if err != nil {
		// A slice of strings always encodes; keep the stored form valid if it ever did not.
		return "[]"
	}
	return string(encoded)
}

func issueFromRow(row gen.JiraIssue) (models.Issue, error) {
	var labels []string
	if err := json.Unmarshal([]byte(row.Labels), &labels); err != nil {
		return models.Issue{}, jiraerrors.Repository(fmt.Sprintf("etiquetas inválidas en %s: %v", row.Key, err))
	}
	jiraUpdatedAt, err := parseTimestamp(row.JiraUpdatedAt)
	if err != nil {
		return models.Issue{}, err
	}
	syncedAt, err := parseTimestamp(row.SyncedAt)
	if err != nil {
		return models.Issue{}, err
	}
	return models.Issue{
		Remote: models.RemoteIssue{
			Key:           row.Key,
			Summary:       row.Summary,
			Description:   row.Description,
			Status:        row.Status,
			IssueType:     row.IssueType,
			Priority:      optionalString(row.Priority),
			Assignee:      optionalString(row.Assignee),
			Reporter:      optionalString(row.Reporter),
			Labels:        labels,
			URL:           row.Url,
			JiraUpdatedAt: jiraUpdatedAt,
		},
		LocalState: models.ParseIssueLocalState(row.LocalState),
		TaskID:     optionalString(row.TaskID),
		SyncedAt:   syncedAt,
	}, nil
}
