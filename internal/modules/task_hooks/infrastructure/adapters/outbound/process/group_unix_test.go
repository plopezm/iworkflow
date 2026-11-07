//go:build !windows

package process

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// Review advisory (b): a timeout can fire after the script already exited and its group is gone. The kill then
// answers ESRCH, which is not a failure: the process is already gone.
func TestCancelIgnoresAGroupThatAlreadyExited(t *testing.T) {
	t.Parallel()
	cmd := exec.CommandContext(context.Background(), "true")
	ownProcessGroup(cmd)
	require.NoError(t, cmd.Run())

	require.NoError(t, cmd.Cancel())
}
