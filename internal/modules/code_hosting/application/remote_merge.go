//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"sync/atomic"

	"iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/ports/inbound"
)

// RemoteMerge implements inbound.RemoteMergePort on top of MergeTaskMergeRequest. The tasks module
// needs it before this module exists and this module needs the tasks module, so the composition root
// creates it first and calls Bind once the merge use case is built. Until then a task integrates
// locally.
type RemoteMerge struct {
	merge atomic.Pointer[MergeTaskMergeRequest]
}

var _ inbound.RemoteMergePort = (*RemoteMerge)(nil)

// NewRemoteMerge builds an unbound remote merge.
func NewRemoteMerge() *RemoteMerge {
	return &RemoteMerge{}
}

// Bind connects the merge use case. Only the first binding is kept.
func (r *RemoteMerge) Bind(merge *MergeTaskMergeRequest) {
	r.merge.CompareAndSwap(nil, merge)
}

// MergeOpenRequest merges the open merge request of the task. Tasks without a remote provider, or
// without an open request, report RemoteMergeNoOpenRequest and no error.
func (r *RemoteMerge) MergeOpenRequest(ctx context.Context, taskID string) (inbound.RemoteMergeOutcome, error) {
	merge := r.merge.Load()
	if merge == nil {
		return inbound.RemoteMergeNoOpenRequest, nil
	}
	status, err := merge.ExecuteIfOpen(ctx, taskID)
	if err != nil {
		if errors.IsRemoteMergeNotApplicable(err) {
			return inbound.RemoteMergeNoOpenRequest, nil
		}
		return 0, err
	}
	if status == nil {
		return inbound.RemoteMergeNoOpenRequest, nil
	}
	return inbound.RemoteMergeMerged, nil
}
