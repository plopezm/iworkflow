// Package application holds the task_hooks use cases.
package application

import (
	"context"
	"fmt"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// RunHooksInput says which hooks to run: those of the workspace attached to the event and phase of Run, for
// the task described by Run.
type RunHooksInput struct {
	WorkspaceID string
	Run         models.RunContext
}

// RunHooksReport is what ran: every recorded run in order and the warnings of the failed hooks that did not
// block.
type RunHooksReport struct {
	Runs     []models.HookRun
	Warnings []string
}

// RunHooks runs the enabled hooks of a workspace for one task event and phase, in position order. A hook
// disabled for the task is recorded as skipped. A failed hook follows its policy: block stops the chain and
// fails the use case with ErrHookBlocked, warn is reported and the chain continues, ignore continues silently.
type RunHooks struct {
	assigned outbound.AssignmentRepository
	skips    outbound.SkipRepository
	runs     outbound.RunRepository
	scripts  outbound.ScriptRunner
	agents   outbound.AgentRunner
	clock    outbound.Clock
	ids      outbound.IDGenerator
}

// NewRunHooks builds the use case.
func NewRunHooks(
	assigned outbound.AssignmentRepository, skips outbound.SkipRepository, runs outbound.RunRepository,
	scripts outbound.ScriptRunner, agents outbound.AgentRunner, clock outbound.Clock, ids outbound.IDGenerator,
) *RunHooks {
	return &RunHooks{assigned: assigned, skips: skips, runs: runs, scripts: scripts, agents: agents, clock: clock, ids: ids}
}

// Execute runs the hooks. The report always lists what ran, also when the error is ErrHookBlocked.
func (u *RunHooks) Execute(ctx context.Context, in RunHooksInput) (RunHooksReport, error) {
	hooks, err := u.assigned.ListFor(ctx, in.WorkspaceID, in.Run.Event, in.Run.Phase)
	if err != nil {
		return RunHooksReport{}, fmt.Errorf("list hooks: %w", err)
	}
	skippedIDs, err := u.skips.SkippedAssignmentIDs(ctx, in.Run.TaskID)
	if err != nil {
		return RunHooksReport{}, fmt.Errorf("list skipped hooks: %w", err)
	}
	skipped := make(map[string]bool, len(skippedIDs))
	for _, id := range skippedIDs {
		skipped[id] = true
	}

	var report RunHooksReport
	for _, item := range hooks {
		if !item.Assignment.Enabled() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return report, fmt.Errorf("run hooks: %w", err)
		}
		record, err := u.runOne(ctx, item, in.Run, skipped[item.Assignment.ID()])
		if record.ID != "" {
			report.Runs = append(report.Runs, record)
		}
		if err != nil {
			return report, err
		}
		if record.Status == models.RunOK || record.Status == models.RunSkipped {
			continue
		}
		switch item.Assignment.OnFailure() {
		case models.FailureBlock:
			return report, fmt.Errorf("%w: hook %s %s", hookerrors.ErrHookBlocked, item.Hook.Name(), record.Status)
		case models.FailureWarn:
			report.Warnings = append(report.Warnings, fmt.Sprintf("hook %s %s", item.Hook.Name(), record.Status))
		case models.FailureIgnore:
		}
	}
	return report, nil
}

// runOne records and runs one hook. Its error is only about storage; a failed hook is a record, not an error.
func (u *RunHooks) runOne(ctx context.Context, item models.AssignedHook, run models.RunContext, skip bool) (models.HookRun, error) {
	hook := item.Hook
	record := models.HookRun{
		ID: u.ids.NewID(), TaskID: run.TaskID, HookID: hook.ID(), AssignmentID: item.Assignment.ID(),
		HookName: hook.Name(), WorkspaceID: item.Assignment.WorkspaceID(), Phase: run.Phase, Event: run.Event,
		Status: models.RunRunning, StartedAt: u.clock.Now(),
	}
	if skip {
		record.Status, record.FinishedAt = models.RunSkipped, record.StartedAt
		if err := u.runs.Save(ctx, record); err != nil {
			return record, fmt.Errorf("record skipped hook %s: %w", hook.ID(), err)
		}
		return record, nil
	}
	if err := u.runs.Save(ctx, record); err != nil {
		return record, fmt.Errorf("record hook %s: %w", hook.ID(), err)
	}

	outcome, runErr := u.dispatch(ctx, hook, run)
	if runErr != nil {
		outcome = models.RunOutcome{Status: models.RunErrored, ExitCode: -1, Output: runErr.Error()}
	}
	record.Status, record.ExitCode, record.Output, record.FinishedAt = outcome.Status, outcome.ExitCode, outcome.Output, u.clock.Now()
	// The run may have outlived the request that started it; the record is still written.
	if err := u.runs.Save(context.WithoutCancel(ctx), record); err != nil {
		return record, fmt.Errorf("record hook %s result: %w", hook.ID(), err)
	}
	return record, nil
}

func (u *RunHooks) dispatch(ctx context.Context, hook models.Hook, run models.RunContext) (models.RunOutcome, error) {
	runner := u.scripts
	if hook.Kind() == models.KindAgent {
		runner = u.agents
	}
	outcome, err := runner.Run(ctx, hook, run)
	if err != nil {
		return models.RunOutcome{}, fmt.Errorf("run hook %s: %w", hook.ID(), err)
	}
	return outcome, nil
}
