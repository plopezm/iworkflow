package agent_settings_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/git"
)

func pinPath(commonDir string) string {
	return filepath.Join(commonDir, "gentle-ai", "profile-pin.json")
}

func TestWritesPinFileWithExpectedContent(t *testing.T) {
	root := gitRepo(t)
	profile := profileOf(t, "Claude")
	adapter := git.NewProfilePinAdapter()

	require.NoError(t, adapter.Apply(context.Background(), root, profile))

	content, err := os.ReadFile(pinPath(filepath.Join(root, ".git")))
	require.NoError(t, err)
	require.Equal(t,
		"{\n  \"kind\": \"gentle-pi.agent_model_profile_pin\",\n  \"version\": 1,\n  \"profile\": \"Claude\"\n}\n",
		string(content))
}

func TestNilProfileRemovesExistingPinFile(t *testing.T) {
	root := gitRepo(t)
	adapter := git.NewProfilePinAdapter()
	require.NoError(t, adapter.Apply(context.Background(), root, profileOf(t, "Claude")))

	require.NoError(t, adapter.Apply(context.Background(), root, nil))

	require.NoFileExists(t, pinPath(filepath.Join(root, ".git")))
}

func TestNilProfileOnMissingPinFileIsNotAnError(t *testing.T) {
	root := gitRepo(t)

	require.NoError(t, git.NewProfilePinAdapter().Apply(context.Background(), root, nil))
}

func TestResolvesCommonDirFromASubfolder(t *testing.T) {
	root := gitRepo(t)
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))

	require.NoError(t, git.NewProfilePinAdapter().Apply(context.Background(), sub, profileOf(t, "Claude")))

	require.FileExists(t, pinPath(filepath.Join(root, ".git")))
}

func TestPinIsSharedByLinkedWorktrees(t *testing.T) {
	root := gitRepo(t)
	worktree := filepath.Join(t.TempDir(), "task-worktree")
	runGit(t, root, "worktree", "add", "-q", "--detach", worktree)
	adapter := git.NewProfilePinAdapter()

	require.NoError(t, adapter.Apply(context.Background(), worktree, profileOf(t, "Gentleman")))

	require.FileExists(t, pinPath(filepath.Join(root, ".git")))
}

func TestNotARepositoryIsAnIOError(t *testing.T) {
	err := git.NewProfilePinAdapter().Apply(context.Background(), t.TempDir(), (*models.ProfileName)(nil))

	require.Error(t, err)
}
