//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"fmt"
	"strings"

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
	for _, lane := range lanes {
		issues, err := u.client.Search(ctx, connection, lane.JQL)
		if err != nil {
			failedLanes = append(failedLanes, fmt.Sprintf("«%s»: %s", lane.Name, err.Error()))
			failures = append(failures, err)
			continue
		}
		batches = append(batches, models.LaneBatch{LaneID: lane.ID, Issues: issues})
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
	// The issues are already stored: failing to note the sync time must not fail the sync.
	_ = u.connections.RecordSync(ctx, now, summary)
	return SyncOutcome{Total: len(distinct), Added: added, FailedLanes: failedLanes}, nil
}

// syncLane syncs one lane and returns the text of its failure, nil when it worked.
func (u *SyncIssues) syncLane(ctx context.Context, laneID string) *string {
	if _, err := u.ExecuteLane(ctx, laneID); err != nil {
		message := err.Error()
		return &message
	}
	return nil
}
