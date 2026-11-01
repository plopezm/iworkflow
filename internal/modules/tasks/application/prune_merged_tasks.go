//nolint:wrapcheck // Port errors are returned unwrapped, as in the other use cases of the module.
package application

import (
	"context"
	"log/slog"
	"time"

	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// MergedTaskRetention is how long a merged task is kept before the prune deletes it.
const MergedTaskRetention = 7 * 24 * time.Hour

// PruneMergedTasks deletes the merged tasks that finished more than MergedTaskRetention ago, so a finished
// board does not grow the database forever. The rows go in one statement (events, attachment rows and graph
// rows follow by cascade); then the attachment folders are removed and the board is told. A merged task has
// no worktree left, so there is nothing else to clean.
type PruneMergedTasks struct {
	lifecycle *TaskLifecycle
	storage   outbound.AttachmentStorage
	enabled   outbound.MergedPrunePreference
}

// NewPruneMergedTasks builds the use case.
func NewPruneMergedTasks(lifecycle *TaskLifecycle, storage outbound.AttachmentStorage, enabled outbound.MergedPrunePreference) *PruneMergedTasks {
	return &PruneMergedTasks{lifecycle: lifecycle, storage: storage, enabled: enabled}
}

// Execute prunes at now and returns how many tasks were deleted; it deletes nothing while the user keeps the
// prune switched off. It is the MergedTasksPruner port.
func (u *PruneMergedTasks) Execute(ctx context.Context, now time.Time) (int, error) {
	enabled, err := u.enabled.Enabled(ctx)
	if err != nil {
		return 0, err
	}
	if !enabled {
		return 0, nil
	}
	ids, err := u.lifecycle.Repository().DeleteMergedBefore(ctx, now.Add(-MergedTaskRetention))
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := u.storage.DeleteTaskDir(ctx, id); err != nil {
			slog.Warn("could not delete the attachments of a pruned task", "task_id", id.String(), "error", err)
		}
		if err := u.lifecycle.NotifyRemoved(ctx, id); err != nil {
			slog.Warn("could not announce a pruned task", "task_id", id.String(), "error", err)
		}
	}
	return len(ids), nil
}
