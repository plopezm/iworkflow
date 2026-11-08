//go:build !windows

package process_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/process"
)

func TestTimeoutKillsGrandchildren(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	hook := newHook(t, models.ScriptSpec{Command: "sh", Args: []string{"-c", "sleep 60 & echo $! > pid; wait"}}, 300*time.Millisecond)
	out, err := process.NewScriptRunner().Run(context.Background(), hook, runCtx(dir))
	require.NoError(t, err)
	assert.Equal(t, models.RunTimedOut, out.Status)

	raw, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return syscall.Kill(pid, 0) != nil }, 3*time.Second, 50*time.Millisecond,
		"the grandchild must not survive the timeout")
}
