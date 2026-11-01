// Package application holds the open_with use cases: one struct per user action.
package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/domain/ports/outbound"
)

// ListLaunchers reads the configured launchers.
type ListLaunchers struct {
	repository outbound.LauncherRepository
}

// NewListLaunchers builds the use case.
func NewListLaunchers(repository outbound.LauncherRepository) *ListLaunchers {
	return &ListLaunchers{repository: repository}
}

// Execute returns the launchers in creation order.
func (u *ListLaunchers) Execute(ctx context.Context) ([]models.Launcher, error) {
	launchers, err := u.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list launchers: %w", err)
	}
	return launchers, nil
}

// CreateLauncherInput is the data of a new launcher. Args and Env are optional.
type CreateLauncherInput struct {
	Name string
	Path string
	Args []string
	Env  []models.EnvVar
}

// CreateLauncher stores a new launcher.
type CreateLauncher struct {
	repository outbound.LauncherRepository
	ids        outbound.IDGenerator
}

// NewCreateLauncher builds the use case.
func NewCreateLauncher(repository outbound.LauncherRepository, ids outbound.IDGenerator) *CreateLauncher {
	return &CreateLauncher{repository: repository, ids: ids}
}

// Execute validates the input and saves it. An invalid input stores nothing.
func (u *CreateLauncher) Execute(ctx context.Context, input CreateLauncherInput) (models.Launcher, error) {
	launcher, err := models.NewLauncher(u.ids.NewID(), input.Name, input.Path, input.Args, input.Env)
	if err != nil {
		return models.Launcher{}, err //nolint:wrapcheck // domain errors are user-visible and passed unchanged
	}
	if err := u.repository.Save(ctx, launcher); err != nil {
		return models.Launcher{}, fmt.Errorf("save launcher: %w", err)
	}
	return launcher, nil
}

// DeleteLauncher removes a launcher.
type DeleteLauncher struct {
	repository outbound.LauncherRepository
}

// NewDeleteLauncher builds the use case.
func NewDeleteLauncher(repository outbound.LauncherRepository) *DeleteLauncher {
	return &DeleteLauncher{repository: repository}
}

// Execute deletes the launcher; an unknown id is errors.ErrLauncherNotFound.
func (u *DeleteLauncher) Execute(ctx context.Context, id string) error {
	if err := u.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete launcher: %w", err)
	}
	return nil
}

// LaunchInput names the task whose worktree is the working directory and the launcher to run.
type LaunchInput struct {
	TaskID     string
	LauncherID string
}

// Launch runs a launcher in the worktree of a task.
type Launch struct {
	repository outbound.LauncherRepository
	worktrees  outbound.WorktreeLookup
	starter    outbound.ProcessStarter
}

// NewLaunch builds the use case.
func NewLaunch(repository outbound.LauncherRepository, worktrees outbound.WorktreeLookup, starter outbound.ProcessStarter) *Launch {
	return &Launch{repository: repository, worktrees: worktrees, starter: starter}
}

// Execute starts the application and returns the launcher that was run. Nothing starts when the launcher or the
// worktree is unknown.
func (u *Launch) Execute(ctx context.Context, input LaunchInput) (models.Launcher, error) {
	launcher, err := u.repository.Get(ctx, input.LauncherID)
	if err != nil {
		return models.Launcher{}, fmt.Errorf("find launcher: %w", err)
	}
	dir, err := u.worktrees.Path(ctx, input.TaskID)
	if err != nil {
		return models.Launcher{}, fmt.Errorf("find worktree: %w", err)
	}
	if err := u.starter.Start(ctx, launcher.SpecIn(dir)); err != nil {
		return models.Launcher{}, fmt.Errorf("start %s: %w", launcher.Name(), err)
	}
	return launcher, nil
}
