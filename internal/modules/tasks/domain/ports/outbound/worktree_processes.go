package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// WorktreeProcessCloser finds and closes the processes that use the files of a worktree: the ones whose working
// directory, executable or open files are inside it. It never touches the iworkflow server itself.
type WorktreeProcessCloser interface {
	// ListUsers returns the processes using worktreePath, without closing any. The agent of the task and its
	// children are left out: finishing stops the agent without asking.
	ListUsers(ctx context.Context, taskID models.TaskID, worktreePath string) ([]models.WorktreeProcess, error)
	// CloseUsers terminates the selected processes and returns how many it signalled. A selected process that
	// exited meanwhile, or whose pid now belongs to another process, is left alone.
	CloseUsers(ctx context.Context, worktreePath string, selected []models.WorktreeProcess) (int, error)
}
