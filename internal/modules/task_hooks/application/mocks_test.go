package application_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

type fakeHooks struct{ hooks []models.Hook }

func (f *fakeHooks) List(context.Context, string) ([]models.Hook, error) { return f.hooks, nil }
func (f *fakeHooks) ListFor(_ context.Context, _ string, event models.Event, phase models.Phase) ([]models.Hook, error) {
	var out []models.Hook
	for _, h := range f.hooks {
		if h.Event() == event && h.Phase() == phase {
			out = append(out, h)
		}
	}
	return out, nil
}
func (f *fakeHooks) Get(context.Context, string) (models.Hook, error) { return models.Hook{}, nil }
func (f *fakeHooks) Save(context.Context, models.Hook) error          { return nil }
func (f *fakeHooks) Delete(context.Context, string) error             { return nil }
func (f *fakeHooks) Reorder(context.Context, string, []string) error  { return nil }

type fakeSkips struct{ ids []string }

func (f *fakeSkips) SetSkipped(context.Context, string, string, bool) error { return nil }
func (f *fakeSkips) SkippedHookIDs(context.Context, string) ([]string, error) {
	return f.ids, nil
}

type fakeRuns struct{ saved []models.HookRun }

func (f *fakeRuns) Save(_ context.Context, run models.HookRun) error {
	f.saved = append(f.saved, run)
	return nil
}
func (f *fakeRuns) ListByTask(context.Context, string) ([]models.HookRun, error) { return f.saved, nil }

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
