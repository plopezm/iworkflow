package dto

import (
	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskDiffDTO is the diff of a task against its base branch: the stat summary, the unified patch, whether the
// patch was cut short, and the commits of the task.
type TaskDiffDTO struct {
	Stat      string   `json:"stat"`
	Patch     string   `json:"patch"`
	Truncated bool     `json:"truncated"`
	Commits   []string `json:"commits"`
}

// TaskDiffFromDomain maps the diff of a task. A nil list of commits becomes an empty one, so it is sent as [] and not null.
func TaskDiffFromDomain(diff models.TaskDiff) TaskDiffDTO {
	commits := diff.Commits
	if commits == nil {
		commits = []string{}
	}
	return TaskDiffDTO{
		Stat:      diff.Stat,
		Patch:     diff.Patch,
		Truncated: diff.Truncated,
		Commits:   commits,
	}
}
