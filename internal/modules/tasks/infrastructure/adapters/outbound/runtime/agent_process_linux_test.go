//go:build linux

package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/env"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// These tests run real subprocesses through the login shell and read /proc to prove that no child
// survives. They are Linux-only, like the Rust lifecycle tests that check /proc.

const (
	testStopTimeout = 200 * time.Millisecond
	testExitTimeout = 300 * time.Millisecond
	pollInterval    = 20 * time.Millisecond
)

// newTestRuntime builds a runtime whose host environment is fixed: SHELL=/bin/sh, no systemd, and
// request ids that count up, so tests do not depend on the machine running them.
func newTestRuntime(t *testing.T, program string, stopTimeout time.Duration) *ProcessRuntime {
	t.Helper()
	runtime := NewProcessRuntime(program).WithStopTimeout(stopTimeout)
	runtime.exitTimeout = testExitTimeout
	runtime.hostEnv = func() []env.Var {
		return []env.Var{
			{Key: "HOME", Value: t.TempDir()},
			{Key: "SHELL", Value: "/bin/sh"},
			{Key: "PARENT_SECRET", Value: "parent"},
		}
	}
	runtime.systemdAvailable = func() bool { return false }
	var counter int
	runtime.newRequestID = func() string {
		counter++
		return "req-" + strconv.Itoa(counter)
	}
	return runtime
}

// startAgent starts an agent for a new task and stops it when the test ends, so a failing test
// cannot leave a process behind.
func startAgent(t *testing.T, runtime *ProcessRuntime, spec LaunchSpec, codec LineCodec) (models.TaskID, *fakeEventSink) {
	t.Helper()
	taskID := newTaskID(t)
	sink := &fakeEventSink{}
	require.NoError(t, runtime.Start(context.Background(), taskID, t.TempDir(), spec, models.ResourceLimits{}, codec, sink))
	t.Cleanup(func() {
		_ = runtime.Stop(context.Background(), taskID)
	})
	return taskID, sink
}

// registeredProcess returns the registry entry of a running task (test-only access, same package).
func registeredProcess(t *testing.T, runtime *ProcessRuntime, taskID models.TaskID) *agentProcess {
	t.Helper()
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	process, ok := runtime.processes[taskID]
	require.True(t, ok, "task is not registered")
	return process
}

// procStat is the state letter and process group of one /proc/<pid>/stat entry.
type procStat struct {
	state string
	pgrp  int
}

func readProcStat(pid int) (procStat, bool) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return procStat{}, false
	}
	// The command name sits in parentheses and may contain spaces: fields start after the last ')'.
	fields := strings.Fields(string(raw[strings.LastIndex(string(raw), ")")+1:]))
	if len(fields) < 3 {
		return procStat{}, false
	}
	pgrp, err := strconv.Atoi(fields[2])
	if err != nil {
		return procStat{}, false
	}
	return procStat{state: fields[0], pgrp: pgrp}, true
}

// processAlive reports whether pid is running. A zombie (exited, not yet reaped) does not count,
// as in the Rust helper that reads /proc.
func processAlive(pid int) bool {
	stat, ok := readProcStat(pid)
	return ok && stat.state != "Z"
}

// processEntryExists reports whether /proc still has an entry for pid, zombies included. It proves
// the leader was reaped: once reaped, the entry is gone.
func processEntryExists(pid int) bool {
	_, ok := readProcStat(pid)
	return ok
}

// liveGroupMembers lists the non-zombie processes whose process group is pgid.
func liveGroupMembers(t *testing.T, pgid int) []int {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	require.NoError(t, err)
	var members []int
	for _, entry := range entries {
		pid, convErr := strconv.Atoi(entry.Name())
		if convErr != nil {
			continue
		}
		stat, ok := readProcStat(pid)
		if ok && stat.pgrp == pgid && stat.state != "Z" {
			members = append(members, pid)
		}
	}
	return members
}

// readPIDFile waits until the agent has written a pid into path, then returns it.
func readPIDFile(t *testing.T, path string) int {
	t.Helper()
	var pid int
	require.Eventually(t, func() bool {
		raw, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		value, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if convErr != nil {
			return false
		}
		pid = value
		return true
	}, waitTimeout, pollInterval)
	return pid
}

// requireGroupEmptySoon waits for every non-zombie member of the group to be gone. Like requireDeadSoon it
// polls, because SIGKILL is delivered asynchronously and a member can still be dying for a moment.
func requireGroupEmptySoon(t *testing.T, pgid int) {
	t.Helper()
	require.Eventually(t, func() bool { return len(liveGroupMembers(t, pgid)) == 0 }, waitTimeout, pollInterval,
		"process group still has live members")
}

func requireDeadSoon(t *testing.T, pid int, message string) {
	t.Helper()
	require.Eventually(t, func() bool { return !processAlive(pid) }, waitTimeout, pollInterval, message)
}

// shellPath quotes a path for use inside an agent script.
func shellPath(path string) string {
	return env.ShellQuote(path)
}

// Rust: agent_process.rs::forced_stop_also_kills_the_processes_the_agent_launched
func TestForcedStopAlsoKillsTheProcessesTheAgentLaunched(t *testing.T) {
	runtime := newTestRuntime(t, "sh", testStopTimeout)
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	spec := LaunchSpec{AgentArgs: []string{"-c", "sleep 300 & echo $! > " + shellPath(pidFile) + "; wait"}}
	taskID, sink := startAgent(t, runtime, spec, &scriptedCodec{})
	child := readPIDFile(t, pidFile)
	require.True(t, processAlive(child))
	leader := registeredProcess(t, runtime, taskID).pgid
	require.Equal(t, leader, readProcStatOrFail(t, leader).pgrp, "agent must lead its own process group")

	started := time.Now()
	require.NoError(t, runtime.Stop(context.Background(), taskID))

	assert.GreaterOrEqual(t, time.Since(started), testStopTimeout, "stop must escalate only after the stop timeout")
	assert.False(t, runtime.IsRunning(taskID))
	// SIGKILL is delivered asynchronously: the leader is reaped before Stop returns, but a group member
	// can still be dying for a moment under load. The invariant is that it dies, not that it already did.
	requireDeadSoon(t, child, "the child the agent launched survived stop")
	requireGroupEmptySoon(t, leader)
	assert.False(t, processEntryExists(leader), "agent leader was not reaped")
	assert.Equal(t, []*int32{nil}, sink.exitCodes(), "forced stop must report ProcessExited without a code")
}

// Rust: agent_process.rs::stop_all_stops_every_running_agent_and_their_children
func TestStopAllStopsEveryRunningAgentAndTheirChildren(t *testing.T) {
	runtime := newTestRuntime(t, "sh", testStopTimeout)
	firstPID := filepath.Join(t.TempDir(), "child.pid")
	secondPID := filepath.Join(t.TempDir(), "child.pid")
	firstTask, firstSink := startAgent(t, runtime,
		LaunchSpec{AgentArgs: []string{"-c", "sleep 300 & echo $! > " + shellPath(firstPID) + "; wait"}}, &scriptedCodec{})
	secondTask, secondSink := startAgent(t, runtime,
		LaunchSpec{AgentArgs: []string{"-c", "sleep 300 & echo $! > " + shellPath(secondPID) + "; wait"}}, &scriptedCodec{})
	firstChild := readPIDFile(t, firstPID)
	secondChild := readPIDFile(t, secondPID)
	firstLeader := registeredProcess(t, runtime, firstTask).pgid
	secondLeader := registeredProcess(t, runtime, secondTask).pgid

	runtime.StopAll(context.Background())

	assert.False(t, runtime.IsRunning(firstTask))
	assert.False(t, runtime.IsRunning(secondTask))
	requireDeadSoon(t, firstChild, "the first agent's child survived StopAll")
	requireDeadSoon(t, secondChild, "the second agent's child survived StopAll")
	requireGroupEmptySoon(t, firstLeader)
	requireGroupEmptySoon(t, secondLeader)
	assert.Equal(t, []*int32{nil}, firstSink.exitCodes())
	assert.Equal(t, []*int32{nil}, secondSink.exitCodes())
}

// Rust: agent_process.rs::an_agent_that_exits_on_its_own_does_not_leave_its_children_behind
func TestAgentThatExitsOnItsOwnDoesNotLeaveItsChildrenBehind(t *testing.T) {
	runtime := newTestRuntime(t, "sh", time.Second)
	dir := t.TempDir()
	leaderFile := filepath.Join(dir, "leader.pid")
	childFile := filepath.Join(dir, "child.pid")
	script := "echo $$ > " + shellPath(leaderFile) + "; " +
		"sleep 300 </dev/null >/dev/null 2>&1 & echo $! > " + shellPath(childFile)
	taskID, sink := startAgent(t, runtime, LaunchSpec{AgentArgs: []string{"-c", script}}, &scriptedCodec{})
	leader := readPIDFile(t, leaderFile)
	child := readPIDFile(t, childFile)

	require.Eventually(t, func() bool { return len(sink.exitCodes()) == 1 }, waitTimeout, pollInterval)

	assert.False(t, runtime.IsRunning(taskID), "an exited agent must be deregistered")
	assert.Equal(t, []*int32{int32Ptr(0)}, sink.exitCodes())
	requireDeadSoon(t, child, "the child of an exited agent survived the sweep")
	requireGroupEmptySoon(t, leader)
	assert.False(t, processEntryExists(leader), "agent leader was not reaped")
}

// Rule from source: StopAll with nothing running does nothing and returns.
func TestStopAllWithoutAgentsIsANoOp(t *testing.T) {
	runtime := newTestRuntime(t, "sh", testStopTimeout)

	runtime.StopAll(context.Background())

	assert.False(t, runtime.IsRunning(newTaskID(t)))
}

// Rule from source: stop closes stdin, the agent sees EOF and exits by itself. Graceful stop must not
// escalate: the exit code is reported and the stop returns long before the forced path would.
func TestGracefulStopLetsTheAgentExitOnEOFWithoutForcing(t *testing.T) {
	runtime := newTestRuntime(t, "cat", 10*time.Second)
	taskID, sink := startAgent(t, runtime, LaunchSpec{}, &scriptedCodec{})
	leader := registeredProcess(t, runtime, taskID).pgid
	require.True(t, runtime.IsRunning(taskID))

	started := time.Now()
	require.NoError(t, runtime.Stop(context.Background(), taskID))

	assert.Less(t, time.Since(started), 5*time.Second, "graceful stop must not wait for the stop timeout")
	assert.False(t, runtime.IsRunning(taskID))
	assert.Equal(t, []*int32{int32Ptr(0)}, sink.exitCodes(), "EOF must end the agent with exit code 0, not a kill")
	requireGroupEmptySoon(t, leader)
	assert.False(t, processEntryExists(leader), "agent leader was not reaped")
}

// Rule from source: a second stop, or a stop after the agent exited, is a no-op and delivers no event.
func TestStopIsIdempotentAndDeliversProcessExitedOnce(t *testing.T) {
	runtime := newTestRuntime(t, "cat", 10*time.Second)
	taskID, sink := startAgent(t, runtime, LaunchSpec{}, &scriptedCodec{})

	require.NoError(t, runtime.Stop(context.Background(), taskID))
	require.NoError(t, runtime.Stop(context.Background(), taskID))

	assert.False(t, runtime.IsRunning(taskID))
	assert.Equal(t, []*int32{int32Ptr(0)}, sink.exitCodes(), "ProcessExited must be delivered exactly once")
}

// Rule from source: a task with a live agent cannot be started again; the rejection happens before any
// spawn, and the running agent keeps working.
func TestStartWhileRunningIsRejectedAndTheRunningAgentIsUntouched(t *testing.T) {
	running := &scriptedCodec{encodeFn: func(_ models.AgentCommand, requestID string) (string, bool, error) {
		return "still:" + requestID, true, nil
	}}
	runtime := newTestRuntime(t, "cat", testStopTimeout)
	taskID, sink := startAgent(t, runtime, LaunchSpec{}, running)
	leader := registeredProcess(t, runtime, taskID).pgid

	err := runtime.Start(context.Background(), taskID, t.TempDir(), LaunchSpec{}, models.ResourceLimits{}, &scriptedCodec{}, sink)

	require.ErrorIs(t, err, werrors.Runtime("el agente ya está en ejecución"))
	assert.Equal(t, leader, registeredProcess(t, runtime, taskID).pgid, "the running agent must not be replaced")
	assert.True(t, runtime.IsRunning(taskID))
	require.NoError(t, runtime.Send(context.Background(), taskID, models.AbortCommand{}))
	require.Eventually(t, func() bool { return len(sink.notices()) == 1 }, waitTimeout, pollInterval)
	assert.Equal(t, []string{"still:req-1"}, sink.notices(), "the original agent must still receive commands")
}

// Rule from source: when the shell itself cannot be launched, Start fails with the program name and
// nothing is registered or reported. The message never carries the launch environment.
func TestSpawnFailureNamesTheProgramAndRegistersNothing(t *testing.T) {
	runtime := newTestRuntime(t, "cat", testStopTimeout)
	runtime.hostEnv = func() []env.Var {
		return []env.Var{{Key: "HOME", Value: t.TempDir()}, {Key: "SHELL", Value: "/nonexistent/shell"}}
	}
	taskID := newTaskID(t)
	sink := &fakeEventSink{}
	spec := LaunchSpec{ExtraEnv: []env.Var{{Key: "AGENT_TOKEN", Value: "s3cr3t"}}}

	err := runtime.Start(context.Background(), taskID, t.TempDir(), spec, models.ResourceLimits{}, &scriptedCodec{}, sink)

	require.Error(t, err)
	assert.True(t, errors.Is(err, werrors.Runtime("")), "spawn failure must be a runtime error")
	assert.Contains(t, err.Error(), "no se pudo lanzar cat:")
	assert.NotContains(t, err.Error(), "s3cr3t")
	assert.False(t, runtime.IsRunning(taskID))
	assert.Empty(t, sink.snapshot())
}

// Rule from source: the shell launches the agent with exec, so a missing agent binary surfaces as the
// shell's exit status 127 through ProcessExited; Start itself succeeds.
func TestMissingAgentBinaryIsReportedAsExitStatusThroughProcessExited(t *testing.T) {
	runtime := newTestRuntime(t, "definitely-missing-agent-binary", testStopTimeout)
	taskID, sink := startAgent(t, runtime, LaunchSpec{}, &scriptedCodec{})

	require.Eventually(t, func() bool { return len(sink.exitCodes()) == 1 }, waitTimeout, pollInterval)

	assert.False(t, runtime.IsRunning(taskID))
	assert.Equal(t, []*int32{int32Ptr(127)}, sink.exitCodes())
}

// Rule from source: stdin lines are written in the order the codec produces them. The codec's start
// lines go out first, and the lines it queues in reaction to a decoded line go out after it.
// Rust: agent_process.rs::codec_start_lines_and_decode_replies_reach_the_agent_stdin_in_order
func TestCodecStartLinesAndDecodeRepliesReachTheAgentStdinInOrder(t *testing.T) {
	runtime := newTestRuntime(t, "cat", testStopTimeout)
	codec := &scriptedCodec{
		startLines: []string{"first"},
		replies:    map[string][]string{"first": {"second"}},
	}
	_, sink := startAgent(t, runtime, LaunchSpec{}, codec)

	require.Eventually(t, func() bool { return len(sink.notices()) == 2 }, waitTimeout, pollInterval)

	assert.Equal(t, []string{"first", "second"}, sink.notices())
}

// Rule from source: Send encodes the command with a fresh request id and writes the encoded line, then
// the lines the codec queues afterwards. Here the echo agent proves what reached its stdin.
func TestSendWritesTheEncodedCommandWithTheRequestID(t *testing.T) {
	runtime := newTestRuntime(t, "cat", testStopTimeout)
	codec := &scriptedCodec{encodeFn: func(command models.AgentCommand, requestID string) (string, bool, error) {
		require.IsType(t, models.AbortCommand{}, command)
		return "abort:" + requestID, true, nil
	}}
	taskID, sink := startAgent(t, runtime, LaunchSpec{}, codec)

	require.NoError(t, runtime.Send(context.Background(), taskID, models.AbortCommand{}))

	require.Eventually(t, func() bool { return len(sink.notices()) == 1 }, waitTimeout, pollInterval)
	assert.Equal(t, []string{"abort:req-1"}, sink.notices())
}

// Rule from source: an encode error is returned to the caller and nothing is written to stdin.
func TestSendReturnsTheEncodeErrorAndWritesNothing(t *testing.T) {
	runtime := newTestRuntime(t, "cat", testStopTimeout)
	encodeErr := werrors.Unsupported("steer")
	codec := &scriptedCodec{encodeFn: func(models.AgentCommand, string) (string, bool, error) {
		return "", false, encodeErr
	}}
	taskID, sink := startAgent(t, runtime, LaunchSpec{}, codec)

	err := runtime.Send(context.Background(), taskID, models.AbortCommand{})

	require.ErrorIs(t, err, encodeErr)
	require.NoError(t, runtime.Stop(context.Background(), taskID))
	assert.Empty(t, sink.notices(), "a failed encode must not write anything to stdin")
}

// Rule from source: sending to a task without a live agent, or after its agent stopped, is AgentNotRunning.
func TestSendAfterStopOrForUnknownTaskIsAgentNotRunning(t *testing.T) {
	runtime := newTestRuntime(t, "cat", testStopTimeout)
	taskID, _ := startAgent(t, runtime, LaunchSpec{}, &scriptedCodec{})
	require.NoError(t, runtime.Stop(context.Background(), taskID))

	assert.ErrorIs(t, runtime.Send(context.Background(), taskID, models.AbortCommand{}), werrors.AgentNotRunning())
	assert.ErrorIs(t, runtime.Send(context.Background(), newTaskID(t), models.AbortCommand{}), werrors.AgentNotRunning())
}

// Rule from source: stop and a natural exit racing each other deliver one ProcessExited with the real
// exit code (no forced kill, the agent ends on its own) and leave the registry empty.
func TestConcurrentStopAndNaturalExitDeliverOneProcessExited(t *testing.T) {
	runtime := newTestRuntime(t, "sh", 10*time.Second)
	taskID, sink := startAgent(t, runtime, LaunchSpec{AgentArgs: []string{"-c", "sleep 0.05; exit 3"}}, &scriptedCodec{})

	done := make(chan error, 1)
	go func() { done <- runtime.Stop(context.Background(), taskID) }()
	require.NoError(t, <-done)

	assert.False(t, runtime.IsRunning(taskID))
	assert.Equal(t, []*int32{int32Ptr(3)}, sink.exitCodes())
}

// Rule from source: stderr is drained for the whole life of the agent, so its writes keep succeeding. The
// Rust logger returns at the first line that is not valid UTF-8 (see report), which closes the read end:
// every later stderr write of the agent then fails with a broken pipe. The agent here writes an invalid
// line and then more than a pipe buffer of stderr, and reports whether the whole write succeeded.
func TestStderrIsDrainedSoTheAgentsStderrWritesKeepSucceeding(t *testing.T) {
	runtime := newTestRuntime(t, "sh", testStopTimeout)
	codec := &scriptedCodec{}
	script := "printf '\\377\\n' >&2; " +
		"if head -c 262144 /dev/zero >&2; then echo wrote-all; else echo stderr-broken; fi"
	taskID, sink := startAgent(t, runtime, LaunchSpec{AgentArgs: []string{"-c", script}}, codec)

	require.Eventually(t, func() bool { return len(sink.exitCodes()) == 1 }, waitTimeout, pollInterval)

	assert.Equal(t, []string{"wrote-all"}, sink.notices())
	assert.Equal(t, []*int32{int32Ptr(0)}, sink.exitCodes())
	assert.False(t, runtime.IsRunning(taskID))
}

// Rule from source: lines end at LF, a trailing CR/LF run is trimmed, and a last line without LF is
// still delivered.
func TestLineFramingTrimsLineEndingsAndDeliversTheLastLine(t *testing.T) {
	runtime := newTestRuntime(t, "sh", testStopTimeout)
	_, sink := startAgent(t, runtime,
		LaunchSpec{AgentArgs: []string{"-c", "printf 'a\\r\\nb\\r\\r\\nlast'"}}, &scriptedCodec{})

	require.Eventually(t, func() bool { return len(sink.exitCodes()) == 1 }, waitTimeout, pollInterval)

	assert.Equal(t, []string{"a", "b", "last"}, sink.notices())
}

// Rule from source: invalid UTF-8 from the agent is replaced, not dropped, so the rest of the line survives.
func TestInvalidUTF8FromTheAgentIsReplacedNotDropped(t *testing.T) {
	runtime := newTestRuntime(t, "sh", testStopTimeout)
	_, sink := startAgent(t, runtime,
		LaunchSpec{AgentArgs: []string{"-c", "printf '\\377x\\n'"}}, &scriptedCodec{})

	require.Eventually(t, func() bool { return len(sink.exitCodes()) == 1 }, waitTimeout, pollInterval)

	assert.Equal(t, []string{"\uFFFDx"}, sink.notices())
}

// Rule from source: extra variables reach the agent, the parent's variables do not, and the value of a
// secret is never written to the logs at any level.
func TestAgentSeesOnlyItsOwnEnvironmentAndSecretsStayOutOfLogs(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	runtime := newTestRuntime(t, "sh", testStopTimeout)
	spec := LaunchSpec{
		AgentArgs: []string{"-c", `printf '%s|%s\n' "$AGENT_TOKEN" "${PARENT_SECRET-unset}"`},
		ExtraEnv:  []env.Var{{Key: "AGENT_TOKEN", Value: "s3cr3t"}},
	}
	_, sink := startAgent(t, runtime, spec, &scriptedCodec{})

	require.Eventually(t, func() bool { return len(sink.exitCodes()) == 1 }, waitTimeout, pollInterval)

	assert.Equal(t, []string{"s3cr3t|unset"}, sink.notices())
	assert.NotContains(t, logs.String(), "s3cr3t", "a launch secret reached the logs")
}

// Rule from source: killing an agent that already exited and was reaped must not signal anything or
// fail. The Rust live_pid helper guards the same case (only a live, unreaped child is signalled).
// Rust: agent_process.rs::live_pid_is_only_reported_while_the_child_runs
func TestKillAgentAfterItExitedIsANoOp(t *testing.T) {
	runtime := newTestRuntime(t, "cat", testStopTimeout)
	taskID, _ := startAgent(t, runtime, LaunchSpec{}, &scriptedCodec{})
	process := registeredProcess(t, runtime, taskID)
	require.NoError(t, runtime.Stop(context.Background(), taskID))
	<-process.done

	assert.NoError(t, killAgent(context.Background(), process))
}

// Rule from source: a grandchild that leaves the process group (setsid) is outside the kill reach. The
// forced stop must still return: stdout is closed after exitTimeout so a held pipe cannot block the stop.
// The escaped process itself is not reached; the test kills it at cleanup and documents that limit.
func TestStopReturnsEvenWhenAGrandchildEscapesTheGroupAndHoldsStdout(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid is not installed")
	}
	runtime := newTestRuntime(t, "sh", testStopTimeout)
	pidFile := filepath.Join(t.TempDir(), "escaped.pid")
	script := "setsid sleep 300 </dev/null 2>/dev/null & echo $! > " + shellPath(pidFile) + "; wait"
	taskID, sink := startAgent(t, runtime, LaunchSpec{AgentArgs: []string{"-c", script}}, &scriptedCodec{})
	escaped := readPIDFile(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(escaped, syscall.SIGKILL) })

	stopped := make(chan error, 1)
	started := time.Now()
	go func() { stopped <- runtime.Stop(context.Background(), taskID) }()
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "stop hung on a pipe held by a process outside the group")
	}

	assert.Less(t, time.Since(started), 5*time.Second, "stop must not wait for a process outside the group")
	assert.False(t, runtime.IsRunning(taskID))
	assert.Equal(t, []*int32{nil}, sink.exitCodes())
	assert.True(t, processAlive(escaped), "known limit: a process that left the group is not reached by the kill")
}

func int32Ptr(value int32) *int32 {
	return &value
}

func readProcStatOrFail(t *testing.T, pid int) procStat {
	t.Helper()
	stat, ok := readProcStat(pid)
	require.True(t, ok, "pid %d has no /proc entry", pid)
	return stat
}

// snapshot returns a copy of the events recorded so far.
func (s *fakeEventSink) snapshot() []recordedEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recordedEvent(nil), s.events...)
}

// exitCodes returns the code of every ProcessExited event, in order. A nil entry means no code (killed).
func (s *fakeEventSink) exitCodes() []*int32 {
	var codes []*int32
	for _, recorded := range s.snapshot() {
		if exited, ok := recorded.event.(models.ProcessExited); ok {
			codes = append(codes, exited.Code)
		}
	}
	return codes
}

// notices returns the message of every Notice event, in order.
func (s *fakeEventSink) notices() []string {
	var messages []string
	for _, recorded := range s.snapshot() {
		if notice, ok := recorded.event.(models.Notice); ok {
			messages = append(messages, notice.Message)
		}
	}
	return messages
}

// scriptedCodec is a LineCodec whose behavior each test scripts: lines written at start, reply lines
// queued per decoded line, and an optional encoder.
type scriptedCodec struct {
	startLines []string
	replies    map[string][]string
	encodeFn   func(command models.AgentCommand, requestID string) (string, bool, error)

	mu      sync.Mutex
	pending []string
}

func (c *scriptedCodec) Start() []string {
	return c.startLines
}

func (c *scriptedCodec) Decode(_ models.TaskID, line string) []models.AgentEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = append(c.pending, c.replies[line]...)
	return []models.AgentEvent{models.Notice{Level: models.NoticeInfo, Message: line}}
}

func (c *scriptedCodec) TakeOutgoing() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	outgoing := c.pending
	c.pending = nil
	return outgoing
}

func (c *scriptedCodec) Encode(command models.AgentCommand, requestID string) (string, bool, error) {
	if c.encodeFn == nil {
		return "", false, nil
	}
	return c.encodeFn(command, requestID)
}
