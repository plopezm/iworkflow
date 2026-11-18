// Package processes adapts the OS process table to the tasks ports.
package processes

import (
	"context"
	"fmt"
	"time"

	"iworkflow/internal/core/process"
)

// scanTimeout bounds the search for the processes of one worktree; lsof on a huge tree is the slow case.
const scanTimeout time.Duration = 60 * time.Second

// WorktreeCloser implements outbound.WorktreeProcessCloser on top of core/process.
type WorktreeCloser struct{}

// NewWorktreeCloser builds the adapter.
func NewWorktreeCloser() *WorktreeCloser { return &WorktreeCloser{} }

// CloseUsers terminates every process that uses files under worktreePath and returns how many it killed.
func (WorktreeCloser) CloseUsers(ctx context.Context, worktreePath string) (int, error) {
	scanCtx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()
	killed, err := process.KillUsers(scanCtx, worktreePath)
	if err != nil {
		return killed, fmt.Errorf("close the processes using the worktree: %w", err)
	}
	return killed, nil
}
