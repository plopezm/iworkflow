package application_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

type fakeOpener struct {
	opened []string
	err    error
}

func (o *fakeOpener) Open(_ context.Context, path string) error {
	o.opened = append(o.opened, path)
	return o.err
}

func TestOpenTaskPath(t *testing.T) {
	running := newTask(models.StatusRunning)
	backlog := newTask(models.StatusBacklog)
	cases := []struct {
		name      string
		task      models.Task
		raw       string
		openerErr error
		want      string
		wantErr   error
	}{
		{"relative resolves in the worktree", running, "src/a.go:4", nil, filepath.Join(running.WorktreePath, "src", "a.go"), nil},
		{"absolute opens as is", running, "/etc/hosts", nil, filepath.FromSlash("/etc/hosts"), nil},
		{"backlog task has no worktree", backlog, "src/a.go", nil, "", werrors.ErrOpenPathInvalid},
		{"program is blocked", running, "/tmp/run.sh", nil, "", werrors.ErrOpenPathBlocked},
		{"missing path", running, "/nope/a.txt", werrors.ErrOpenPathMissing, filepath.FromSlash("/nope/a.txt"), werrors.ErrOpenPathMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lifecycle, _ := newLifecycle(newFakeRepository(tc.task))
			opener := &fakeOpener{err: tc.openerErr}
			err := application.NewOpenTaskPath(lifecycle, opener, "/home/me").Execute(context.Background(), tc.task.ID, tc.raw)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
			} else {
				require.NoError(t, err)
			}
			if tc.want == "" {
				assert.Empty(t, opener.opened)
			} else {
				assert.Equal(t, []string{tc.want}, opener.opened)
			}
		})
	}
}

func TestOpenTaskPathUnknownTask(t *testing.T) {
	lifecycle, _ := newLifecycle(newFakeRepository())
	id, err := models.NewTaskID()
	require.NoError(t, err)
	err = application.NewOpenTaskPath(lifecycle, &fakeOpener{}, "").Execute(context.Background(), id, "/a/b")
	assert.ErrorIs(t, err, werrors.NotFound(""))
}
