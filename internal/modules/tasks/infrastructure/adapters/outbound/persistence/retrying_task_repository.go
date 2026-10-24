//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// Retry bounds. They are the values of the Rust version and must not drift.
const (
	// InitialRetryDelay is the wait before the first retry.
	InitialRetryDelay = 25 * time.Millisecond
	// MaxRetryDelay caps the wait between retries (the backoff doubles up to here).
	MaxRetryDelay = time.Second
	// BoundedMaxElapsed is the total time the bounded policy keeps retrying.
	BoundedMaxElapsed = 30 * time.Second
	// warnEveryAttempts logs on the first retry and then every this many attempts.
	warnEveryAttempts = 10
)

// RetryPolicy is the backoff applied to transient errors.
type RetryPolicy struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	// MaxElapsed bounds the total retry time; nil means retry forever.
	MaxElapsed *time.Duration
}

// BoundedRetryPolicy gives up after BoundedMaxElapsed.
func BoundedRetryPolicy() RetryPolicy {
	elapsed := BoundedMaxElapsed
	return RetryPolicy{InitialDelay: InitialRetryDelay, MaxDelay: MaxRetryDelay, MaxElapsed: &elapsed}
}

// PersistentRetryPolicy retries transient errors without a time bound.
func PersistentRetryPolicy() RetryPolicy {
	return RetryPolicy{InitialDelay: InitialRetryDelay, MaxDelay: MaxRetryDelay}
}

// Sleep waits for d or until ctx is done, whichever comes first. It is injected so tests can run without waiting.
type Sleep func(ctx context.Context, d time.Duration) error

// ContextSleep is the real Sleep: it honours the context.
func ContextSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("retry wait cancelled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// RetryingTaskRepository retries every call of the repository while it fails with a transient error.
// Each port method is atomic, so retrying one call is safe; a retry never spans several calls.
type RetryingTaskRepository struct {
	inner  outbound.TaskRepository
	policy RetryPolicy
	clock  Clock
	sleep  Sleep
}

var _ outbound.TaskRepository = (*RetryingTaskRepository)(nil)

// NewRetryingTaskRepository decorates inner with the policy.
func NewRetryingTaskRepository(inner outbound.TaskRepository, policy RetryPolicy, clock Clock, sleep Sleep) *RetryingTaskRepository {
	return &RetryingTaskRepository{inner: inner, policy: policy, clock: clock, sleep: sleep}
}

// retry runs call until it succeeds, fails permanently, or the policy budget is spent.
func retry[T any](r *RetryingTaskRepository, ctx context.Context, operation string, call func() (T, error)) (T, error) {
	started := r.clock()
	delay := r.policy.InitialDelay
	attempt := 0
	for {
		value, err := call()
		if err == nil {
			if attempt > 0 {
				slog.Info("operation recovered after retries", "operation", operation, "attempts", attempt+1)
			}
			return value, nil
		}
		if !werrors.IsTransient(err) {
			return value, err
		}
		expired := r.policy.MaxElapsed != nil && r.clock().Sub(started) >= *r.policy.MaxElapsed
		if expired {
			slog.Error("retries exhausted", "operation", operation, "attempts", attempt+1, "error", err)
			return value, err
		}
		attempt++
		if attempt%warnEveryAttempts == 1 {
			slog.Warn("transient error, retrying", "operation", operation, "attempt", attempt, "error", err)
		}
		if sleepErr := r.sleep(ctx, delay); sleepErr != nil {
			return value, errors.Join(err, sleepErr)
		}
		delay = min(delay*2, r.policy.MaxDelay)
	}
}

// retryVoid adapts a call without a result to retry.
func retryVoid(r *RetryingTaskRepository, ctx context.Context, operation string, call func() error) error {
	_, err := retry(r, ctx, operation, func() (struct{}, error) { return struct{}{}, call() })
	return err
}

func (r *RetryingTaskRepository) Insert(ctx context.Context, task models.Task) error {
	return retryVoid(r, ctx, "insert", func() error { return r.inner.Insert(ctx, task) })
}

func (r *RetryingTaskRepository) Find(ctx context.Context, id models.TaskID) (*models.Task, error) {
	return retry(r, ctx, "find", func() (*models.Task, error) { return r.inner.Find(ctx, id) })
}

func (r *RetryingTaskRepository) List(ctx context.Context, workspaceID *string) ([]models.Task, error) {
	return retry(r, ctx, "list", func() ([]models.Task, error) { return r.inner.List(ctx, workspaceID) })
}

func (r *RetryingTaskRepository) ListActive(ctx context.Context, workspaceID *string) ([]models.Task, error) {
	return retry(r, ctx, "list_active", func() ([]models.Task, error) { return r.inner.ListActive(ctx, workspaceID) })
}

func (r *RetryingTaskRepository) ListDonePage(ctx context.Context, workspaceID *string, limit int64, offset int64) ([]models.Task, error) {
	return retry(r, ctx, "list_done_page", func() ([]models.Task, error) {
		return r.inner.ListDonePage(ctx, workspaceID, limit, offset)
	})
}

func (r *RetryingTaskRepository) ListByStatus(ctx context.Context, statuses []models.TaskStatus) ([]models.Task, error) {
	return retry(r, ctx, "list_by_status", func() ([]models.Task, error) { return r.inner.ListByStatus(ctx, statuses) })
}

func (r *RetryingTaskRepository) UpdateStatus(ctx context.Context, id models.TaskID, status models.TaskStatus, errorText *string) error {
	return retryVoid(r, ctx, "update_status", func() error { return r.inner.UpdateStatus(ctx, id, status, errorText) })
}

func (r *RetryingTaskRepository) UpdateDetailsIfBacklog(ctx context.Context, id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string) (bool, error) {
	return retry(r, ctx, "update_details_if_backlog", func() (bool, error) {
		return r.inner.UpdateDetailsIfBacklog(ctx, id, title, prompt, priority, limits, baseBranch)
	})
}

func (r *RetryingTaskRepository) ClaimForStart(ctx context.Context, id models.TaskID, branch string, worktreePath string, baseBranch string) (bool, error) {
	return retry(r, ctx, "claim_for_start", func() (bool, error) {
		return r.inner.ClaimForStart(ctx, id, branch, worktreePath, baseBranch)
	})
}

func (r *RetryingTaskRepository) RevertStartClaim(ctx context.Context, id models.TaskID) (bool, error) {
	return retry(r, ctx, "revert_start_claim", func() (bool, error) { return r.inner.RevertStartClaim(ctx, id) })
}

func (r *RetryingTaskRepository) ResetToBacklog(ctx context.Context, id models.TaskID) (bool, error) {
	return retry(r, ctx, "reset_to_backlog", func() (bool, error) { return r.inner.ResetToBacklog(ctx, id) })
}

func (r *RetryingTaskRepository) AddPendingDialog(ctx context.Context, id models.TaskID, dialog models.PendingDialog) error {
	return retryVoid(r, ctx, "add_pending_dialog", func() error { return r.inner.AddPendingDialog(ctx, id, dialog) })
}

func (r *RetryingTaskRepository) RemovePendingDialog(ctx context.Context, id models.TaskID, dialogID string) error {
	return retryVoid(r, ctx, "remove_pending_dialog", func() error { return r.inner.RemovePendingDialog(ctx, id, dialogID) })
}

func (r *RetryingTaskRepository) ClearPendingDialogs(ctx context.Context, id models.TaskID) error {
	return retryVoid(r, ctx, "clear_pending_dialogs", func() error { return r.inner.ClearPendingDialogs(ctx, id) })
}

func (r *RetryingTaskRepository) SetSubagentActivity(ctx context.Context, id models.TaskID, activity models.SubagentActivity) error {
	return retryVoid(r, ctx, "set_subagent_activity", func() error { return r.inner.SetSubagentActivity(ctx, id, activity) })
}

func (r *RetryingTaskRepository) RecordDiffRange(ctx context.Context, id models.TaskID, diffRange models.DiffRange) error {
	return retryVoid(r, ctx, "record_diff_range", func() error { return r.inner.RecordDiffRange(ctx, id, diffRange) })
}

func (r *RetryingTaskRepository) SetUsageLimit(ctx context.Context, id models.TaskID, state models.UsageLimitState) error {
	return retryVoid(r, ctx, "set_usage_limit", func() error { return r.inner.SetUsageLimit(ctx, id, state) })
}

func (r *RetryingTaskRepository) DeleteIfBacklog(ctx context.Context, id models.TaskID) (bool, error) {
	return retry(r, ctx, "delete_if_backlog", func() (bool, error) { return r.inner.DeleteIfBacklog(ctx, id) })
}

// deleteInactiveResult carries the two results of DeleteInactiveByWorkspace through retry.
type deleteInactiveResult struct {
	ids     []models.TaskID
	deleted bool
}

func (r *RetryingTaskRepository) DeleteInactiveByWorkspace(ctx context.Context, workspaceID string) ([]models.TaskID, bool, error) {
	result, err := retry(r, ctx, "delete_inactive_by_workspace", func() (deleteInactiveResult, error) {
		ids, deleted, callErr := r.inner.DeleteInactiveByWorkspace(ctx, workspaceID)
		return deleteInactiveResult{ids: ids, deleted: deleted}, callErr
	})
	if err != nil {
		return nil, false, err
	}
	return result.ids, result.deleted, nil
}

func (r *RetryingTaskRepository) AppendEvent(ctx context.Context, id models.TaskID, kind models.TaskEventKind) (models.TaskEvent, error) {
	return retry(r, ctx, "append_event", func() (models.TaskEvent, error) { return r.inner.AppendEvent(ctx, id, kind) })
}

func (r *RetryingTaskRepository) Events(ctx context.Context, id models.TaskID, afterSeq *int64) ([]models.TaskEvent, error) {
	return retry(r, ctx, "events", func() ([]models.TaskEvent, error) { return r.inner.Events(ctx, id, afterSeq) })
}
