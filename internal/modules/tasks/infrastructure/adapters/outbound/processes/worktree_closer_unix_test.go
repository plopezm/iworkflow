//go:build unix

package processes_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/processes"
)

var _ outbound.WorktreeProcessCloser = processes.WorktreeCloser{}

func TestCloseUsersKillsAProcessWorkingInTheWorktree(t *testing.T) {
	worktree := t.TempDir()
	cmd := exec.Command("sleep", "60")
	cmd.Dir = worktree
	require.NoError(t, cmd.Start())
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-done })

	killed, err := processes.NewWorktreeCloser().CloseUsers(context.Background(), worktree)

	require.NoError(t, err)
	assert.Equal(t, 1, killed)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the process is still alive")
	}
}
