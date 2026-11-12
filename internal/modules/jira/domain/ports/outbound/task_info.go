package outbound

import (
	"context"
	"time"
)

// TaskInfo reads what the Jira panel of a task needs from the tasks module. The composition root answers it, so
// the jira module never imports tasks.
type TaskInfo interface {
	Info(ctx context.Context, taskID string) (TaskSnapshot, error)
}

// TaskSnapshot is the part of a task a worklog is proposed from.
type TaskSnapshot struct {
	Title string
	// Finished is true when the task has ended its work: awaiting review or merged.
	Finished bool
	// AgentRunTime is the time the agent worked on the task, waiting for the user excluded.
	AgentRunTime time.Duration
	// Commits are the commits of the task branch as `git log --oneline` lines; empty when they cannot be read.
	Commits []string
}
