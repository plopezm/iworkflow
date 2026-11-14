package models_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

func TestResolveOpenPath(t *testing.T) {
	root := filepath.FromSlash("/work/wt")
	home := filepath.FromSlash("/home/me")
	cases := []struct {
		name, raw, want string
	}{
		{"relative to the worktree", "src/main.go", filepath.Join(root, "src", "main.go")},
		{"position is dropped", "src/main.go:12:3", filepath.Join(root, "src", "main.go")},
		{"dot relative", "./a/../b.md", filepath.Join(root, "b.md")},
		{"home", "~/notes/a.md", filepath.Join(home, "notes", "a.md")},
		{"home with backslash", `~\notes\a.md`, filepath.Join(home, "notes", "a.md")},
		{"absolute", filepath.FromSlash("/var/log/syslog"), filepath.FromSlash("/var/log/syslog")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := models.ResolveOpenPath(tc.raw, root, home)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveOpenPathRefusals(t *testing.T) {
	for _, raw := range []string{"", "  ", "a\x00b"} {
		_, err := models.ResolveOpenPath(raw, "/w", "/h")
		assert.ErrorIs(t, err, werrors.ErrOpenPathInvalid, raw)
	}
	_, err := models.ResolveOpenPath("src/a.go", "", "/h")
	assert.ErrorIs(t, err, werrors.ErrOpenPathInvalid, "relative without worktree")
	for _, raw := range []string{"/tmp/run.sh", "/tmp/Setup.EXE:3", "/tmp/a.bat", "/tmp/x.app", "~/x.desktop"} {
		_, err := models.ResolveOpenPath(raw, "/w", "/h")
		assert.ErrorIs(t, err, werrors.ErrOpenPathBlocked, raw)
	}
}
