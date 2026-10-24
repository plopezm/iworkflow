package runtime

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

const gentleShellID = "gentle-shell"

func mustRuntimeID(t *testing.T, raw string) models.RuntimeID {
	t.Helper()
	id, err := models.ParseRuntimeID(raw)
	require.NoError(t, err)
	return id
}

func newRouter(t *testing.T, defaultAdapter outbound.AgentRuntime, piAdapter outbound.AgentRuntime) *AgentRuntimeRouter {
	t.Helper()
	router, err := NewAgentRuntimeRouter(mustRuntimeID(t, gentleShellID), map[models.RuntimeID]outbound.AgentRuntime{
		mustRuntimeID(t, gentleShellID): defaultAdapter,
		mustRuntimeID(t, "pi"):          piAdapter,
	})
	require.NoError(t, err)
	return router
}

// startWith starts a task on the router; an empty runtime means no explicit runtime in LaunchOptions.
func startWith(t *testing.T, router *AgentRuntimeRouter, taskID models.TaskID, runtime string) error {
	t.Helper()
	options := models.LaunchOptions{}
	if runtime != "" {
		id := mustRuntimeID(t, runtime)
		options.Runtime = &id
	}
	return router.Start(context.Background(), taskID, "/tmp", false, options, &fakeEventSink{})
}

// Rust: agent_runtime_router.rs::tests::no_runtime_routes_to_the_default
func TestRouterWithoutRuntimeRoutesToTheDefault(t *testing.T) {
	defaultAdapter := &fakeAgentRuntime{}
	piAdapter := &fakeAgentRuntime{}
	router := newRouter(t, defaultAdapter, piAdapter)

	require.NoError(t, startWith(t, router, newTaskID(t), ""))

	assert.Equal(t, 1, defaultAdapter.startCount())
	assert.Equal(t, 0, piAdapter.startCount())
}

// Rust: agent_runtime_router.rs::tests::explicit_runtime_routes_to_that_adapter
func TestRouterWithExplicitRuntimeRoutesToThatAdapter(t *testing.T) {
	defaultAdapter := &fakeAgentRuntime{}
	piAdapter := &fakeAgentRuntime{}
	router := newRouter(t, defaultAdapter, piAdapter)

	require.NoError(t, startWith(t, router, newTaskID(t), "pi"))

	assert.Equal(t, 0, defaultAdapter.startCount())
	assert.Equal(t, 1, piAdapter.startCount())
}

// Rust: agent_runtime_router.rs::tests::unknown_runtime_is_unsupported_and_launches_nothing
func TestRouterUnknownRuntimeIsUnsupportedAndLaunchesNothing(t *testing.T) {
	defaultAdapter := &fakeAgentRuntime{}
	piAdapter := &fakeAgentRuntime{}
	router := newRouter(t, defaultAdapter, piAdapter)
	taskID := newTaskID(t)

	err := startWith(t, router, taskID, "codex")

	require.ErrorIs(t, err, werrors.Unsupported("runtime de agente desconocido: codex"))
	assert.Equal(t, 0, defaultAdapter.startCount())
	assert.Equal(t, 0, piAdapter.startCount())
	assert.ErrorIs(t, router.Send(context.Background(), taskID, models.AbortCommand{}), werrors.AgentNotRunning())
}

// Rust: agent_runtime_router.rs::tests::failed_start_is_not_recorded
func TestRouterFailedStartIsNotRecorded(t *testing.T) {
	failing := &fakeAgentRuntime{startErr: werrors.Runtime("boom")}
	router := newRouter(t, failing, &fakeAgentRuntime{})
	taskID := newTaskID(t)

	err := startWith(t, router, taskID, "")

	require.ErrorIs(t, err, werrors.Runtime("boom"))
	assert.False(t, router.IsRunning(context.Background(), taskID))
	assert.ErrorIs(t, router.Send(context.Background(), taskID, models.AbortCommand{}), werrors.AgentNotRunning())
	assert.Equal(t, models.MinimalAgentCapabilities(), router.Capabilities(taskID))
}

// Rust: agent_runtime_router.rs::tests::operations_delegate_to_the_recorded_adapter
func TestRouterOperationsDelegateToTheRecordedAdapter(t *testing.T) {
	pi := &fakeAgentRuntime{running: true, capabilities: models.FullAgentCapabilities()}
	router := newRouter(t, &fakeAgentRuntime{}, pi)
	taskID := newTaskID(t)
	require.NoError(t, startWith(t, router, taskID, "pi"))

	require.NoError(t, router.Send(context.Background(), taskID, models.AbortCommand{}))
	assert.True(t, router.IsRunning(context.Background(), taskID))
	assert.Equal(t, models.FullAgentCapabilities(), router.Capabilities(taskID))

	assert.Equal(t, 1, pi.sendCount())
	assert.Equal(t, 1, pi.isRunningCount())
	assert.Equal(t, 1, pi.capabilitiesCount())
}

// Rust: agent_runtime_router.rs::tests::subagents_delegate_to_the_recorded_adapter
func TestRouterSubagentsDelegateToTheRecordedAdapter(t *testing.T) {
	pi := &fakeAgentRuntime{subagents: []models.SubagentTask{{ID: "t1", Agent: "explore", Status: models.SubagentRunning}}}
	router := newRouter(t, &fakeAgentRuntime{}, pi)
	taskID := newTaskID(t)
	require.NoError(t, startWith(t, router, taskID, "pi"))

	subagents := router.Subagents(context.Background(), taskID)

	require.Len(t, subagents, 1)
	assert.Equal(t, "t1", subagents[0].ID)
	assert.Equal(t, 1, pi.subagentsCount())
}

// Rust: agent_runtime_router.rs::tests::subagents_of_an_unknown_task_are_empty
func TestRouterSubagentsOfAnUnknownTaskAreEmpty(t *testing.T) {
	router := newRouter(t, &fakeAgentRuntime{}, &fakeAgentRuntime{})

	assert.Empty(t, router.Subagents(context.Background(), newTaskID(t)))
}

// Rust: agent_runtime_router.rs::tests::stop_all_reaches_every_registered_runtime
func TestRouterStopAllReachesEveryRegisteredRuntimeAndClearsTheRecords(t *testing.T) {
	defaultAdapter := &fakeAgentRuntime{}
	pi := &fakeAgentRuntime{}
	router := newRouter(t, defaultAdapter, pi)
	taskID := newTaskID(t)
	require.NoError(t, startWith(t, router, taskID, "pi"))

	router.StopAll(context.Background())

	assert.Equal(t, 1, defaultAdapter.stopAllCount())
	assert.Equal(t, 1, pi.stopAllCount())
	assert.ErrorIs(t, router.Send(context.Background(), taskID, models.AbortCommand{}), werrors.AgentNotRunning())
}

// Rust: agent_runtime_router.rs::tests::stop_delegates_and_prunes_the_entry
func TestRouterStopDelegatesAndPrunesTheEntry(t *testing.T) {
	pi := &fakeAgentRuntime{}
	router := newRouter(t, &fakeAgentRuntime{}, pi)
	taskID := newTaskID(t)
	require.NoError(t, startWith(t, router, taskID, "pi"))

	require.NoError(t, router.Stop(context.Background(), taskID))

	assert.ErrorIs(t, router.Send(context.Background(), taskID, models.AbortCommand{}), werrors.AgentNotRunning())
	assert.Equal(t, models.MinimalAgentCapabilities(), router.Capabilities(taskID))
	// A second stop finds no entry and does not reach the adapter again.
	require.NoError(t, router.Stop(context.Background(), taskID))
	assert.Equal(t, 1, pi.stopCount())
}

// Rust: agent_runtime_router.rs::tests::failed_stop_keeps_the_entry_for_retry
func TestRouterFailedStopKeepsTheEntryForRetry(t *testing.T) {
	pi := &fakeAgentRuntime{stopErr: werrors.Runtime("no para"), running: true}
	router := newRouter(t, &fakeAgentRuntime{}, pi)
	taskID := newTaskID(t)
	require.NoError(t, startWith(t, router, taskID, "pi"))

	require.ErrorIs(t, router.Stop(context.Background(), taskID), werrors.Runtime("no para"))

	assert.True(t, router.IsRunning(context.Background(), taskID), "a failed stop must keep the record for a retry")
	assert.Equal(t, 1, pi.stopCount())
}

// Rust: agent_runtime_router.rs::tests::unknown_task_uses_safe_fallbacks
func TestRouterUnknownTaskUsesSafeFallbacks(t *testing.T) {
	router := newRouter(t, &fakeAgentRuntime{}, &fakeAgentRuntime{})
	taskID := newTaskID(t)

	assert.False(t, router.IsRunning(context.Background(), taskID))
	assert.ErrorIs(t, router.Send(context.Background(), taskID, models.AbortCommand{}), werrors.AgentNotRunning())
	assert.NoError(t, router.Stop(context.Background(), taskID))
	assert.Equal(t, models.MinimalAgentCapabilities(), router.Capabilities(taskID))
}

// Rust: agent_runtime_router.rs::tests::start_is_rejected_while_the_recorded_adapter_still_runs
func TestRouterStartIsRejectedWhileTheRecordedAdapterStillRuns(t *testing.T) {
	shell := &fakeAgentRuntime{running: true}
	pi := &fakeAgentRuntime{}
	router := newRouter(t, shell, pi)
	taskID := newTaskID(t)
	require.NoError(t, startWith(t, router, taskID, ""))

	err := startWith(t, router, taskID, "pi")

	require.ErrorIs(t, err, werrors.Runtime("el agente ya está en ejecución"))
	assert.Equal(t, 0, pi.startCount(), "a second adapter must not be started while the first runs")
}

// Rust: agent_runtime_router.rs::tests::start_is_allowed_again_once_the_recorded_adapter_stopped
func TestRouterStartIsAllowedAgainOnceTheRecordedAdapterStopped(t *testing.T) {
	shell := &fakeAgentRuntime{running: false}
	pi := &fakeAgentRuntime{capabilities: models.FullAgentCapabilities()}
	router := newRouter(t, shell, pi)
	taskID := newTaskID(t)
	require.NoError(t, startWith(t, router, taskID, ""))

	require.NoError(t, startWith(t, router, taskID, "pi"))

	assert.Equal(t, 1, pi.startCount())
	assert.Equal(t, models.FullAgentCapabilities(), router.Capabilities(taskID), "the new adapter now answers")
}

// Rust: agent_runtime_router.rs::tests::stop_does_not_prune_an_entry_recreated_by_a_concurrent_start
// A stop whose adapter is restarted by a concurrent start must keep the new entry. The restart runs
// inside the adapter's Stop, which only works if the router does not hold its lock during adapter calls.
func TestRouterStopDoesNotPruneAnEntryRecreatedByAConcurrentStart(t *testing.T) {
	taskID := newTaskID(t)
	var router *AgentRuntimeRouter
	shell := &fakeAgentRuntime{}
	shell.stopHook = func() {
		require.NoError(t, startWith(t, router, taskID, "pi"))
	}
	pi := &fakeAgentRuntime{capabilities: models.FullAgentCapabilities()}
	router = newRouter(t, shell, pi)
	require.NoError(t, startWith(t, router, taskID, ""))

	require.NoError(t, router.Stop(context.Background(), taskID))

	assert.Equal(t, models.FullAgentCapabilities(), router.Capabilities(taskID))
}

// Rust: agent_runtime_router.rs::tests::concurrent_starts_of_the_same_task_launch_a_single_agent
func TestRouterConcurrentStartsOfTheSameTaskLaunchASingleAgent(t *testing.T) {
	entered := make(chan struct{}, 2)
	gate := make(chan struct{}, 2)
	blocking := &fakeAgentRuntime{startEntered: entered, startGate: gate}
	router := newRouter(t, blocking, &fakeAgentRuntime{})
	taskID := newTaskID(t)

	firstResult := make(chan error, 1)
	go func() { firstResult <- startWith(t, router, taskID, "") }()
	select {
	case <-entered:
	case <-time.After(waitTimeout):
		require.FailNow(t, "the first start never reached the adapter")
	}

	secondErr := startWith(t, router, taskID, "")
	gate <- struct{}{}
	select {
	case err := <-firstResult:
		require.NoError(t, err)
	case <-time.After(waitTimeout):
		require.FailNow(t, "the first start did not return after release")
	}

	require.ErrorIs(t, secondErr, werrors.Runtime("el agente ya está en ejecución"))
	assert.Equal(t, 1, blocking.startCount())
	// The reservation is released when the start returns, so a new start is possible again.
	gate <- struct{}{}
	require.NoError(t, startWith(t, router, taskID, ""))
	assert.Equal(t, 2, blocking.startCount())
}

// Rust: agent_runtime_router.rs::tests::default_must_be_registered
func TestRouterDefaultMustBeRegistered(t *testing.T) {
	_, err := NewAgentRuntimeRouter(mustRuntimeID(t, gentleShellID), map[models.RuntimeID]outbound.AgentRuntime{
		mustRuntimeID(t, "pi"): &fakeAgentRuntime{},
	})

	require.ErrorIs(t, err, werrors.Unsupported("el runtime predeterminado 'gentle-shell' no está registrado"))
}

// Rule from source (Go port): a nil adapter would panic on the first call, so the router refuses it at
// construction with the same kind of error as a missing default.
func TestRouterRejectsANilAdapter(t *testing.T) {
	_, err := NewAgentRuntimeRouter(mustRuntimeID(t, gentleShellID), map[models.RuntimeID]outbound.AgentRuntime{
		mustRuntimeID(t, gentleShellID): nil,
	})

	require.ErrorIs(t, err, werrors.Unsupported("runtime sin adaptador: gentle-shell"))
}

// Rule from source: RuntimeCapabilities answers for the runtime a launch would use, before any start.
// nil means the default; an unknown runtime yields the minimal set without asking any adapter.
func TestRouterRuntimeCapabilitiesUseTheRequestedRuntime(t *testing.T) {
	defaultAdapter := &fakeAgentRuntime{runtimeCapabilities: models.MinimalAgentCapabilities()}
	pi := &fakeAgentRuntime{runtimeCapabilities: models.FullAgentCapabilities()}
	router := newRouter(t, defaultAdapter, pi)
	pid := mustRuntimeID(t, "pi")
	unknown := mustRuntimeID(t, "codex")

	assert.Equal(t, models.FullAgentCapabilities(), router.RuntimeCapabilities(&pid))
	assert.Equal(t, models.MinimalAgentCapabilities(), router.RuntimeCapabilities(nil))
	assert.Equal(t, models.MinimalAgentCapabilities(), router.RuntimeCapabilities(&unknown))
	assert.Equal(t, 1, pi.runtimeCapabilitiesCount(), "only the explicit pi call reaches pi")
	assert.Equal(t, 1, defaultAdapter.runtimeCapabilitiesCount(), "nil goes to the default only")
}

// Rule from source: the router never writes launch options to the logs. Options may carry model and
// thinking choices, so a failing start with a distinctive thinking value must leave it out of every log.
func TestRouterNeverLogsLaunchOptions(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	router := newRouter(t, &fakeAgentRuntime{startErr: werrors.Runtime("boom")}, &fakeAgentRuntime{})
	thinking := "thinking-secret-42"
	options := models.LaunchOptions{Thinking: &thinking}

	err := router.Start(context.Background(), newTaskID(t), "/tmp", false, options, &fakeEventSink{})

	require.ErrorIs(t, err, werrors.Runtime("boom"))
	assert.NotContains(t, logs.String(), thinking)
}

// fakeAgentRuntime is a hand-written outbound.AgentRuntime. Each test sets the answers it needs and
// reads the call counters afterwards. The mutex guards the counters because some tests call it from
// goroutines.
type fakeAgentRuntime struct {
	startErr     error
	startEntered chan struct{}
	startGate    chan struct{}
	stopErr      error
	stopHook     func()
	running      bool

	capabilities        models.AgentCapabilities
	runtimeCapabilities models.AgentCapabilities
	subagents           []models.SubagentTask

	mu                     sync.Mutex
	starts                 int
	sends                  int
	stops                  int
	stopAlls               int
	isRunnings             int
	subagentCalls          int
	capabilityCalls        int
	runtimeCapabilityCalls int
}

func (f *fakeAgentRuntime) Start(_ context.Context, _ models.TaskID, _ string, _ bool, _ models.LaunchOptions, _ inbound.AgentEventSink) error {
	f.mu.Lock()
	f.starts++
	f.mu.Unlock()
	if f.startEntered != nil {
		f.startEntered <- struct{}{}
	}
	if f.startGate != nil {
		<-f.startGate
	}
	return f.startErr
}

func (f *fakeAgentRuntime) Send(_ context.Context, _ models.TaskID, _ models.AgentCommand) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends++
	return nil
}

func (f *fakeAgentRuntime) Stop(_ context.Context, _ models.TaskID) error {
	f.mu.Lock()
	f.stops++
	f.mu.Unlock()
	if f.stopHook != nil {
		f.stopHook()
	}
	return f.stopErr
}

func (f *fakeAgentRuntime) StopAll(_ context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopAlls++
}

func (f *fakeAgentRuntime) IsRunning(_ context.Context, _ models.TaskID) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isRunnings++
	return f.running
}

func (f *fakeAgentRuntime) Subagents(_ context.Context, _ models.TaskID) []models.SubagentTask {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subagentCalls++
	return f.subagents
}

func (f *fakeAgentRuntime) Capabilities(_ models.TaskID) models.AgentCapabilities {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.capabilityCalls++
	return f.capabilities
}

func (f *fakeAgentRuntime) RuntimeCapabilities(_ *models.RuntimeID) models.AgentCapabilities {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runtimeCapabilityCalls++
	return f.runtimeCapabilities
}

func (f *fakeAgentRuntime) startCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts
}

func (f *fakeAgentRuntime) sendCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sends
}

func (f *fakeAgentRuntime) stopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stops
}

func (f *fakeAgentRuntime) stopAllCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopAlls
}

func (f *fakeAgentRuntime) isRunningCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.isRunnings
}

func (f *fakeAgentRuntime) subagentsCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subagentCalls
}

func (f *fakeAgentRuntime) capabilitiesCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.capabilityCalls
}

func (f *fakeAgentRuntime) runtimeCapabilitiesCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runtimeCapabilityCalls
}
