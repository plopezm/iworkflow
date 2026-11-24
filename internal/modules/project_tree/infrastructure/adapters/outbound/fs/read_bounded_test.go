package fs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
)

func TestReadBoundedReturnsAFileWithinTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.png")
	require.NoError(t, os.WriteFile(path, []byte("12345"), 0o600))
	content, err := readBounded(path, "a.png", 5)
	require.NoError(t, err)
	assert.Equal(t, []byte("12345"), content)
}

func TestReadBoundedRejectsAFileThatGrewPastTheLimitAfterTheStatCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.png")
	require.NoError(t, os.WriteFile(path, []byte("123456"), 0o600))
	_, err := readBounded(path, "a.png", 5)
	assert.ErrorIs(t, err, treeerrors.FileTooLarge("a.png"))
}
