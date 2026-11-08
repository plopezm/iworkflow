package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// ReconcileHookRuns closes the runs a previous process left running. It belongs at the start of the process, before
// the server serves: at that point no hook of this process can be alive, so every running record is stale.
type ReconcileHookRuns struct {
	runs  outbound.RunRepository
	clock outbound.Clock
}

// NewReconcileHookRuns builds the use case.
func NewReconcileHookRuns(runs outbound.RunRepository, clock outbound.Clock) *ReconcileHookRuns {
	return &ReconcileHookRuns{runs: runs, clock: clock}
}

// Execute closes the stale runs at the current time and returns how many it closed.
func (u *ReconcileHookRuns) Execute(ctx context.Context) (int64, error) {
	closed, err := u.runs.FailStaleRunning(ctx, u.clock.Now())
	if err != nil {
		return 0, fmt.Errorf("reconcile hook runs: %w", err)
	}
	return closed, nil
}
