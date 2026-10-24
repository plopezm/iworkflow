// Package inbound holds the contracts this module offers to other modules.
package inbound

import "context"

// RemoteMergeOutcome is the result of integrating a task through its remote merge request.
type RemoteMergeOutcome uint8

const (
	// RemoteMergeMerged means the open merge request of the task branch was merged on the provider.
	RemoteMergeMerged RemoteMergeOutcome = iota + 1
	// RemoteMergeNoOpenRequest means there is nothing to merge remotely: the task is integrated
	// locally instead (no open request, or no remote provider for the task).
	RemoteMergeNoOpenRequest
)

// RemoteMergePort integrates a task through its remote merge request. The tasks module consumes it
// through its own outbound port, which the composition root adapts.
type RemoteMergePort interface {
	// MergeOpenRequest pushes the pending commits of the task branch to the open merge request and
	// merges it remotely.
	MergeOpenRequest(ctx context.Context, taskID string) (RemoteMergeOutcome, error)
}
