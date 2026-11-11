package application_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// fakeHooks keeps the hooks of the library in memory.
type fakeHooks struct{ hooks []models.Hook }

func (f *fakeHooks) List(context.Context) ([]models.Hook, error) { return f.hooks, nil }

func (f *fakeHooks) Get(_ context.Context, id string) (models.Hook, error) {
	for _, h := range f.hooks {
		if h.ID() == id {
			return h, nil
		}
	}
	return models.Hook{}, hookerrors.ErrHookNotFound
}

func (f *fakeHooks) Save(_ context.Context, hook models.Hook) error {
	for i, h := range f.hooks {
		if h.ID() == hook.ID() {
			f.hooks[i] = hook
			return nil
		}
	}
	f.hooks = append(f.hooks, hook)
	return nil
}

func (f *fakeHooks) Delete(_ context.Context, id string) error {
	for i, h := range f.hooks {
		if h.ID() == id {
			f.hooks = append(f.hooks[:i], f.hooks[i+1:]...)
			return nil
		}
	}
	return hookerrors.ErrHookNotFound
}

// fakeAssignments keeps the assignments in position order; the hooks of the joined lists come from hooks.
type fakeAssignments struct {
	items       []models.Assignment
	hooks       *fakeHooks
	reorderedTo []string
}

func (f *fakeAssignments) join(ctx context.Context, items []models.Assignment) []models.AssignedHook {
	var out []models.AssignedHook
	for _, a := range items {
		hook, err := f.hooks.Get(ctx, a.HookID())
		if err == nil {
			out = append(out, models.AssignedHook{Assignment: a, Hook: hook})
		}
	}
	return out
}

func (f *fakeAssignments) ListByWorkspace(ctx context.Context, workspaceID string) ([]models.AssignedHook, error) {
	var items []models.Assignment
	for _, a := range f.items {
		if a.WorkspaceID() == workspaceID {
			items = append(items, a)
		}
	}
	return f.join(ctx, items), nil
}

func (f *fakeAssignments) ListFor(ctx context.Context, workspaceID string, event models.Event, phase models.Phase) ([]models.AssignedHook, error) {
	var items []models.Assignment
	for _, a := range f.items {
		if a.WorkspaceID() == workspaceID && a.Event() == event && a.Phase() == phase {
			items = append(items, a)
		}
	}
	return f.join(ctx, items), nil
}

func (f *fakeAssignments) ListByHook(_ context.Context, hookID string) ([]models.Assignment, error) {
	var out []models.Assignment
	for _, a := range f.items {
		if a.HookID() == hookID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAssignments) Get(_ context.Context, id string) (models.Assignment, error) {
	for _, a := range f.items {
		if a.ID() == id {
			return a, nil
		}
	}
	return models.Assignment{}, hookerrors.ErrAssignmentNotFound
}

func (f *fakeAssignments) Save(_ context.Context, assignment models.Assignment) error {
	for i, a := range f.items {
		if a.ID() == assignment.ID() {
			f.items[i] = assignment
			return nil
		}
	}
	f.items = append(f.items, assignment)
	return nil
}

func (f *fakeAssignments) Delete(_ context.Context, id string) error {
	for i, a := range f.items {
		if a.ID() == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return hookerrors.ErrAssignmentNotFound
}

func (f *fakeAssignments) Reorder(_ context.Context, _ string, _ models.Event, _ models.Phase, ids []string) error {
	f.reorderedTo = ids
	return nil
}

// fakeSkips keeps the ids of the skipped assignments. It does not track the task: the tests use one task at a time.
type fakeSkips struct{ ids []string }

func (f *fakeSkips) SetSkipped(_ context.Context, _ string, assignmentID string, skipped bool) error {
	kept := f.ids[:0:0]
	for _, id := range f.ids {
		if id != assignmentID {
			kept = append(kept, id)
		}
	}
	if skipped {
		kept = append(kept, assignmentID)
	}
	f.ids = kept
	return nil
}

func (f *fakeSkips) SkippedAssignmentIDs(context.Context, string) ([]string, error) {
	return f.ids, nil
}

// fakeRuns records every save in order. ListByTask returns the saves of the task, so a run saved twice shows up
// twice, as an upsert history would.
type fakeRuns struct {
	saved      []models.HookRun
	stale      int64
	staleErr   error
	staleAt    time.Time
	staleCalls int
}

func (f *fakeRuns) Save(_ context.Context, run models.HookRun) error {
	f.saved = append(f.saved, run)
	return nil
}

func (f *fakeRuns) ListByTask(_ context.Context, taskID string) ([]models.HookRun, error) {
	var out []models.HookRun
	for _, run := range f.saved {
		if run.TaskID == taskID {
			out = append(out, run)
		}
	}
	return out, nil
}

// List returns the saves that match the filter, newest first.
func (f *fakeRuns) List(_ context.Context, filter outbound.RunFilter) ([]models.HookRun, error) {
	var out []models.HookRun
	for i := len(f.saved) - 1; i >= 0; i-- {
		run := f.saved[i]
		if filter.HookID != "" && run.HookID != filter.HookID || filter.WorkspaceID != "" && run.WorkspaceID != filter.WorkspaceID {
			continue
		}
		if filter.FailedOnly && (run.Status == models.RunOK || run.Status == models.RunSkipped || run.Status == models.RunRunning) {
			continue
		}
		out = append(out, run)
	}
	return out, nil
}

func (f *fakeRuns) LastFailures(context.Context) (map[string]models.HookRun, error) {
	failures := map[string]models.HookRun{}
	for _, run := range f.saved {
		if run.Status != models.RunOK && run.Status != models.RunSkipped && run.Status != models.RunRunning {
			failures[run.HookID] = run
		}
	}
	return failures, nil
}

func (f *fakeRuns) FailStaleRunning(_ context.Context, finishedAt time.Time) (int64, error) {
	f.staleCalls++
	f.staleAt = finishedAt
	if f.staleErr != nil {
		return 0, f.staleErr
	}
	return f.stale, nil
}

// fakeAgentSession answers with a fixed result or error. When wait is set it blocks until ctx ends, as a stuck
// agent would, and returns the ctx error.
type fakeAgentSession struct {
	result   outbound.AgentSessionResult
	err      error
	wait     bool
	requests []outbound.AgentSessionRequest
}

func (f *fakeAgentSession) Run(ctx context.Context, request outbound.AgentSessionRequest) (outbound.AgentSessionResult, error) {
	f.requests = append(f.requests, request)
	if f.wait {
		<-ctx.Done()
		return outbound.AgentSessionResult{}, fmt.Errorf("agent session: %w", ctx.Err())
	}
	return f.result, f.err
}

// fakeRunner answers by hook id and records the order of the calls.
type fakeRunner struct {
	outcomes map[string]models.RunOutcome
	errs     map[string]error
	calls    *[]string
}

func (f *fakeRunner) Run(_ context.Context, hook models.Hook, _ models.RunContext) (models.RunOutcome, error) {
	*f.calls = append(*f.calls, hook.ID())
	if err := f.errs[hook.ID()]; err != nil {
		return models.RunOutcome{}, err
	}
	if out, ok := f.outcomes[hook.ID()]; ok {
		return out, nil
	}
	return models.RunOutcome{Status: models.RunOK}, nil
}

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { f.now = f.now.Add(time.Second); return f.now }

type fakeIDs struct{ n int }

func (f *fakeIDs) NewID() string { f.n++; return "run" + strconv.Itoa(f.n) }
