package opener

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

func TestCommandPerOS(t *testing.T) {
	for goos, want := range map[string]string{"windows": "explorer", "darwin": "open", "linux": "xdg-open", "freebsd": "xdg-open"} {
		name, args := command(goos, "/x/y")
		assert.Equal(t, want, name, goos)
		assert.Equal(t, []string{"/x/y"}, args, goos)
	}
}

func TestOpenReportsAMissingPath(t *testing.T) {
	err := NewOSOpener().Open(context.Background(), filepath.Join(t.TempDir(), "missing.txt"))
	assert.ErrorIs(t, err, werrors.ErrOpenPathMissing)
}
