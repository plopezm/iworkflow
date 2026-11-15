package application_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

type fakeImageReader struct {
	paths []string
	data  []byte
	err   error
}

func (r *fakeImageReader) ReadImage(_ context.Context, path string, _ int64) ([]byte, error) {
	r.paths = append(r.paths, path)
	return r.data, r.err
}

func TestPreviewTaskImage(t *testing.T) {
	running := newTask(models.StatusRunning)
	cases := []struct {
		name    string
		raw     string
		readErr error
		want    string
		wantErr error
	}{
		{"absolute image", "/tmp/shots/1.png", nil, filepath.FromSlash("/tmp/shots/1.png"), nil},
		{"relative image resolves in the worktree", "docs/a.jpg:2", nil, filepath.Join(running.WorktreePath, "docs", "a.jpg"), nil},
		{"not an image is never read", "/etc/hosts", nil, "", werrors.ErrOpenPathBlocked},
		{"missing file", "/tmp/none.png", werrors.ErrOpenPathMissing, filepath.FromSlash("/tmp/none.png"), werrors.ErrOpenPathMissing},
		{"too large", "/tmp/big.png", werrors.ErrOpenPathTooLarge, filepath.FromSlash("/tmp/big.png"), werrors.ErrOpenPathTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lifecycle, _ := newLifecycle(newFakeRepository(running))
			reader := &fakeImageReader{data: []byte("bytes"), err: tc.readErr}
			image, err := application.NewPreviewTaskImage(lifecycle, reader, "/home/me").Execute(context.Background(), running.ID, tc.raw)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, []byte("bytes"), image.Data)
				assert.NotEmpty(t, image.ContentType)
			}
			if tc.want == "" {
				assert.Empty(t, reader.paths)
			} else {
				assert.Equal(t, []string{tc.want}, reader.paths)
			}
		})
	}
}
