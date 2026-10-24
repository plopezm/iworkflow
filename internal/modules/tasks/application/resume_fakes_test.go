package application_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// resumeSink is the event sink handed to the runtime. The tests compare it by identity.
type resumeSink struct{}

func (*resumeSink) Handle(context.Context, models.TaskID, models.AgentEvent) {}

// startCall is one Start the resume tests observe.
type startCall struct {
	id      models.TaskID
	cwd     string
	resume  bool
	options models.LaunchOptions
	sink    inbound.AgentEventSink
}

// sentCommand is one Send the resume tests observe.
type sentCommand struct {
	id      models.TaskID
	command models.AgentCommand
}

// resumeRuntime is the agent runtime fake of the resume tests. It is safe for concurrent use: the
// overlap tests call the use cases from several goroutines. Methods the tests do not use panic through
// the embedded nil interface, so an unexpected call fails loudly.
type resumeRuntime struct {
	outbound.AgentRuntime

	mu           sync.Mutex
	running      map[models.TaskID]bool
	capabilities models.AgentCapabilities
	// startErrs fails the start of the listed tasks.
	startErrs map[models.TaskID]error
	// sendErr fails every send.
	sendErr error
	// stopErr fails every stop; the process keeps running when it fails.
	stopErr error
	// startEntered is signalled when a start begins; startGate, when set, holds the start until it is closed.
	startEntered chan struct{}
	startGate    chan struct{}

	started []startCall
	sent    []sentCommand
	stopped []models.TaskID
}

func newResumeRuntime() *resumeRuntime {
	return &resumeRuntime{
		running:      map[models.TaskID]bool{},
		capabilities: models.FullAgentCapabilities(),
		startErrs:    map[models.TaskID]error{},
	}
}

func (r *resumeRuntime) IsRunning(_ context.Context, id models.TaskID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running[id]
}

func (r *resumeRuntime) Stop(_ context.Context, id models.TaskID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = append(r.stopped, id)
	if r.stopErr != nil {
		return r.stopErr
	}
	delete(r.running, id)
	return nil
}

func (r *resumeRuntime) Start(_ context.Context, id models.TaskID, cwd string, resume bool, options models.LaunchOptions, sink inbound.AgentEventSink) error {
	r.mu.Lock()
	r.started = append(r.started, startCall{id: id, cwd: cwd, resume: resume, options: options, sink: sink})
	entered := r.startEntered
	gate := r.startGate
	startErr := r.startErrs[id]
	r.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
	}
	if gate != nil {
		<-gate
	}
	if startErr != nil {
		return startErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running[id] = true
	return nil
}

func (r *resumeRuntime) Send(_ context.Context, id models.TaskID, command models.AgentCommand) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, sentCommand{id: id, command: command})
	return r.sendErr
}

func (r *resumeRuntime) RuntimeCapabilities(*models.RuntimeID) models.AgentCapabilities {
	return r.capabilities
}

// fakeLaunchSettings answers Prepare with options, or with err, and records the folders asked for.
type fakeLaunchSettings struct {
	options models.LaunchOptions
	err     error
	asked   []string
}

func (f *fakeLaunchSettings) Prepare(_ context.Context, cwd string) (models.LaunchOptions, error) {
	f.asked = append(f.asked, cwd)
	return f.options, f.err
}

// usageWrite is one SetUsageLimit call the resume tests observe.
type usageWrite struct {
	id    models.TaskID
	state models.UsageLimitState
}

// resumeRepository extends the shared fake with the calls the resume tests look at. It keeps the
// insertion order, so ListByStatus is deterministic, and it can fail the calls a test targets.
type resumeRepository struct {
	*fakeRepository

	order       []models.TaskID
	listedWith  [][]models.TaskStatus
	listErr     error
	usageErr    error
	usageWrites []usageWrite
}

func newResumeRepository(tasks ...models.Task) *resumeRepository {
	repo := &resumeRepository{fakeRepository: newFakeRepository(tasks...)}
	for _, task := range tasks {
		repo.order = append(repo.order, task.ID)
	}
	return repo
}

// ListByStatus returns the stored tasks in one of the statuses, in insertion order.
func (r *resumeRepository) ListByStatus(_ context.Context, statuses []models.TaskStatus) ([]models.Task, error) {
	r.listedWith = append(r.listedWith, statuses)
	if r.listErr != nil {
		return nil, r.listErr
	}
	var listed []models.Task
	for _, id := range r.order {
		task := r.tasks[id]
		if slices.Contains(statuses, task.Status) {
			listed = append(listed, task)
		}
	}
	return listed, nil
}

// SetUsageLimit records the write; the stored task is not changed, so the tests assert on the writes.
func (r *resumeRepository) SetUsageLimit(_ context.Context, id models.TaskID, state models.UsageLimitState) error {
	if r.usageErr != nil {
		return r.usageErr
	}
	r.usageWrites = append(r.usageWrites, usageWrite{id: id, state: state})
	return nil
}

// resumeFixture wires the resume use cases over the fakes and keeps the fakes for the assertions.
type resumeFixture struct {
	repo      *resumeRepository
	lifecycle *application.TaskLifecycle
	publisher *fakePublisher
	runtime   *resumeRuntime
	worktrees *fakeWorktree
	settings  *fakeLaunchSettings
	sink      *resumeSink
}

func newResumeFixture(tasks ...models.Task) *resumeFixture {
	repo := newResumeRepository(tasks...)
	lifecycle, publisher := newLifecycle(repo)
	return &resumeFixture{
		repo:      repo,
		lifecycle: lifecycle,
		publisher: publisher,
		runtime:   newResumeRuntime(),
		worktrees: &fakeWorktree{exists: true},
		settings:  &fakeLaunchSettings{},
		sink:      &resumeSink{},
	}
}

func (f *resumeFixture) resumeTask() *application.ResumeTask {
	return application.NewResumeTask(f.lifecycle, f.worktrees, f.runtime, f.settings, f.sink)
}

func (f *resumeFixture) rateLimited() *application.ResumeRateLimitedTasks {
	return application.NewResumeRateLimitedTasks(f.lifecycle, f.runtime, f.resumeTask())
}

// parkedTask is a task stopped by a usage limit with the given schedule.
func parkedTask(state models.UsageLimitState) models.Task {
	task := newTask(models.StatusRateLimited)
	task.UsageLimit = state
	return task
}

// instantAt returns fixedNow moved by offset, as a pointer for the schedule fields.
func instantAt(offset time.Duration) *time.Time {
	instant := fixedNow.Add(offset)
	return &instant
}

// runtimeID parses a runtime slug or fails the test.
func runtimeID(t *testing.T, raw string) *models.RuntimeID {
	t.Helper()
	parsed, err := models.ParseRuntimeID(raw)
	require.NoError(t, err)
	return &parsed
}

// waitForSignal fails the test when the signal does not arrive in time, instead of hanging.
func waitForSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the agent start")
	}
}
