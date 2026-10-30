package tasks_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// optionLikeRef would write a file if git read it as an option instead of a ref.
func optionLikeRef(t *testing.T) (ref string, marker string) {
	t.Helper()
	marker = filepath.Join(t.TempDir(), "owned.txt")
	return "--output=" + marker, marker
}

func TestDiffRefusesRefsThatLookLikeOptions(t *testing.T) {
	f := newWorktreeFixture(t)
	ref, marker := optionLikeRef(t)

	_, errBase := f.adapter.Diff(bg(), f.repo, ref, taskBranch)
	_, errBranch := f.adapter.Diff(bg(), f.repo, "main", ref)
	_, errLive := f.adapter.DiffLive(bg(), f.worktree, ref)

	assert.Error(t, errBase)
	assert.Error(t, errBranch)
	assert.Error(t, errLive)
	assert.NoFileExists(t, marker)
}

func TestFingerprintAndPinRefuseRefsThatLookLikeOptions(t *testing.T) {
	f := newWorktreeFixture(t)
	ref, marker := optionLikeRef(t)

	_, errFingerprint := f.adapter.DiffFingerprint(bg(), f.repo, ref, taskBranch)
	_, errPin := f.adapter.PinRange(bg(), f.repo, "main", ref, pinRef)

	assert.Error(t, errFingerprint)
	assert.Error(t, errPin)
	assert.NoFileExists(t, marker)
}

func TestMergeRefusesRefsThatLookLikeOptions(t *testing.T) {
	f := newWorktreeFixture(t)
	f.commitFile(t, "a.txt", "a\n", "pi: a")
	ref, marker := optionLikeRef(t)

	_, errBase := f.adapter.Merge(bg(), f.repo, ref, taskBranch, "merge")
	_, errBranch := f.adapter.Merge(bg(), f.repo, "main", ref, "merge")

	assert.Error(t, errBase)
	assert.Error(t, errBranch)
	assert.NoFileExists(t, marker)
}

// Containment: git itself would remove any registered worktree, so the root check has to run first.
func TestRemoveRefusesARegisteredWorktreeOutsideTheManagedRoot(t *testing.T) {
	f := newWorktreeFixture(t)
	outside := filepath.Join(realDir(t, t.TempDir()), "elsewhere")
	gitCmd(t, f.repo, "worktree", "add", "-q", "-b", "pi/outside-9", outside, "main")

	err := f.adapter.Remove(bg(), f.repo, outside, "pi/outside-9")

	assert.ErrorIs(t, err, werrors.UnsafeWorktreePath(""))
	assert.DirExists(t, outside)
}
