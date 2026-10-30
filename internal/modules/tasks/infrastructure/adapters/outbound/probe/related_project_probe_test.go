package probe_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/probe"
)

// writeFile creates a regular file, with its parent folders, so that it is not a directory.
func writeFile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
}

// isolateEngram points PATH at an empty folder and HOME at a folder without ~/.engram, so that the host
// installation does not leak into the test. It returns the home folder.
func isolateEngram(t *testing.T) string {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func TestHasCodegraphReadsTheCodegraphFolder(t *testing.T) {
	t.Parallel()
	withIndex := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(withIndex, ".codegraph"), 0o755))
	withFile := t.TempDir()
	writeFile(t, filepath.Join(withFile, ".codegraph"))

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "codegraph folder present", path: withIndex, want: true},
		{name: "no codegraph folder", path: t.TempDir(), want: false},
		{name: ".codegraph is a file, not a folder", path: withFile, want: false},
		{name: "empty path never has an index", path: "", want: false},
		{name: "missing project", path: filepath.Join(t.TempDir(), "gone"), want: false},
	}
	adapter := probe.NewRelatedProjectProbe()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, adapter.HasCodegraph(context.Background(), tt.path))
		})
	}
}

func TestEngramAvailableWhenTheExecutableIsOnPath(t *testing.T) {
	isolateEngram(t)
	binDir := t.TempDir()
	writeFile(t, filepath.Join(binDir, "engram"))
	require.NoError(t, os.Chmod(filepath.Join(binDir, "engram"), 0o755))
	t.Setenv("PATH", binDir)

	assert.True(t, probe.NewRelatedProjectProbe().EngramAvailable(context.Background()))
}

func TestEngramAvailableWhenTheHomeFolderExists(t *testing.T) {
	home := isolateEngram(t)
	require.NoError(t, os.Mkdir(filepath.Join(home, ".engram"), 0o755))

	assert.True(t, probe.NewRelatedProjectProbe().EngramAvailable(context.Background()))
}

func TestEngramNotAvailableWithoutExecutableNorHomeFolder(t *testing.T) {
	isolateEngram(t)

	assert.False(t, probe.NewRelatedProjectProbe().EngramAvailable(context.Background()))
}

func TestEngramNotAvailableWhenDotEngramIsAFile(t *testing.T) {
	home := isolateEngram(t)
	writeFile(t, filepath.Join(home, ".engram"))

	assert.False(t, probe.NewRelatedProjectProbe().EngramAvailable(context.Background()))
}
