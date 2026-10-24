//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// WorktreeCommitter commits the pending changes of a worktree. It is the part of the git port that the
// file editor needs; the full port belongs to slice E and satisfies it.
type WorktreeCommitter interface {
	// CommitAll commits every pending change. It returns false when there was nothing to commit.
	CommitAll(ctx context.Context, path string, message string) (bool, error)
}

// SaveTaskFile overwrites a file in the worktree of an open task and commits the change on its branch.
// It does not coordinate with a running agent: if the agent is mid-run, its own edits and this one end
// up in the same worktree and the next commit picks up both.
type SaveTaskFile struct {
	lifecycle *TaskLifecycle
	files     outbound.TaskFile
	committer WorktreeCommitter
}

// NewSaveTaskFile builds the use case.
func NewSaveTaskFile(lifecycle *TaskLifecycle, files outbound.TaskFile, committer WorktreeCommitter) *SaveTaskFile {
	return &SaveTaskFile{lifecycle: lifecycle, files: files, committer: committer}
}

// Execute writes the file and commits it. It returns whether a commit was created.
func (u *SaveTaskFile) Execute(ctx context.Context, id models.TaskID, rel string, content string) (bool, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return false, err
	}
	if !task.Status.HasWorktree() {
		return false, invalidState("guardar ficheros", task.Status)
	}
	if err := u.files.Write(ctx, task.WorktreePath, rel, content); err != nil {
		return false, err
	}
	return u.committer.CommitAll(ctx, task.WorktreePath, "edit "+rel+" from diff view")
}

// GetTaskFile reads a file from the worktree of an open task.
type GetTaskFile struct {
	lifecycle *TaskLifecycle
	files     outbound.TaskFile
}

// NewGetTaskFile builds the use case.
func NewGetTaskFile(lifecycle *TaskLifecycle, files outbound.TaskFile) *GetTaskFile {
	return &GetTaskFile{lifecycle: lifecycle, files: files}
}

// Execute reads the file.
func (u *GetTaskFile) Execute(ctx context.Context, id models.TaskID, rel string) (models.TaskFileContent, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return models.TaskFileContent{}, err
	}
	if !task.Status.HasWorktree() {
		return models.TaskFileContent{}, invalidState("leer ficheros", task.Status)
	}
	return u.files.Read(ctx, task.WorktreePath, rel)
}
