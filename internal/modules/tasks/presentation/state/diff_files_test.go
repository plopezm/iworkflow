package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// Port of the Rust tests of src/modules/tasks/presentation/state/diff_files.rs (same names, same fixtures).

const diffModifyPatch = "diff --git a/src/a.rs b/src/a.rs\nindex 1..2 100644\n--- a/src/a.rs\n+++ b/src/a.rs\n@@ -1,4 +1,5 @@ fn x\n one\n-two\n-three\n+TWO\n+THREE\n+extra\n four\n"

func onlyFile(t *testing.T, patch string) state.FileDiff {
	t.Helper()
	files := state.ParsePatch(patch)
	require.Len(t, files, 1)
	return files[0]
}

// cellOf dereferences a split cell that the test expects to be present.
func cellOf(t *testing.T, cell *state.Cell) state.Cell {
	t.Helper()
	require.NotNil(t, cell)
	return *cell
}

func TestModifyCountsAndPairsBlocks(t *testing.T) {
	t.Parallel()
	file := onlyFile(t, diffModifyPatch)
	assert.Equal(t, "src/a.rs", file.Path)
	assert.Equal(t, state.FileStatusModified, file.Status)
	assert.Equal(t, 3, file.Additions)
	assert.Equal(t, 2, file.Deletions)
	rows := file.Hunks[0].Rows
	require.Len(t, rows, 5)
	left := cellOf(t, rows[1].Left)
	right := cellOf(t, rows[1].Right)
	assert.Equal(t, "two", left.Text)
	assert.Equal(t, state.CellRemoved, left.Kind)
	assert.Equal(t, "TWO", right.Text)
	assert.Equal(t, state.CellAdded, right.Kind)
	assert.Nil(t, rows[3].Left)
	assert.Equal(t, "extra", cellOf(t, rows[3].Right).Text)
}

func TestLineNumbersFollowHunkHeader(t *testing.T) {
	t.Parallel()
	file := onlyFile(t, diffModifyPatch)
	rows := file.Hunks[0].Rows
	assert.Equal(t, 1, cellOf(t, rows[0].Left).Number)
	assert.Equal(t, 2, cellOf(t, rows[1].Left).Number)
	assert.Equal(t, 3, cellOf(t, rows[2].Left).Number)
	assert.Equal(t, 2, cellOf(t, rows[1].Right).Number)
	assert.Equal(t, 4, cellOf(t, rows[3].Right).Number)
	last := rows[4]
	assert.Equal(t, 4, cellOf(t, last.Left).Number)
	assert.Equal(t, 5, cellOf(t, last.Right).Number)
	unified := file.Hunks[0].Lines
	assert.Equal(t, 2, unified[1].OldNumber)
	assert.Zero(t, unified[1].NewNumber)
	assert.Equal(t, "two", unified[1].Text)
	assert.Equal(t, 2, unified[3].NewNumber)
}

func TestSecondHunkUsesItsOwnStart(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,1 +1,1 @@\n-a\n+b\n@@ -20,1 +30,1 @@\n-c\n+d\n"
	file := onlyFile(t, patch)
	require.Len(t, file.Hunks, 2)
	row := file.Hunks[1].Rows[0]
	assert.Equal(t, 20, cellOf(t, row.Left).Number)
	assert.Equal(t, 30, cellOf(t, row.Right).Number)
}

func TestAddedFile(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/n.txt b/n.txt\nnew file mode 100644\nindex 0..1\n--- /dev/null\n+++ b/n.txt\n@@ -0,0 +1,2 @@\n+a\n+b\n"
	file := onlyFile(t, patch)
	assert.Equal(t, state.FileStatusAdded, file.Status)
	assert.Equal(t, "n.txt", file.Path)
	assert.Equal(t, 2, file.Additions)
	assert.Equal(t, 0, file.Deletions)
	for _, row := range file.Hunks[0].Rows {
		assert.Nil(t, row.Left)
	}
}

func TestDeletedFileUsesOldPath(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/d.txt b/d.txt\ndeleted file mode 100644\nindex 1..0\n--- a/d.txt\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n"
	file := onlyFile(t, patch)
	assert.Equal(t, state.FileStatusDeleted, file.Status)
	assert.Equal(t, "d.txt", file.Path)
	assert.Equal(t, 0, file.Additions)
	assert.Equal(t, 2, file.Deletions)
}

func TestRenameWithAndWithoutChanges(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/old.rs b/new.rs\nsimilarity index 90%\nrename from old.rs\nrename to new.rs\nindex 1..2\n--- a/old.rs\n+++ b/new.rs\n@@ -1 +1 @@\n-x\n+y\ndiff --git a/p b/q\nsimilarity index 100%\nrename from p\nrename to q\n"
	files := state.ParsePatch(patch)
	require.Len(t, files, 2)
	assert.Equal(t, state.FileStatusRenamed, files[0].Status)
	assert.Equal(t, "new.rs", files[0].Path)
	assert.Equal(t, "old.rs", files[0].OldPath)
	assert.Equal(t, 1, files[0].Additions)
	assert.Equal(t, "q", files[1].Path)
	assert.Equal(t, "p", files[1].OldPath)
	assert.Empty(t, files[1].Hunks)
}

func TestBinaryFile(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/i.png b/i.png\nindex 1..2 100644\nBinary files a/i.png and b/i.png differ\n"
	file := onlyFile(t, patch)
	assert.True(t, file.Binary)
	assert.Empty(t, file.Hunks)
	assert.Equal(t, state.FileStatusModified, file.Status)
}

func TestPureModeChange(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/s.sh b/s.sh\nold mode 100644\nnew mode 100755\n"
	file := onlyFile(t, patch)
	assert.Equal(t, state.FileStatusModified, file.Status)
	assert.Equal(t, "s.sh", file.Path)
	assert.Empty(t, file.Hunks)
}

func TestNoNewlineMarkerIsIgnored(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b\n\\ No newline at end of file\n"
	file := onlyFile(t, patch)
	assert.Equal(t, 1, file.Additions)
	assert.Equal(t, 1, file.Deletions)
	assert.Len(t, file.Hunks[0].Lines, 2)
	assert.Len(t, file.Hunks[0].Rows, 1)
}

func TestTruncatedPatchKeepsPartialHunk(t *testing.T) {
	t.Parallel()
	patch := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,3 +1,3 @@\n a\n-b\n+par"
	file := onlyFile(t, patch)
	assert.Equal(t, 1, file.Additions)
	assert.Equal(t, 1, file.Deletions)
	assert.Equal(t, "par", cellOf(t, file.Hunks[0].Rows[1].Right).Text)
	assert.Len(t, state.ParsePatch("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,3"), 1)
	assert.Empty(t, state.ParsePatch(""))
}

func TestStatusLetters(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "A", state.StatusLetter(state.FileStatusAdded))
	assert.Equal(t, "M", state.StatusLetter(state.FileStatusModified))
	assert.Equal(t, "D", state.StatusLetter(state.FileStatusDeleted))
	assert.Equal(t, "R", state.StatusLetter(state.FileStatusRenamed))
}

// treeFile is a file of a tree test: only its path and counters matter to the tree.
func treeFile(path string, additions int, deletions int) state.FileDiff {
	return state.FileDiff{Path: path, Status: state.FileStatusModified, Additions: additions, Deletions: deletions}
}

func TestTreeOrdersAggregatesAndCompacts(t *testing.T) {
	t.Parallel()
	files := []state.FileDiff{
		treeFile("zeta.md", 1, 0),
		treeFile("src/modules/b.rs", 2, 1),
		treeFile("src/modules/a.rs", 3, 0),
		treeFile("src/main.rs", 4, 4),
		treeFile("docs/deep/only/x.md", 5, 0),
		treeFile("alpha.md", 1, 1),
	}
	tree := state.BuildTree(files)
	names := make([]string, 0, len(tree))
	for _, node := range tree {
		names = append(names, node.Name)
	}
	assert.Equal(t, []string{"docs/deep/only", "src", "alpha.md", "zeta.md"}, names)
	assert.Equal(t, "docs/deep/only", tree[0].Path)
	src := tree[1]
	assert.Equal(t, 9, src.Additions)
	assert.Equal(t, 5, src.Deletions)
	require.True(t, src.Dir)
	childNames := make([]string, 0, len(src.Children))
	for _, child := range src.Children {
		childNames = append(childNames, child.Name)
	}
	assert.Equal(t, []string{"modules", "main.rs"}, childNames)
	assert.Equal(t, "src/modules", src.Children[0].Path)
	leaves := src.Children[0].Children
	require.Len(t, leaves, 2)
	assert.False(t, leaves[0].Dir)
	assert.Equal(t, 2, leaves[0].FileIndex)
	assert.Equal(t, 1, leaves[1].FileIndex)
}

func TestTreeCompactsRootChain(t *testing.T) {
	t.Parallel()
	tree := state.BuildTree([]state.FileDiff{treeFile("src/modules/a.rs", 1, 0)})
	require.Len(t, tree, 1)
	assert.Equal(t, "src/modules", tree[0].Name)
	assert.Equal(t, "src/modules", tree[0].Path)
}
