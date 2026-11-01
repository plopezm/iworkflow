package process_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/outbound/process"
)

func TestStartRunsTheExecutableInTheDirectoryWithTheExtraEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	dir := t.TempDir()
	starter := process.NewStarter()

	err := starter.Start(context.Background(), models.LaunchSpec{
		Path: "/bin/sh",
		Args: []string{"-c", `printf '%s %s' "$PWD" "$EXTRA" > out.txt`},
		Env:  []models.EnvVar{{Name: "EXTRA", Value: "yes"}},
		Dir:  dir,
	})
	starter.Wait()

	require.NoError(t, err)
	written, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	require.Equal(t, resolved+" yes", string(written))
}

func TestStartReportsAMissingExecutableOrDirectory(t *testing.T) {
	tests := map[string]models.LaunchSpec{
		"missing executable": {Path: "/no/such/app", Dir: t.TempDir()},
		"missing directory":  {Path: "/bin/true", Dir: "/no/such/dir"},
	}
	for name, spec := range tests {
		t.Run(name, func(t *testing.T) {
			err := process.NewStarter().Start(context.Background(), spec)

			require.ErrorIs(t, err, openerrors.ErrLaunchFailed)
		})
	}
}
