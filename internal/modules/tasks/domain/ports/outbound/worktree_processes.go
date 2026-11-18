package outbound

import "context"

// WorktreeProcessCloser closes every process that is using the files of a worktree: whose working
// directory, executable or open files are inside it. It never touches the iworkflow server itself.
type WorktreeProcessCloser interface {
	// CloseUsers terminates the processes that use worktreePath and returns how many it signalled.
	CloseUsers(ctx context.Context, worktreePath string) (int, error)
}
