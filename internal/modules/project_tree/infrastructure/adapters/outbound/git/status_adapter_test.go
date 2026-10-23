package git

import (
	"testing"

	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestParseStatusClassifiesColumns(t *testing.T) {
	output := " M src/a.rs\x00A  src/b.rs\x00?? src/c.rs\x00 D src/d.rs\x00UU src/e.rs\x00AM src/f.rs"

	changes := parseStatus(output, "")

	require.Equal(t, []models.GitChange{
		{Path: "src/a.rs", Kind: models.GitChangeModified, Unstaged: true},
		{Path: "src/b.rs", Kind: models.GitChangeAdded, Staged: true},
		{Path: "src/c.rs", Kind: models.GitChangeUntracked, Unstaged: true},
		{Path: "src/d.rs", Kind: models.GitChangeDeleted, Unstaged: true},
		{Path: "src/e.rs", Kind: models.GitChangeConflicted, Staged: true, Unstaged: true},
		{Path: "src/f.rs", Kind: models.GitChangeAdded, Staged: true, Unstaged: true},
	}, changes)
}

func TestParseStatusSetsStagedAndUnstagedFlags(t *testing.T) {
	output := "M  a\x00 M b\x00MM c\x00?? d\x00A  e\x00 D f"

	changes := parseStatus(output, "")

	flags := make([][3]any, 0, len(changes))
	for _, change := range changes {
		flags = append(flags, [3]any{change.Path, change.Staged, change.Unstaged})
	}
	require.Equal(t, [][3]any{
		{"a", true, false},
		{"b", false, true},
		{"c", true, true},
		{"d", false, true},
		{"e", true, false},
		{"f", false, true},
	}, flags)
}

func TestCommitFailureReasonPrefersStderrThenStdout(t *testing.T) {
	require.Equal(t, "boom", commitFailureReason(" boom \n", "out"))
	require.Equal(t, "nothing to commit", commitFailureReason("", "nothing to commit\n"))
	require.Equal(t, "git commit failed without output", commitFailureReason("", " "))
}

func TestPathspecCommandDefaultsToTheWholeProject(t *testing.T) {
	args, err := pathspecCommand([]string{"add", "-A"}, nil)

	require.NoError(t, err)
	require.Equal(t, []string{"--literal-pathspecs", "add", "-A", "--", "."}, args)
}

func TestPathspecCommandRejectsEscapingAndEmptyPaths(t *testing.T) {
	_, err := pathspecCommand([]string{"add", "-A"}, []string{"../x"})
	require.Equal(t, treeerrors.InvalidPath("../x"), err)

	_, err = pathspecCommand([]string{"add", "-A"}, []string{""})
	require.Equal(t, treeerrors.InvalidPath(""), err)
}

func TestRenameConsumesTheOriginalPathField(t *testing.T) {
	changes := parseStatus("R  new.rs\x00old.rs\x00 M other.rs", "")

	require.Len(t, changes, 2)
	require.Equal(t, models.GitChangeRenamed, changes[0].Kind)
	require.Equal(t, "new.rs", changes[0].Path)
	require.Equal(t, "other.rs", changes[1].Path)
}

func TestPrefixIsStrippedAndOutsidePathsAreDropped(t *testing.T) {
	changes := parseStatus(" M app/src/a.rs\x00 M other/b.rs", "app/")

	require.Equal(t, []models.GitChange{
		{Path: "src/a.rs", Kind: models.GitChangeModified, Unstaged: true},
	}, changes)
}

func TestIgnoredAndEmptyOutputYieldNothing(t *testing.T) {
	require.Empty(t, parseStatus("", ""))
	require.Empty(t, parseStatus("!! target/", ""))
}

func TestParseStatusToleratesShortAndMultibyteFields(t *testing.T) {
	// A field shorter than the two columns and a field whose cut would split a character are both
	// skipped or give an empty path, never a panic.
	require.Empty(t, parseStatus("M", ""))
	changes := parseStatus(" M ñ.rs", "")
	require.Len(t, changes, 1)
	require.Equal(t, "ñ.rs", changes[0].Path)
	// Byte 3 is a continuation byte, so cutting there would split the character.
	require.Empty(t, pathOf("AB \xB1x"))
}
