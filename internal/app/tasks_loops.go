package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/infrastructure/background"
)

// taskLoops are the background loops of the tasks module: the dependency graph and the usage-limit resume. They run
// in goroutines the application owns, on a context that outlives the HTTP server's drain, and Shutdown cancels them
// and waits for every one.
type taskLoops struct {
	// graphTick and resumeTick are the intervals of the loops. They default to the Rust ones.
	graphTick  time.Duration
	resumeTick time.Duration
	// graphSweeps and resumeSweeps count the sweeps the loops started. The tests read them to see that the loops run.
	graphSweeps  atomic.Int64
	resumeSweeps atomic.Int64
	// cancel stops both loops; nil until Start runs.
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// newTaskLoops returns the loops with the intervals of the Rust version.
func newTaskLoops() *taskLoops {
	return &taskLoops{graphTick: background.GraphTick, resumeTick: background.ResumeTick}
}

// countedSuccessors counts the sweeps of the graph loop before it runs the trigger.
type countedSuccessors struct {
	inner  inbound.SuccessorTrigger
	sweeps *atomic.Int64
}

// Trigger counts the sweep and starts the successors of the graph. Errors pass through unchanged: the loop only logs them.
func (c countedSuccessors) Trigger(ctx context.Context) (int, error) {
	c.sweeps.Add(1)
	return c.inner.Trigger(ctx) //nolint:wrapcheck // The loop logs the error as it is; it is not user-visible text.
}

// countedResumer counts the sweeps of the resume loop before it runs the resumer.
type countedResumer struct {
	inner  inbound.RateLimitedResumer
	sweeps *atomic.Int64
}

// Execute counts the sweep and resumes the due tasks. Errors pass through unchanged: the loop classifies them.
func (c countedResumer) Execute(ctx context.Context, now time.Time) (int, error) {
	c.sweeps.Add(1)
	return c.inner.Execute(ctx, now) //nolint:wrapcheck // The loop classifies the error by its kind; wrapping is not needed.
}

// Start runs the startup steps of the tasks module and then starts its background loops. It runs before the server
// listens, as the Rust main does: the tasks a restart interrupted are marked first, and then the worktrees that no
// open task owns are swept. A failed reconciliation stops the start-up; a failed sweep is only logged.
func (a *App) Start(ctx context.Context) error {
	if a.loops.cancel != nil {
		return errors.New("the tasks background loops are already running")
	}
	interrupted, err := a.tasks.Reconcile.Execute(ctx)
	if err != nil {
		return fmt.Errorf("reconcile tasks: %w", err)
	}
	if interrupted > 0 {
		a.logger.Info("tasks marked as interrupted after the restart", "count", interrupted)
	}
	removed, err := a.tasks.SweepWorktrees.Execute(ctx)
	switch {
	case err != nil:
		a.logger.Warn("cannot sweep the orphan worktrees", "error", err)
	case removed > 0:
		a.logger.Info("orphan worktree folders deleted", "count", removed)
	}
	a.startLoops(ctx)
	return nil
}

// startLoops runs the two loops in goroutines the application owns. Their context is detached from the start-up
// context, so the loops keep running while the HTTP server drains; Shutdown cancels them afterwards.
func (a *App) startLoops(ctx context.Context) {
	loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	a.loops.cancel = cancel
	a.loops.wg.Add(2)
	go a.runGraphLoop(loopCtx)
	go a.runResumeLoop(loopCtx)
}

// runGraphLoop evaluates the dependency graph until ctx ends. The error is only returned for an invalid interval.
func (a *App) runGraphLoop(ctx context.Context) {
	defer a.loops.wg.Done()
	trigger := countedSuccessors{inner: a.tasks.Successors, sweeps: &a.loops.graphSweeps}
	if err := background.RunGraphLoop(ctx, trigger, a.loops.graphTick); err != nil {
		a.logger.Error("the dependency graph loop stopped with an error", "error", err)
	}
}

// runResumeLoop resumes the rate-limited tasks until ctx ends, on the wall clock.
func (a *App) runResumeLoop(ctx context.Context) {
	defer a.loops.wg.Done()
	resumer := countedResumer{inner: a.tasks.ResumeRateLimited, sweeps: &a.loops.resumeSweeps}
	clock := func() time.Time { return time.Now().UTC() }
	if err := background.RunResumeLoop(ctx, resumer, clock, a.loops.resumeTick); err != nil {
		a.logger.Error("the usage-limit resume loop stopped with an error", "error", err)
	}
}

// stopLoops cancels both loops and waits for them to return, within ctx. Before Start it does nothing.
func (a *App) stopLoops(ctx context.Context) error {
	if a.loops.cancel == nil {
		return nil
	}
	a.loops.cancel()
	return boundedStep(ctx, "stop the background loops", a.loops.wg.Wait)
}

// boundedStep runs step and waits for it until ctx ends. A step still running at the deadline is abandoned and
// reported: its goroutine ends when the step returns, and the caller moves on to the next shutdown step.
func boundedStep(ctx context.Context, name string, step func()) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		step()
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s did not finish before the shutdown deadline: %w", name, ctx.Err())
	}
}
