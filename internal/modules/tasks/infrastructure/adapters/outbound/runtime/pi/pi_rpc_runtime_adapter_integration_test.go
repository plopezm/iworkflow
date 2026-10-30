//go:build unix

package pi

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// These tests run the adapter against real subprocesses through the login shell. They reuse the
// fixtures of the Rust suite (tests/fixtures) in place, so both suites exercise the same fake agents.

const (
	testWait        = 10 * time.Second
	testStopTimeout = 500 * time.Millisecond
	testPollEvery   = 20 * time.Millisecond
)

// collectingSink accumulates the events of one run and wakes the waiters on each arrival.
type collectingSink struct {
	mu     sync.Mutex
	events []models.AgentEvent
	// notify holds at most one pending wake-up; a waiter re-reads the snapshot after each one.
	notify chan struct{}
}

func newCollectingSink() *collectingSink {
	return &collectingSink{notify: make(chan struct{}, 1)}
}

// Handle records the event and wakes any waiter.
func (s *collectingSink) Handle(_ context.Context, _ models.TaskID, event models.AgentEvent) {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *collectingSink) snapshot() []models.AgentEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.events)
}

// waitFor blocks until an event matches predicate and returns the first match.
func (s *collectingSink) waitFor(t *testing.T, predicate func(models.AgentEvent) bool) models.AgentEvent {
	t.Helper()
	deadline := time.After(testWait)
	for {
		for _, event := range s.snapshot() {
			if predicate(event) {
				return event
			}
		}
		select {
		case <-s.notify:
		case <-deadline:
			t.Fatal("timed out waiting for an event")
		}
	}
}

// fixturePath returns the path of a fixture of the shared Rust suite, relative to the module root.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod not found above the test package")
		dir = parent
	}
	return filepath.Join(dir, "tests", "fixtures", name)
}

// pythonRuntime runs a python fixture as the agent, with the same unbuffered flag as the Rust suite.
func pythonRuntime(t *testing.T, script string) *RpcRuntimeAdapter {
	t.Helper()
	adapter := NewRpcRuntimeAdapterWithCommand("python3", []string{"-u", fixturePath(t, script)}).
		WithStopTimeout(testStopTimeout)
	t.Cleanup(func() { adapter.StopAll(context.Background()) })
	return adapter
}

// newTaskID returns a fresh task id for a test.
func newTaskID(t *testing.T) models.TaskID {
	t.Helper()
	taskID, err := models.NewTaskID()
	require.NoError(t, err)
	return taskID
}

func isNoticeContaining(event models.AgentEvent, text string) bool {
	notice, ok := event.(models.Notice)
	return ok && strings.Contains(notice.Message, text)
}

func isProcessExited(event models.AgentEvent) bool {
	_, ok := event.(models.ProcessExited)
	return ok
}

// requireEventually polls cond until it holds or the wait budget runs out.
func requireEventually(t *testing.T, cond func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for !cond() {
		require.True(t, time.Now().Before(deadline), message)
		time.Sleep(testPollEvery)
	}
}

// runToExit starts the agent and waits until its ProcessExited event arrives.
func runToExit(t *testing.T, adapter *RpcRuntimeAdapter, taskID models.TaskID, cwd string, resume bool) {
	t.Helper()
	sink := newCollectingSink()
	require.NoError(t, adapter.Start(context.Background(), taskID, cwd, resume, models.LaunchOptions{}, sink))
	sink.waitFor(t, isProcessExited)
}

func TestPiRpcRuntimeRunsPromptRoundTripAndStopsGracefully(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::runs_prompt_round_trip_and_stops_gracefully
	cwd, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	adapter := pythonRuntime(t, "fake_agent.py")
	sink := newCollectingSink()
	taskID := newTaskID(t)
	ctx := context.Background()

	require.NoError(t, adapter.Start(ctx, taskID, cwd, false, models.LaunchOptions{}, sink))
	boot := sink.waitFor(t, func(e models.AgentEvent) bool { return isNoticeContaining(e, "args=") })
	require.NoError(t, adapter.Send(ctx, taskID, models.PromptCommand{Message: "hola"}))
	sink.waitFor(t, func(e models.AgentEvent) bool { return e == models.RunSettled{} })

	assert.True(t, adapter.IsRunning(ctx, taskID))
	require.NoError(t, adapter.Stop(ctx, taskID))

	assert.False(t, adapter.IsRunning(ctx, taskID))
	events := sink.snapshot()
	assert.True(t, isNoticeContaining(boot, "cwd="+cwd))
	assert.True(t, isNoticeContaining(boot, "host=1"))
	assert.False(t, isNoticeContaining(boot, "--continue"))
	assert.Contains(t, events, models.RunStarted{})
	assert.Contains(t, events, models.TextDelta{Delta: "eco"})
	assert.Contains(t, events, models.AssistantMessage{Text: "eco: hola", Thinking: nil, Error: nil})
	assert.True(t, slices.ContainsFunc(events, func(e models.AgentEvent) bool {
		requested, ok := e.(models.DialogRequested)
		if !ok {
			return false
		}
		_, confirm := requested.Request.Kind.(models.ConfirmDialog)
		return confirm
	}))
	require.NotEmpty(t, events)
	assert.Equal(t, models.ProcessExited{Code: ptrTo(int32(0))}, events[len(events)-1])
}

func TestPiRpcRuntimeAppendsProviderAndModelFlagsWhenSet(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::appends_provider_and_model_flags_when_set
	adapter := pythonRuntime(t, "fake_agent.py")
	sink := newCollectingSink()
	taskID := newTaskID(t)
	ctx := context.Background()
	options := models.LaunchOptions{
		Model: &models.LaunchModel{Provider: "anthropic", Model: "claude-5"},
	}

	require.NoError(t, adapter.Start(ctx, taskID, t.TempDir(), false, options, sink))
	boot := sink.waitFor(t, func(e models.AgentEvent) bool { return isNoticeContaining(e, "args=") })
	require.NoError(t, adapter.Stop(ctx, taskID))

	assert.True(t, isNoticeContaining(boot, "--provider anthropic --model claude-5"))
}

func TestPiRpcRuntimeAppendsThinkingFlagWhenSetWithModel(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::appends_thinking_flag_when_set_with_model
	adapter := pythonRuntime(t, "fake_agent.py")
	sink := newCollectingSink()
	taskID := newTaskID(t)
	ctx := context.Background()
	options := models.LaunchOptions{
		Model:    &models.LaunchModel{Provider: "anthropic", Model: "claude-5"},
		Thinking: ptrTo("high"),
	}

	require.NoError(t, adapter.Start(ctx, taskID, t.TempDir(), false, options, sink))
	boot := sink.waitFor(t, func(e models.AgentEvent) bool { return isNoticeContaining(e, "args=") })
	require.NoError(t, adapter.Stop(ctx, taskID))

	assert.True(t, isNoticeContaining(boot, "--provider anthropic --model claude-5 --thinking high"))
}

func TestPiRpcRuntimeAppendsThinkingFlagWhenSetWithoutModel(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::appends_thinking_flag_when_set_without_model
	adapter := pythonRuntime(t, "fake_agent.py")
	sink := newCollectingSink()
	taskID := newTaskID(t)
	ctx := context.Background()
	options := models.LaunchOptions{Thinking: ptrTo("low")}

	require.NoError(t, adapter.Start(ctx, taskID, t.TempDir(), false, options, sink))
	boot := sink.waitFor(t, func(e models.AgentEvent) bool { return isNoticeContaining(e, "args=") })
	require.NoError(t, adapter.Stop(ctx, taskID))

	assert.True(t, isNoticeContaining(boot, "--thinking low"))
	assert.False(t, isNoticeContaining(boot, "--provider"))
}

func TestPiRpcRuntimeForwardsDialogAnswersAndReportsRejections(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::forwards_dialog_answers_and_reports_rejections
	adapter := pythonRuntime(t, "fake_agent.py")
	sink := newCollectingSink()
	taskID := newTaskID(t)
	ctx := context.Background()
	require.NoError(t, adapter.Start(ctx, taskID, t.TempDir(), true, models.LaunchOptions{}, sink))

	require.NoError(t, adapter.Send(ctx, taskID, models.AnswerDialogCommand{
		DialogID: "dlg",
		Answer:   models.ConfirmedAnswer{Confirmed: true},
	}))
	require.NoError(t, adapter.Send(ctx, taskID, models.SteerCommand{Message: "para"}))

	ack := sink.waitFor(t, func(e models.AgentEvent) bool { return isNoticeContaining(e, "respuesta") })
	rejected := sink.waitFor(t, func(e models.AgentEvent) bool {
		notice, ok := e.(models.Notice)
		return ok && notice.Level == models.NoticeError
	})
	boot := sink.waitFor(t, func(e models.AgentEvent) bool { return isNoticeContaining(e, "args=") })
	require.NoError(t, adapter.Stop(ctx, taskID))

	assert.True(t, isNoticeContaining(ack, `"confirmed": true, "id": "dlg"`))
	assert.True(t, isNoticeContaining(rejected, "steer rechazado: not streaming"))
	assert.True(t, isNoticeContaining(boot, "--continue"))
}

func TestPiRpcRuntimeKillsAgentThatIgnoresShutdown(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::kills_agent_that_ignores_shutdown
	adapter := pythonRuntime(t, "stubborn_agent.py")
	sink := newCollectingSink()
	taskID := newTaskID(t)
	ctx := context.Background()
	require.NoError(t, adapter.Start(ctx, taskID, t.TempDir(), false, models.LaunchOptions{}, sink))

	// Stop runs in its own goroutine so a hang fails this test instead of blocking it. The channel is
	// buffered, so the goroutine ends even when the test has already failed.
	stopped := make(chan error, 1)
	go func() { stopped <- adapter.Stop(ctx, taskID) }()
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(testWait):
		t.Fatal("stop did not finish in time")
	}

	events := sink.snapshot()
	require.NotEmpty(t, events)
	assert.True(t, isProcessExited(events[len(events)-1]))
}

func TestPiRpcRuntimeReportsExitAndUnregistersWhenProcessDies(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::reports_exit_and_unregisters_when_process_dies
	adapter := NewRpcRuntimeAdapterWithCommand("sh", []string{"-c", "exit 3"})
	sink := newCollectingSink()
	taskID := newTaskID(t)
	ctx := context.Background()

	require.NoError(t, adapter.Start(ctx, taskID, t.TempDir(), false, models.LaunchOptions{}, sink))
	exited := sink.waitFor(t, isProcessExited)
	// Rust sleeps 50 ms here, so the registry entry is gone; polling states the same condition without a fixed delay.
	requireEventually(t, func() bool { return !adapter.IsRunning(ctx, taskID) }, "agent is still registered after exit")

	assert.Equal(t, models.ProcessExited{Code: ptrTo(int32(3))}, exited)
	assert.ErrorIs(t, adapter.Send(ctx, taskID, models.AbortCommand{}), werrors.AgentNotRunning())
}

func TestPiRpcRuntimeMissingBinaryExitsWithTheShellNotFoundCode(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::missing_binary_exits_with_the_shell_not_found_code
	adapter := NewRpcRuntimeAdapter("/nonexistent/agent")
	sink := newCollectingSink()

	// With the login shell the launch itself does not fail: it is `exec` that cannot find the binary.
	require.NoError(t, adapter.Start(context.Background(), newTaskID(t), t.TempDir(), false, models.LaunchOptions{}, sink))

	exited := sink.waitFor(t, isProcessExited)
	assert.Equal(t, models.ProcessExited{Code: ptrTo(int32(127))}, exited)
}

func TestPiRpcRuntimeAgentDoesNotInheritTheServerEnvironment(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::agent_does_not_inherit_the_server_environment
	// The server variable is set here so the test proves it is filtered out, not merely absent.
	t.Setenv("CARGO_MANIFEST_DIR", "/server/only")
	const script = `printf '{"type":"extension_ui_request","id":"e","method":"notify","message":"env cargo=[%s] home=[%s]"}\n' "$CARGO_MANIFEST_DIR" "$HOME"; cat >/dev/null`
	adapter := NewRpcRuntimeAdapterWithCommand("sh", []string{"-c", script}).WithStopTimeout(testStopTimeout)
	t.Cleanup(func() { adapter.StopAll(context.Background()) })
	sink := newCollectingSink()
	taskID := newTaskID(t)
	ctx := context.Background()

	require.NoError(t, adapter.Start(ctx, taskID, t.TempDir(), false, models.LaunchOptions{}, sink))
	notice := sink.waitFor(t, func(e models.AgentEvent) bool { return isNoticeContaining(e, "env cargo=") })
	require.NoError(t, adapter.Stop(ctx, taskID))

	assert.True(t, isNoticeContaining(notice, "cargo=[]"))
	assert.True(t, isNoticeContaining(notice, "home=["+os.Getenv("HOME")+"]"))
}

// widgetScript is a python program that publishes one gentle-agents widget per payload and exits. When
// launched with --continue (resume) it publishes nothing.
func widgetScript(payloads ...string) []string {
	lines := []string{
		"import json, sys",
		"def emit(p):",
		"    sys.stdout.write(json.dumps({'type': 'extension_ui_request', 'id': 'w', " +
			"'method': 'setWidget', 'widgetKey': 'gentle-agents', 'widgetLines': [p]}) + '\\n')",
		"    sys.stdout.flush()",
		"if '--continue' not in sys.argv:",
		"    pass",
	}
	for _, payload := range payloads {
		lines = append(lines, "    emit("+strconv.Quote(payload)+")")
	}
	return []string{"-c", strings.Join(lines, "\n")}
}

const (
	runningSnapshot = `{"schema":"gentle-agents.activity/v1","summary":{"running":1,"queued":0,"waiting":0,"finished":0},"tasks":[{"summary":{"id":"t1","agent":"explore","status":"running"},"thread":{"items":[]}}]}`
	doneSnapshot    = `{"schema":"gentle-agents.activity/v1","summary":{"running":0,"queued":0,"waiting":0,"finished":1},"tasks":[{"summary":{"id":"t1","agent":"explore","status":"completed","turns":4},"thread":{"items":[{"kind":"text","text":"listo"}]}}]}`
)

func TestPiRpcRuntimeKeepsTheLatestValidSubagentSnapshotAfterTheAgentExits(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::keeps_the_latest_valid_subagent_snapshot_after_the_agent_exits
	adapter := NewRpcRuntimeAdapterWithCommand("python3", widgetScript(runningSnapshot, doneSnapshot, "no json"))
	t.Cleanup(func() { adapter.StopAll(context.Background()) })
	taskID := newTaskID(t)
	ctx := context.Background()

	runToExit(t, adapter, taskID, t.TempDir(), false)

	assert.False(t, adapter.IsRunning(ctx, taskID))
	tasks := adapter.Subagents(ctx, taskID)
	require.Len(t, tasks, 1)
	assert.Equal(t, models.SubagentCompleted, tasks[0].Status)
	assert.Equal(t, uint32(4), tasks[0].Turns)
	assert.Empty(t, adapter.Subagents(ctx, newTaskID(t)))
}

func TestPiRpcRuntimeANewStartDiscardsThePreviousSubagents(t *testing.T) {
	// Rust: tests/modules/tasks/pi_rpc_runtime_adapter_test.rs::a_new_start_discards_the_previous_subagents
	adapter := NewRpcRuntimeAdapterWithCommand("python3", widgetScript(doneSnapshot))
	t.Cleanup(func() { adapter.StopAll(context.Background()) })
	taskID := newTaskID(t)
	ctx := context.Background()
	cwd := t.TempDir()

	runToExit(t, adapter, taskID, cwd, false)
	require.Len(t, adapter.Subagents(ctx, taskID), 1)

	// Resuming: this time the agent publishes nothing, and the start discards what was there before.
	runToExit(t, adapter, taskID, cwd, true)
	assert.Empty(t, adapter.Subagents(ctx, taskID))
}
