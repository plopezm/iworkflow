package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLongPathsConfigSetsCoreLongpathsInTheGlobalFile(t *testing.T) {
	global := filepath.Join(t.TempDir(), "gitconfig")

	require.NoError(t, runLongPathsConfig(context.Background(), global))

	content, err := os.ReadFile(global)
	require.NoError(t, err)
	assert.Contains(t, string(content), "longpaths = true")
}

func TestRunGitReturnsFailedErrorOutsideRepository(t *testing.T) {
	_, err := Run(context.Background(), t.TempDir(), "rev-parse", "--show-toplevel")

	var failed *FailedError
	require.ErrorAs(t, err, &failed)
	assert.Nil(t, failed.Code)
	assert.Equal(t, "rev-parse --show-toplevel", failed.Args)
	assert.True(t, strings.HasPrefix(err.Error(), "git rev-parse --show-toplevel falló (None): "), err.Error())
}

func TestRunReturnsTrimmedStdout(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Run(ctx, dir, "init", "-q")
	require.NoError(t, err)

	top, err := Run(ctx, dir, "rev-parse", "--show-toplevel")
	require.NoError(t, err)

	want, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	assert.Equal(t, want, top)
}

func TestRunRawReportsNonZeroExitWithoutError(t *testing.T) {
	out, err := RunRaw(context.Background(), t.TempDir(), "rev-parse", "--show-toplevel")

	require.NoError(t, err)
	assert.False(t, out.Success)
	assert.NotEmpty(t, out.Stderr)
	assert.Empty(t, out.Stdout)
}

func TestRunRawEnvAppliesExtraVariables(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Run(ctx, dir, "init", "-q")
	require.NoError(t, err)

	out, err := RunRawEnv(ctx, dir, []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=iwf.probe", "GIT_CONFIG_VALUE_0=yes"}, "config", "--get", "iwf.probe")

	require.NoError(t, err)
	assert.True(t, out.Success)
	assert.Equal(t, "yes\n", out.Stdout)
}

func TestRunWithStdinWritesToTheProcessInput(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	_, err := Run(ctx, dir, "init", "-q")
	require.NoError(t, err)

	hash, err := RunWithStdin(ctx, dir, []byte("hello"), "hash-object", "-w", "--stdin")
	require.NoError(t, err)
	require.Len(t, hash, 40)

	content, err := Run(ctx, dir, "cat-file", "-p", hash)
	require.NoError(t, err)
	assert.Equal(t, "hello", content)
}

func TestRunWithStdinReturnsFailedErrorWithExitCode(t *testing.T) {
	_, err := RunWithStdin(context.Background(), t.TempDir(), []byte("x"), "rev-parse", "--show-toplevel")

	var failed *FailedError
	require.ErrorAs(t, err, &failed)
	require.NotNil(t, failed.Code)
	assert.NotZero(t, *failed.Code)
}
