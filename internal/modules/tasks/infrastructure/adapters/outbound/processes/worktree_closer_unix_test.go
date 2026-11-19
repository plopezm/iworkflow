//go:build unix

package processes_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/processes"
)

var _ outbound.WorktreeProcessCloser = (*processes.WorktreeCloser)(nil)

// agentLocator answers a fixed agent pid.
type agentLocator struct{ pid int }

func (a agentLocator) AgentPID(models.TaskID) (int, bool) { return a.pid, a.pid != 0 }

type child struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func start(t *testing.T, dir string, name string, args ...string) *child {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	require.NoError(t, cmd.Start())
	c := &child{cmd: cmd, done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(c.done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-c.done })
	return c
}

func (c *child) exited(within time.Duration) bool {
	select {
	case <-c.done:
		return true
	case <-time.After(within):
		return false
	}
}

func TestListUsersMapsTheProcessesAndLeavesTheAgentOut(t *testing.T) {
	worktree := t.TempDir()
	agent := start(t, worktree, "sh", "-c", "sleep 60 & wait")
	devServer := start(t, worktree, "sleep", "61")
	closer := processes.NewWorktreeCloser(agentLocator{pid: agent.cmd.Process.Pid})

	users, err := closer.ListUsers(context.Background(), newTaskID(t), worktree)

	require.NoError(t, err)
	require.Len(t, users, 1, "the agent and its children are left out: %+v", users)
	assert.Equal(t, devServer.cmd.Process.Pid, users[0].PID)
	assert.Equal(t, "sleep", users[0].Name)
	assert.Equal(t, models.ProcessReasonWorkingDirectory, users[0].Reason)
	assert.NotEmpty(t, users[0].Started)
}

func TestListUsersWithoutALiveAgentListsEveryProcess(t *testing.T) {
	worktree := t.TempDir()
	first := start(t, worktree, "sleep", "60")
	closer := processes.NewWorktreeCloser(agentLocator{})

	users, err := closer.ListUsers(context.Background(), newTaskID(t), worktree)

	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, first.cmd.Process.Pid, users[0].PID)
}

func TestCloseUsersClosesOnlyTheSelectedProcess(t *testing.T) {
	worktree := t.TempDir()
	chosen := start(t, worktree, "sleep", "60")
	kept := start(t, worktree, "sleep", "61")
	closer := processes.NewWorktreeCloser(agentLocator{})
	users, err := closer.ListUsers(context.Background(), newTaskID(t), worktree)
	require.NoError(t, err)
	var selected []models.WorktreeProcess
	for _, user := range users {
		if user.PID == chosen.cmd.Process.Pid {
			selected = append(selected, user)
		}
	}
	require.Len(t, selected, 1)

	closed, err := closer.CloseUsers(context.Background(), worktree, selected)

	require.NoError(t, err)
	assert.Equal(t, 1, closed)
	assert.True(t, chosen.exited(5*time.Second))
	assert.False(t, kept.exited(200*time.Millisecond), "a process the user did not select stays alive")
}

func newTaskID(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return id
}
