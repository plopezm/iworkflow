// Package outbound holds the ports the open_with use cases need from the infrastructure.
package outbound

import (
	"context"

	"iworkflow/internal/modules/open_with/domain/models"
)

// LauncherRepository stores the launchers. Get and Delete return errors.ErrLauncherNotFound for an unknown id;
// List returns the launchers in creation order.
type LauncherRepository interface {
	List(ctx context.Context) ([]models.Launcher, error)
	Get(ctx context.Context, id string) (models.Launcher, error)
	Save(ctx context.Context, launcher models.Launcher) error
	Delete(ctx context.Context, id string) error
}

// WorktreeLookup resolves the worktree directory of a task. It returns errors.ErrWorktreeNotFound when the
// task does not exist or has no worktree.
type WorktreeLookup interface {
	Path(ctx context.Context, taskID string) (string, error)
}

// ProcessStarter starts the process of a launch spec detached from the server. It returns once the process
// started, without waiting for it to finish, and wraps failures in errors.ErrLaunchFailed.
type ProcessStarter interface {
	Start(ctx context.Context, spec models.LaunchSpec) error
}

// IDGenerator creates the identifiers of new launchers.
type IDGenerator interface {
	NewID() string
}

// FileBrowser lists a folder of the server. An empty path means the home folder; a path to a file lists its
// folder. Entries come sorted (folders first). It wraps failures in errors.ErrBrowseFailed.
type FileBrowser interface {
	Browse(ctx context.Context, path string) (models.Listing, error)
}
