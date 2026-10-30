package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteSkipsTemporaryNamesAlreadyTaken(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "profiles.json")
	taken := filepath.Join(dir, fmt.Sprintf(".profiles.json.%d.0.tmp", os.Getpid()))
	require.NoError(t, os.WriteFile(taken, []byte("someone else"), 0o600))

	require.NoError(t, AtomicWrite(target, []byte("{}\n")))

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "{}\n", string(content))
	other, err := os.ReadFile(taken)
	require.NoError(t, err)
	require.Equal(t, "someone else", string(other))
}

func TestReadOptionalReturnsNotFoundForMissingFile(t *testing.T) {
	content, found, err := ReadOptional(filepath.Join(t.TempDir(), "missing.json"))

	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, content)
}
