// Package processes adapts the OS process table to the tasks ports.
package processes

import (
	"context"
	"fmt"
	"slices"
	"time"

	"iworkflow/internal/core/process"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// scanTimeout bounds the search for the processes of one worktree; lsof on a huge tree is the slow case.
const scanTimeout time.Duration = 30 * time.Second

// closeTimeout bounds closing the selected processes: the grace period plus the rescans.
const closeTimeout time.Duration = 60 * time.Second

// WorktreeCloser implements outbound.WorktreeProcessCloser on top of core/process.
type WorktreeCloser struct {
	agents outbound.AgentProcessLocator
}

// NewWorktreeCloser builds the adapter. agents tells the pid of the live agent of a task, which is left out of
// the listing; nil leaves nothing out.
func NewWorktreeCloser(agents outbound.AgentProcessLocator) *WorktreeCloser {
	return &WorktreeCloser{agents: agents}
}

// ListUsers lists the processes using worktreePath, except the agent of the task and its descendants.
func (c *WorktreeCloser) ListUsers(ctx context.Context, taskID models.TaskID, worktreePath string) ([]models.WorktreeProcess, error) {
	scanCtx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()
	users, err := process.ListUsers(scanCtx, worktreePath)
	skip := c.agentTree(scanCtx, taskID)
	listed := make([]models.WorktreeProcess, 0, len(users))
	for _, user := range users {
		if _, isAgent := skip[user.PID]; isAgent {
			continue
		}
		listed = append(listed, models.WorktreeProcess{
			PID: user.PID, Name: user.Name, Path: user.Path, Reason: string(user.Reason), Started: user.Started,
		})
	}
	if err != nil {
		return listed, fmt.Errorf("list the processes using the worktree: %w", err)
	}
	return listed, nil
}

// agentTree returns the pid of the agent of the task and every process below it.
func (c *WorktreeCloser) agentTree(ctx context.Context, taskID models.TaskID) map[int]struct{} {
	skip := map[int]struct{}{}
	if c.agents == nil {
		return skip
	}
	pid, alive := c.agents.AgentPID(taskID)
	if !alive {
		return skip
	}
	skip[pid] = struct{}{}
	// A failed walk only means fewer descendants are left out; the agent itself is stopped anyway.
	descendants, _ := process.Descendants(ctx, pid)
	for _, descendant := range descendants {
		skip[descendant] = struct{}{}
	}
	return skip
}

// CloseUsers terminates the selected processes and returns how many it signalled.
func (c *WorktreeCloser) CloseUsers(ctx context.Context, worktreePath string, selected []models.WorktreeProcess) (int, error) {
	closeCtx, cancel := context.WithTimeout(ctx, closeTimeout)
	defer cancel()
	users := make([]process.User, 0, len(selected))
	for _, item := range selected {
		users = append(users, process.User{
			PID: item.PID, Name: item.Name, Path: item.Path, Reason: process.Reason(item.Reason), Started: item.Started,
		})
	}
	slices.SortFunc(users, func(a process.User, b process.User) int { return a.PID - b.PID })
	closed, err := process.Close(closeCtx, worktreePath, users)
	if err != nil {
		return closed, fmt.Errorf("close the processes using the worktree: %w", err)
	}
	return closed, nil
}
