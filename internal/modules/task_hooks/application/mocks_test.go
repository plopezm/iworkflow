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

// fakeHooks keeps the hooks in memory, in position order. The field is positional in the rigs of the tests.
type fakeHooks struct{ hooks []models.Hook }

func (f *fakeHooks) List(_ context.Context, workspaceID string) ([]models.Hook, error) {
	var out []models.Hook
	for _, h := range f.hooks {
		if h.WorkspaceID() == workspaceID {
			out = append(out, h)
		}
	}
	return out, nil
}

func (f *fakeHooks) ListFor(_ context.Context, _ string, event models.Event, phase models.Phase) ([]models.Hook, error) {
	var out []models.Hook
	for _, h := range f.hooks {
		if h.Event() == event && h.Phase() == phase {
			out = append(out, h)
		}
	}
	return out, nil
}

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

// Reorder moves the hooks of the workspace to the end of the list in the order of ids; other workspaces keep
// their place.
func (f *fakeHooks) Reorder(_ context.Context, workspaceID string, ids []string) error {
	var others []models.Hook
	byID := map[string]models.Hook{}
	for _, h := range f.hooks {
		if h.WorkspaceID() == workspaceID {
			byID[h.ID()] = h
		} else {
			others = append(others, h)
		}
	}
	ordered := make([]models.Hook, 0, len(ids))
	for _, id := range ids {
		h, ok := byID[id]
		if !ok {
			return hookerrors.ErrHookNotFound
		}
		ordered = append(ordered, h)
	}
	f.hooks = append(others, ordered...)
	return nil
}

// fakeSkips keeps the ids of the skipped hooks. It does not track the task: the tests use one task at a time.
type fakeSkips struct{ ids []string }

func (f *fakeSkips) SetSkipped(_ context.Context, _ string, hookID string, skipped bool) error {
	kept := f.ids[:0:0]
	for _, id := range f.ids {
		if id != hookID {
			kept = append(kept, id)
		}
	}
	if skipped {
		kept = append(kept, hookID)
	}
	f.ids = kept
	return nil
}

func (f *fakeSkips) SkippedHookIDs(context.Context, string) ([]string, error) {
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
