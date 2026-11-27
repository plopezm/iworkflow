//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// SyncOutcome summarises one sync.
type SyncOutcome struct {
	// Total is the number of distinct issues Jira returned for the synced lanes.
	Total int
	// Added is the number of issues that were not imported before.
	Added int
	// TruncatedLanes has the name of every lane that has more issues in Jira than the import limit; only the
	// first ones were stored.
	TruncatedLanes []string
	// FailedLanes has one message per lane whose query failed; those lanes keep their previous issues.
	FailedLanes []string
}

// SyncIssues imports the issues of every lane (each one a JQL query) and records the outcome on the
// connection. A lane whose query fails does not prevent the others from syncing.
type SyncIssues struct {
	connections outbound.ConnectionRepository
	issues      outbound.IssueRepository
	lanes       outbound.LaneRepository
	client      outbound.JiraClient
	clock       Clock
}

// NewSyncIssues builds the use case.
func NewSyncIssues(
	connections outbound.ConnectionRepository,
	issues outbound.IssueRepository,
	lanes outbound.LaneRepository,
	client outbound.JiraClient,
	clock Clock,
) *SyncIssues {
	return &SyncIssues{connections: connections, issues: issues, lanes: lanes, client: client, clock: clock}
}

// Execute syncs every lane.
func (u *SyncIssues) Execute(ctx context.Context) (SyncOutcome, error) {
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return SyncOutcome{}, err
	}
	lanes, err := u.lanes.List(ctx)
	if err != nil {
		return SyncOutcome{}, err
	}
	return u.run(ctx, connection, lanes)
}

// ExecuteLane syncs a single lane, e.g. right after it was created or its query changed.
func (u *SyncIssues) ExecuteLane(ctx context.Context, laneID string) (SyncOutcome, error) {
	connection, err := requireConnection(ctx, u.connections)
	if err != nil {
		return SyncOutcome{}, err
	}
	lane, err := u.lanes.Find(ctx, laneID)
	if err != nil {
		return SyncOutcome{}, err
	}
	if lane == nil {
		return SyncOutcome{}, jiraerrors.LaneNotFound(laneID)
	}
	return u.run(ctx, connection, []models.Lane{*lane})
}

func (u *SyncIssues) run(ctx context.Context, connection models.Connection, lanes []models.Lane) (SyncOutcome, error) {
	now := u.clock()
	var batches []models.LaneBatch
	var failures []error
	var failedLanes []string
	var truncatedLanes []string
	for _, lane := range lanes {
		result, err := u.client.Search(ctx, connection, lane.JQL)
		if err != nil {
			failedLanes = append(failedLanes, fmt.Sprintf("«%s»: %s", lane.Name, err.Error()))
			failures = append(failures, err)
			continue
		}
		if result.Truncated {
			truncatedLanes = append(truncatedLanes, lane.Name)
		}
		batches = append(batches, models.LaneBatch{LaneID: lane.ID, Issues: result.Issues})
	}
	var summary *string
	if joined := strings.Join(failedLanes, " · "); joined != "" {
		summary = &joined
	}
	if len(batches) == 0 && len(failures) > 0 {
		// Keep the failure visible on the connection; the original error wins over a bookkeeping failure.
		_ = u.connections.RecordSync(ctx, now, summary)
		return SyncOutcome{}, failures[0]
	}
	distinct := make(map[string]struct{})
	for _, batch := range batches {
		for _, issue := range batch.Issues {
			distinct[issue.Key] = struct{}{}
		}
	}
	added, err := u.issues.ReplaceSynced(ctx, batches, now)
	if err != nil {
		return SyncOutcome{}, err
	}
	u.refreshLeftBehind(ctx, connection, distinct, now)
	// The issues are already stored: failing to note the sync time must not fail the sync.
	_ = u.connections.RecordSync(ctx, now, summary)
	return SyncOutcome{Total: len(distinct), Added: added, FailedLanes: failedLanes, TruncatedLanes: truncatedLanes}, nil
}

const (
	// keyQueryBatch is how many issue keys one refresh query carries.
	keyQueryBatch = 100
	// keyQuerySplits bounds how many times rejected batches are split in one sync, so a Jira that is down is not hit
	// once per key. One bad key in a full batch needs about eight splits.
	keyQuerySplits = 16
)

// refreshLeftBehind refreshes the issues linked to a task that no lane returned in this sync, typically because
// they moved out of the lane's JQL (for example to Done); without it their status would stay frozen. Issues already
// stored as Done are not asked for again, and the rest are read with one `key in (...)` query per batch of keys, not
// one request per issue. A key Jira does not return keeps its stored copy. It is best effort: a failure never fails
// the sync.
//
// Tradeoff: an issue stored as Done that is reopened in Jira, while no lane matches it, keeps showing Done until a
// lane matches it again or it is refreshed by hand from the task panel.
func (u *SyncIssues) refreshLeftBehind(ctx context.Context, connection models.Connection, synced map[string]struct{}, now time.Time) {
	known, err := u.issues.List(ctx)
	if err != nil {
		return
	}
	var keys []string
	for _, issue := range known {
		if issue.LocalState != models.IssueLocalStateConverted || models.IsDoneStatus(issue.Remote.Status) {
			continue
		}
		key := issue.Key()
		if _, inLane := synced[key]; inLane || !models.IsValidIssueKey(key) {
			continue
		}
		keys = append(keys, key)
	}
	for start := 0; start < len(keys); start += keyQueryBatch {
		batch := keys[start:min(start+keyQueryBatch, len(keys))]
		splits := keyQuerySplits
		u.refreshKeys(ctx, connection, batch, now, &splits)
	}
}

// refreshKeys reads the keys with one query. When Jira rejects it (for example because one key does not exist or is
// not visible to the token) the keys are split in two and each half is tried, so one bad key does not freeze the
// others; splits is the remaining budget for that.
func (u *SyncIssues) refreshKeys(ctx context.Context, connection models.Connection, keys []string, now time.Time, splits *int) {
	quoted := make([]string, len(keys))
	for index, key := range keys {
		quoted[index] = `"` + key + `"`
	}
	result, err := u.client.Search(ctx, connection, "key in ("+strings.Join(quoted, ",")+")")
	if err != nil {
		if len(keys) > 1 && *splits > 0 && ctx.Err() == nil {
			*splits--
			middle := len(keys) / 2
			u.refreshKeys(ctx, connection, keys[:middle], now, splits)
			u.refreshKeys(ctx, connection, keys[middle:], now, splits)
		}
		return
	}
	for _, remote := range result.Issues {
		_ = u.issues.UpdateRemote(ctx, remote, now)
	}
}

// syncLane syncs one lane and returns the text of its failure, nil when it worked.
func (u *SyncIssues) syncLane(ctx context.Context, laneID string) *string {
	if _, err := u.ExecuteLane(ctx, laneID); err != nil {
		message := err.Error()
		return &message
	}
	return nil
}
