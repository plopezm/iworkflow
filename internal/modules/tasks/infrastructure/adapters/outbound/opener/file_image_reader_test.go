package opener

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// pngBytes is the signature of a PNG file followed by a few bytes.
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00")

func TestFileImageReaderRejectsAFileThatIsNotAnImage(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("not an image"), 0o600))
	link := filepath.Join(dir, "x.png")
	require.NoError(t, os.Symlink(secret, link))
	_, err := NewFileImageReader().ReadImage(context.Background(), link, 1<<20)
	assert.ErrorIs(t, err, werrors.ErrOpenPathBlocked)
}

func TestFileImageReader(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "a.png")
	require.NoError(t, os.WriteFile(small, pngBytes, 0o600))
	reader := NewFileImageReader()
	ctx := context.Background()

	data, err := reader.ReadImage(ctx, small, 10)
	require.NoError(t, err)
	assert.Equal(t, pngBytes, data)

	_, err = reader.ReadImage(ctx, small, 4)
	assert.ErrorIs(t, err, werrors.ErrOpenPathTooLarge)

	_, err = reader.ReadImage(ctx, filepath.Join(dir, "none.png"), 5)
	assert.ErrorIs(t, err, werrors.ErrOpenPathMissing)

	_, err = reader.ReadImage(ctx, dir, 5)
	assert.ErrorIs(t, err, werrors.ErrOpenPathMissing)
}
