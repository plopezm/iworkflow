//go:build unix

package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/env"
	"iworkflow/internal/core/process"
	"iworkflow/internal/modules/tasks/domain/models"
)

// The timings are local to this file, which builds on every unix, not only the Linux tests that define the shared ones.
const (
	pidTestStopTimeout = 200 * time.Millisecond
	pidTestExitTimeout = 300 * time.Millisecond
	pidPollInterval    = 20 * time.Millisecond
)

// idleCodec is a LineCodec that neither expects nor produces protocol lines, so the agent is only its program.
type idleCodec struct {
	NoHandshake
}

func (idleCodec) Decode(models.TaskID, string) []models.AgentEvent { return nil }

func (idleCodec) Encode(models.AgentCommand, string) (string, bool, error) { return "", false, nil }

// newPIDTestRuntime builds a runtime that runs program through /bin/sh, with no systemd scope.
func newPIDTestRuntime(t *testing.T, program string) *ProcessRuntime {
	t.Helper()
	runtime := NewProcessRuntime(program).WithStopTimeout(pidTestStopTimeout)
	runtime.exitTimeout = pidTestExitTimeout
	runtime.hostEnv = func() []env.Var {
		return []env.Var{{Key: "HOME", Value: t.TempDir()}, {Key: "SHELL", Value: "/bin/sh"}}
	}
	runtime.systemdAvailable = func() bool { return false }
	return runtime
}

func TestAgentPIDIsTheLivePidOfTheAgentAndFalseWithoutOne(t *testing.T) {
	runtime := newPIDTestRuntime(t, "cat")
	taskID := newTaskID(t)
	other := newTaskID(t)

	pid, ok := runtime.AgentPID(taskID)
	assert.False(t, ok, "a task that never started has no agent")
	assert.Zero(t, pid)

	sink := &fakeEventSink{}
	require.NoError(t, runtime.Start(context.Background(), taskID, t.TempDir(), LaunchSpec{}, models.ResourceLimits{}, idleCodec{}, sink))
	t.Cleanup(func() { _ = runtime.Stop(context.Background(), taskID) })

	pid, ok = runtime.AgentPID(taskID)
	require.True(t, ok, "a running agent must be reported")
	assert.Positive(t, pid)
	assert.True(t, process.GroupAlive(pid), "the pid must be the live agent's process group leader")

	_, ok = runtime.AgentPID(other)
	assert.False(t, ok, "another task has no agent")

	require.NoError(t, runtime.Stop(context.Background(), taskID))
	pid, ok = runtime.AgentPID(taskID)
	assert.False(t, ok, "a stopped agent is no longer live")
	assert.Zero(t, pid)
}

func TestAgentPIDIsFalseOnceTheAgentExitedOnItsOwn(t *testing.T) {
	runtime := newPIDTestRuntime(t, "true")
	taskID := newTaskID(t)
	sink := &fakeEventSink{}
	require.NoError(t, runtime.Start(context.Background(), taskID, t.TempDir(), LaunchSpec{}, models.ResourceLimits{}, idleCodec{}, sink))

	require.Eventually(t, func() bool { return !runtime.IsRunning(taskID) }, waitTimeout, pidPollInterval)

	_, ok := runtime.AgentPID(taskID)
	assert.False(t, ok, "an agent that exited is not live")
}
