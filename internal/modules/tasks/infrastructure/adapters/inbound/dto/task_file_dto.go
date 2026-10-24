package dto

import (
	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskFileDTO is a text file of a task worktree.
type TaskFileDTO struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// SaveTaskFileDTO is the result of saving a file: whether a commit was created.
type SaveTaskFileDTO struct {
	Committed bool `json:"committed"`
}

// TaskFileFromDomain maps a file read from a worktree.
func TaskFileFromDomain(file models.TaskFileContent) TaskFileDTO {
	return TaskFileDTO{Path: file.Path, Content: file.Content}
}

// SaveTaskFileResult maps the outcome of a save.
func SaveTaskFileResult(committed bool) SaveTaskFileDTO {
	return SaveTaskFileDTO{Committed: committed}
}
