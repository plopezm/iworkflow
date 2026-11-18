//go:build unix

package process

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandOf returns the command name ps reports for pid. An error means ps could not see the process, which
// is the expected outcome for a pid that exited meanwhile, so the caller decides what it means.
func commandOf(ctx context.Context, pid int) (string, error) {
	out, err := exec.CommandContext(ctx, "ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", fmt.Errorf("ps for process %d: %w", pid, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func TestDescendantsFindsTheSleepStartedByTheShell(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 60 & wait")
	SetProcessGroup(cmd)
	require.NoError(t, cmd.Start())
	shellPID := cmd.Process.Pid
	waited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(waited) }() // Wait's error is the shell being killed by the cleanup.
	t.Cleanup(func() {
		require.NoError(t, KillGroup(shellPID))
		<-waited
	})

	var found []int
	require.Eventually(t, func() bool {
		got, err := Descendants(context.Background(), shellPID)
		if err != nil {
			return false
		}
		for _, pid := range got {
			if name, err := commandOf(context.Background(), pid); err == nil && name == "sleep" {
				found = got
				return true
			}
		}
		return false
	}, 10*time.Second, 20*time.Millisecond, "the sleep started by the shell must show up as its descendant")

	assert.NotContains(t, found, shellPID, "the root is not its own descendant")
	assert.IsIncreasing(t, found, "descendants are sorted")
}
