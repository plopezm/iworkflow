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

// shutdownStepTimeout is the budget of each shutdown step. The steps do not share one deadline: a slow step must not
// leave the next one, such as the agent event flush, without time to persist.
const shutdownStepTimeout = 10 * time.Second

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

// stopLoops cancels both loops and waits for them to return. Before Start it does nothing. The wait is bounded by the
// shutdown step that runs it.
func (a *App) stopLoops(context.Context) error {
	if a.loops.cancel == nil {
		return nil
	}
	a.loops.cancel()
	a.loops.wg.Wait()
	return nil
}

// shutdownStep is one step of Shutdown. Its name is the one the error of a step that runs out of time reports.
type shutdownStep struct {
	name string
	run  func(ctx context.Context) error
}

// runShutdownSteps runs the steps in order, each bounded by boundedStep, and returns their errors joined.
func runShutdownSteps(ctx context.Context, timeout time.Duration, steps []shutdownStep) error {
	errs := make([]error, 0, len(steps))
	for _, step := range steps {
		errs = append(errs, boundedStep(ctx, step.name, timeout, step.run))
	}
	return errors.Join(errs...)
}

// boundedStep runs step with a budget of its own: a context that keeps the values of ctx but not its deadline, expiring
// after timeout. It waits for the step until the step finishes, its budget ends, or ctx ends. ctx is the global upper
// bound of the whole shutdown, so an explicit cancel or an expired deadline of the caller stops the wait too. A step
// still running then is abandoned: its goroutine ends when the step returns, and the caller moves on.
//
// The result is nil only when the step has returned. A step that finished at the same moment the wait ends counts as
// finished, so a result never reports an unfinished step as done.
func boundedStep(ctx context.Context, name string, timeout time.Duration, step func(context.Context) error) error {
	stepCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	finished := make(chan error, 1)
	// The budget is released only once the step returns, so an abandoned step keeps running on a live context.
	go func() {
		defer cancel()
		finished <- step(stepCtx)
	}()
	select {
	case err := <-finished:
		return stepResult(name, err)
	case <-stepCtx.Done():
	case <-ctx.Done():
	}
	select {
	case err := <-finished:
		return stepResult(name, err)
	default:
	}
	reason := ctx.Err()
	if reason == nil {
		reason = stepCtx.Err()
	}
	return fmt.Errorf("%s did not finish before its deadline: %w", name, reason)
}

// stepResult names the error of a step that returned one.
func stepResult(name string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
