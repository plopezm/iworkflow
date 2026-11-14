//nolint:wrapcheck // Domain errors are returned unwrapped: the handler maps them to their user-visible text.
package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// OpenTaskPath opens, with the system, a file or folder that the agent mentioned in the chat. A relative path is
// resolved against the worktree of the task, so the link of an agent that talks about "src/main.go" lands on its file.
type OpenTaskPath struct {
	lifecycle *TaskLifecycle
	opener    outbound.PathOpener
	home      string
}

// NewOpenTaskPath builds the use case. home is the home folder that a leading ~ stands for.
func NewOpenTaskPath(lifecycle *TaskLifecycle, opener outbound.PathOpener, home string) *OpenTaskPath {
	return &OpenTaskPath{lifecycle: lifecycle, opener: opener, home: home}
}

// Execute resolves and opens the path.
func (u *OpenTaskPath) Execute(ctx context.Context, id models.TaskID, raw string) error {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	worktree := ""
	if task.Status.HasWorktree() {
		worktree = task.WorktreePath
	}
	path, err := models.ResolveOpenPath(raw, worktree, u.home)
	if err != nil {
		return err
	}
	if err := u.opener.Open(ctx, path); err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	return nil
}
