// Package persistence implements the tasks repository ports on SQLite through sqlc.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence/gen"
)

// Clock returns the current time. Timestamps written by the repository come from it.
type Clock func() time.Time

// TaskRepository stores the tasks and their events in SQLite.
//
// Every method runs in a single statement or in one BEGIN IMMEDIATE transaction, so a concurrent
// writer cannot slip in between a read and the write that depends on it. A lock that outlasts the
// connection busy timeout is reported as a transient error (werrors.Busy) for the retry decorator.
type TaskRepository struct {
	db      *sql.DB
	queries *gen.Queries
	clock   Clock
}

var _ outbound.TaskRepository = (*TaskRepository)(nil)

// NewTaskRepository builds the repository on an open database.
func NewTaskRepository(db *sql.DB, clock Clock) *TaskRepository {
	return &TaskRepository{db: db, queries: gen.New(db), clock: clock}
}

// dbError maps a driver error: a locked database is transient, anything else is a repository error.
func dbError(err error) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked") {
		return werrors.Busy(err.Error())
	}
	return werrors.Repository(err.Error())
}

// withImmediateTx runs fn in a BEGIN IMMEDIATE transaction on this repository's database.
func (r *TaskRepository) withImmediateTx(ctx context.Context, fn func(queries *gen.Queries) error) error {
	return withImmediateTx(ctx, r.db, fn)
}

func nullableWorkspace(workspaceID *string) any {
	if workspaceID == nil {
		return nil
	}
	return *workspaceID
}

func closedStatusNames() []string {
	names := make([]string, 0, len(models.AllTaskStatuses))
	for _, status := range models.AllTaskStatuses {
		if status.IsClosed() {
			names = append(names, status.String())
		}
	}
	return names
}

func liveStatusNames() []string {
	names := make([]string, 0, len(models.AllTaskStatuses))
	for _, status := range models.AllTaskStatuses {
		if status.HasLiveAgent() {
			names = append(names, status.String())
		}
	}
	return names
}

func resettableStatusNames() []string {
	names := make([]string, 0, len(models.AllTaskStatuses))
	for _, status := range models.AllTaskStatuses {
		if status.CanReset() {
			names = append(names, status.String())
		}
	}
	return names
}

func rowsToTasks(rows []gen.Task) ([]models.Task, error) {
	tasks := make([]models.Task, 0, len(rows))
	for _, row := range rows {
		task, err := rowToTask(row)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// Insert stores a new task.
func (r *TaskRepository) Insert(ctx context.Context, task models.Task) error {
	params, err := taskToInsertParams(task)
	if err != nil {
		return err
	}
	if err := r.queries.InsertTask(ctx, params); err != nil {
		return dbError(err)
	}
	return nil
}

// Find returns nil and no error when the task does not exist.
func (r *TaskRepository) Find(ctx context.Context, id models.TaskID) (*models.Task, error) {
	row, err := r.queries.GetTaskByID(ctx, id.String())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbError(err)
	}
	task, err := rowToTask(row)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// List returns the tasks, newest first; a nil workspace means every workspace.
func (r *TaskRepository) List(ctx context.Context, workspaceID *string) ([]models.Task, error) {
	rows, err := r.queries.ListTasks(ctx, nullableWorkspace(workspaceID))
	if err != nil {
		return nil, dbError(err)
	}
	return rowsToTasks(rows)
}

// ListActive returns the unfinished tasks, newest first (ties by id, descending).
func (r *TaskRepository) ListActive(ctx context.Context, workspaceID *string) ([]models.Task, error) {
	rows, err := r.queries.ListActiveTasks(ctx, gen.ListActiveTasksParams{
		ClosedStatuses: jsonArray(closedStatusNames()),
		WorkspaceID:    nullableWorkspace(workspaceID),
	})
	if err != nil {
		return nil, dbError(err)
	}
	return rowsToTasks(rows)
}

// ListDonePage returns a page of finished tasks ordered by completion, newest first, ties by id.
func (r *TaskRepository) ListDonePage(ctx context.Context, workspaceID *string, limit int64, offset int64) ([]models.Task, error) {
	rows, err := r.queries.ListDoneTasksPage(ctx, gen.ListDoneTasksPageParams{
		ClosedStatuses: jsonArray(closedStatusNames()),
		WorkspaceID:    nullableWorkspace(workspaceID),
		PageOffset:     offset,
		PageLimit:      limit,
	})
	if err != nil {
		return nil, dbError(err)
	}
	return rowsToTasks(rows)
}

// ListByStatus returns the tasks in one of the statuses.
func (r *TaskRepository) ListByStatus(ctx context.Context, statuses []models.TaskStatus) ([]models.Task, error) {
	names := make([]string, 0, len(statuses))
	for _, status := range statuses {
		names = append(names, status.String())
	}
	rows, err := r.queries.ListTasksByStatus(ctx, jsonArray(names))
	if err != nil {
		return nil, dbError(err)
	}
	return rowsToTasks(rows)
}

// UpdateStatus sets the status and its error. Leaving RateLimited clears resume_at, and a status without
// a live agent clears the subagent counters. A missing task is NotFound.
func (r *TaskRepository) UpdateStatus(ctx context.Context, id models.TaskID, status models.TaskStatus, errorText *string) error {
	return r.withImmediateTx(ctx, func(queries *gen.Queries) error {
		updated, err := queries.UpdateTaskStatus(ctx, gen.UpdateTaskStatusParams{
			Status:    status.String(),
			Error:     errorText,
			UpdatedAt: formatTimestamp(r.clock()),
			ID:        id.String(),
		})
		if err != nil {
			return dbError(err)
		}
		if updated == 0 {
			return werrors.NotFound(id.String())
		}
		if status != models.StatusRateLimited {
			if err := queries.ClearTaskResumeAt(ctx, id.String()); err != nil {
				return dbError(err)
			}
		}
		if !status.HasLiveAgent() {
			if err := queries.ClearTaskSubagents(ctx, id.String()); err != nil {
				return dbError(err)
			}
		}
		return nil
	})
}

// SetSubagentActivity sets the subagent counters while the task has a live agent.
func (r *TaskRepository) SetSubagentActivity(ctx context.Context, id models.TaskID, activity models.SubagentActivity) error {
	_, err := r.queries.SetTaskSubagentActivity(ctx, gen.SetTaskSubagentActivityParams{
		SubagentsActive:  clampCount(activity.Active),
		SubagentsWaiting: clampCount(activity.Waiting),
		ID:               id.String(),
		LiveStatuses:     jsonArray(liveStatusNames()),
	})
	if err != nil {
		return dbError(err)
	}
	return nil
}

// SetUsageLimit stores the resume schedule. The zero state clears it.
func (r *TaskRepository) SetUsageLimit(ctx context.Context, id models.TaskID, state models.UsageLimitState) error {
	params := gen.SetTaskUsageLimitParams{ResumeAttempts: clampCount(state.Attempts), ID: id.String()}
	if state.ResumeAt != nil {
		resumeAt := formatTimestamp(*state.ResumeAt)
		params.ResumeAt = &resumeAt
	}
	if err := r.queries.SetTaskUsageLimit(ctx, params); err != nil {
		return dbError(err)
	}
	return nil
}

// RecordDiffRange stores the final commit range.
func (r *TaskRepository) RecordDiffRange(ctx context.Context, id models.TaskID, diffRange models.DiffRange) error {
	base := diffRange.BaseSHA
	head := diffRange.HeadSHA
	if err := r.queries.SetTaskDiffRange(ctx, gen.SetTaskDiffRangeParams{BaseSha: &base, HeadSha: &head, ID: id.String()}); err != nil {
		return dbError(err)
	}
	return nil
}

// UpdateDetailsIfBacklog changes the editable fields only while the task is in backlog.
func (r *TaskRepository) UpdateDetailsIfBacklog(ctx context.Context, id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string, agent models.TaskAgentOptions) (bool, error) {
	updated, err := r.queries.UpdateTaskDetailsIfBacklog(ctx, r.detailsParams(id, title, prompt, priority, limits, baseBranch, agent))
	if err != nil {
		return false, dbError(err)
	}
	return updated > 0, nil
}

// UpdateDetailsAndRelatedIfBacklog changes the editable fields and replaces the related workspaces in one
// BEGIN IMMEDIATE transaction. When the related rows fail, the details roll back with them.
func (r *TaskRepository) UpdateDetailsAndRelatedIfBacklog(ctx context.Context, id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string, agent models.TaskAgentOptions, workspaceIDs []string) (bool, error) {
	var updated bool
	err := r.withImmediateTx(ctx, func(queries *gen.Queries) error {
		changed, err := queries.UpdateTaskDetailsIfBacklog(ctx, r.detailsParams(id, title, prompt, priority, limits, baseBranch, agent))
		if err != nil {
			return dbError(err)
		}
		if changed == 0 {
			return nil
		}
		updated = true
		return replaceRelatedProjects(ctx, queries, id, workspaceIDs)
	})
	if err != nil {
		return false, err
	}
	return updated, nil
}

// detailsParams maps the editable fields of a task to the columns of UpdateTaskDetailsIfBacklog.
func (r *TaskRepository) detailsParams(id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string, agent models.TaskAgentOptions) gen.UpdateTaskDetailsIfBacklogParams {
	var runtime *string
	if agent.Runtime != nil {
		s := agent.Runtime.String()
		runtime = &s
	}
	return gen.UpdateTaskDetailsIfBacklogParams{
		Title:              title,
		Prompt:             prompt,
		Priority:           priority.String(),
		BaseBranch:         baseBranch,
		MemoryLimitMb:      limitToColumn(limits.MemoryMB),
		CpuLimitPercent:    limitToColumn(limits.CPUPercent),
		AgentRuntime:       runtime,
		AgentModelProvider: agent.ModelProvider,
		AgentModelID:       agent.ModelID,
		AgentThinking:      agent.Thinking,
		AgentProfile:       agent.Profile,
		UpdatedAt:          formatTimestamp(r.clock()),
		ID:                 id.String(),
	}
}

// ClaimForStart moves a backlog task to starting and fixes its plan in one conditional update. Two
// concurrent claims of the same task cannot both succeed: the second one finds the status already changed.
func (r *TaskRepository) ClaimForStart(ctx context.Context, id models.TaskID, branch string, worktreePath string, baseBranch string) (bool, error) {
	updated, err := r.queries.ClaimTaskForStart(ctx, gen.ClaimTaskForStartParams{
		Branch:       branch,
		WorktreePath: worktreePath,
		BaseBranch:   baseBranch,
		UpdatedAt:    formatTimestamp(r.clock()),
		ID:           id.String(),
	})
	if err != nil {
		return false, dbError(err)
	}
	return updated > 0, nil
}

// RevertStartClaim moves a starting task back to backlog; false when the task is not starting.
func (r *TaskRepository) RevertStartClaim(ctx context.Context, id models.TaskID) (bool, error) {
	updated, err := r.queries.RevertTaskStartClaim(ctx, gen.RevertTaskStartClaimParams{
		UpdatedAt: formatTimestamp(r.clock()),
		ID:        id.String(),
	})
	if err != nil {
		return false, dbError(err)
	}
	return updated > 0, nil
}

// ResetToBacklog clears the execution fields and deletes the events, in one transaction.
func (r *TaskRepository) ResetToBacklog(ctx context.Context, id models.TaskID) (bool, error) {
	reset := false
	err := r.withImmediateTx(ctx, func(queries *gen.Queries) error {
		updated, err := queries.ResetTaskToBacklog(ctx, gen.ResetTaskToBacklogParams{
			UpdatedAt:          formatTimestamp(r.clock()),
			ID:                 id.String(),
			ResettableStatuses: jsonArray(resettableStatusNames()),
		})
		if err != nil {
			return dbError(err)
		}
		if updated == 0 {
			return nil
		}
		if err := queries.DeleteTaskEvents(ctx, id.String()); err != nil {
			return dbError(err)
		}
		reset = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return reset, nil
}

// AddPendingDialog adds a dialog while the task has a live agent and does not list it yet.
func (r *TaskRepository) AddPendingDialog(ctx context.Context, id models.TaskID, dialog models.PendingDialog) error {
	return r.withImmediateTx(ctx, func(queries *gen.Queries) error {
		row, err := queries.GetTaskByID(ctx, id.String())
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return dbError(err)
		}
		status, err := models.ParseTaskStatus(row.Status)
		if err != nil || !status.HasLiveAgent() {
			return nil
		}
		dialogs := decodePendingDialogsOrEmpty(row.PendingDialogs)
		for _, existing := range dialogs {
			if existing.ID == dialog.ID {
				return nil
			}
		}
		dialogs = append(dialogs, dialog)
		return r.writePendingDialogs(ctx, queries, id, dialogs)
	})
}

// RemovePendingDialog removes a dialog; it does not fail when the task or the dialog is gone.
func (r *TaskRepository) RemovePendingDialog(ctx context.Context, id models.TaskID, dialogID string) error {
	return r.withImmediateTx(ctx, func(queries *gen.Queries) error {
		row, err := queries.GetTaskByID(ctx, id.String())
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return dbError(err)
		}
		dialogs := decodePendingDialogsOrEmpty(row.PendingDialogs)
		remaining := make([]models.PendingDialog, 0, len(dialogs))
		for _, existing := range dialogs {
			if existing.ID != dialogID {
				remaining = append(remaining, existing)
			}
		}
		if len(remaining) == len(dialogs) {
			return nil
		}
		return r.writePendingDialogs(ctx, queries, id, remaining)
	})
}

func (r *TaskRepository) writePendingDialogs(ctx context.Context, queries *gen.Queries, id models.TaskID, dialogs []models.PendingDialog) error {
	payload, err := encodePendingDialogs(dialogs)
	if err != nil {
		return err
	}
	if err := queries.UpdateTaskPendingDialogs(ctx, gen.UpdateTaskPendingDialogsParams{
		PendingDialogs: payload,
		UpdatedAt:      formatTimestamp(r.clock()),
		ID:             id.String(),
	}); err != nil {
		return dbError(err)
	}
	return nil
}

// ClearPendingDialogs removes every pending dialog of the task.
func (r *TaskRepository) ClearPendingDialogs(ctx context.Context, id models.TaskID) error {
	return r.writePendingDialogs(ctx, r.queries, id, nil)
}

// DeleteIfBacklog deletes the task only while it is in backlog; its events cascade.
func (r *TaskRepository) DeleteIfBacklog(ctx context.Context, id models.TaskID) (bool, error) {
	deleted, err := r.queries.DeleteTaskIfBacklog(ctx, id.String())
	if err != nil {
		return false, dbError(err)
	}
	return deleted > 0, nil
}

// DeleteInactiveByWorkspace deletes all the tasks of a workspace when every one of them is inactive.
// When one is active it deletes nothing and returns false.
func (r *TaskRepository) DeleteInactiveByWorkspace(ctx context.Context, workspaceID string) ([]models.TaskID, bool, error) {
	var ids []models.TaskID
	deleted := false
	err := r.withImmediateTx(ctx, func(queries *gen.Queries) error {
		rows, err := queries.ListTaskIDStatusesByWorkspace(ctx, workspaceID)
		if err != nil {
			return dbError(err)
		}
		for _, row := range rows {
			status, parseErr := models.ParseTaskStatus(row.Status)
			if parseErr != nil || !status.IsInactive() {
				return nil
			}
		}
		for _, row := range rows {
			parsed, parseErr := uuid.Parse(row.ID)
			if parseErr == nil {
				ids = append(ids, models.TaskIDFromUUID(parsed))
			}
		}
		if err := queries.DeleteTasksByWorkspace(ctx, workspaceID); err != nil {
			return dbError(err)
		}
		deleted = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return ids, deleted, nil
}

// AppendEvent persists an event and returns it with its sequence number.
func (r *TaskRepository) AppendEvent(ctx context.Context, id models.TaskID, kind models.TaskEventKind) (models.TaskEvent, error) {
	payload, err := models.EncodeTaskEventKind(kind)
	if err != nil {
		return models.TaskEvent{}, err
	}
	createdAt := r.clock()
	row, err := r.queries.InsertTaskEvent(ctx, gen.InsertTaskEventParams{
		TaskID:    id.String(),
		Payload:   string(payload),
		CreatedAt: formatTimestamp(createdAt),
	})
	if err != nil {
		return models.TaskEvent{}, dbError(err)
	}
	return eventFromRow(row)
}

// Events returns the persisted events with sequence greater than afterSeq, in order.
func (r *TaskRepository) Events(ctx context.Context, id models.TaskID, afterSeq *int64) ([]models.TaskEvent, error) {
	var after int64
	if afterSeq != nil {
		after = *afterSeq
	}
	rows, err := r.queries.ListTaskEventsAfter(ctx, gen.ListTaskEventsAfterParams{TaskID: id.String(), AfterSeq: after})
	if err != nil {
		return nil, dbError(err)
	}
	events := make([]models.TaskEvent, 0, len(rows))
	for _, row := range rows {
		event, err := eventFromRow(row)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// eventFromRow maps a stored event row to the domain event.
func eventFromRow(row gen.TaskEvent) (models.TaskEvent, error) {
	taskID, err := uuid.Parse(row.TaskID)
	if err != nil {
		return models.TaskEvent{}, werrors.Repository(fmt.Sprintf("task_id inválido: %v", err))
	}
	kind, err := models.DecodeTaskEventKind([]byte(row.Payload))
	if err != nil {
		return models.TaskEvent{}, werrors.Repository(fmt.Sprintf("evento %d ilegible: %v", row.Seq, err))
	}
	createdAt, err := parseTimestamp(row.CreatedAt)
	if err != nil {
		return models.TaskEvent{}, werrors.Repository(fmt.Sprintf("fecha de evento inválida: %v", err))
	}
	seq := row.Seq
	return models.TaskEvent{
		TaskID:    models.TaskIDFromUUID(taskID),
		Seq:       &seq,
		Kind:      kind,
		CreatedAt: createdAt,
	}, nil
}

// jsonArray encodes a list of strings as one JSON array. The queries expand it with json_each, so every
// list filter is a single bound parameter and cannot shift the numbering of the other arguments.
func jsonArray(names []string) string {
	// json.Marshal cannot fail for a slice of strings, so its error is not checked.
	data, _ := json.Marshal(names)
	return string(data)
}
