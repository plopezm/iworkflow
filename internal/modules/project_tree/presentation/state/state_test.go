package state

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/modules/project_tree/domain/models"
)

var (
	workspaceID = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	taskID      = uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
)

func change(path string, kind models.GitChangeKind, staged bool, unstaged bool) models.GitChange {
	return models.GitChange{Path: path, Kind: kind, Staged: staged, Unstaged: unstaged}
}

func TestFileHrefEncodesEachSegmentAndRoutesTasksApart(t *testing.T) {
	require.Equal(t, "/workspaces/"+workspaceID.String()+"/files/src/a%20b/c%23.rs",
		FileHref(models.WorkspaceScope(workspaceID), "src/a b/c#.rs"))
	require.Equal(t, "/tasks/"+taskID.String()+"/files/src/a%20b.rs",
		FileHref(models.TaskScope(taskID), "src/a b.rs"))
}

func TestTerminalAndEditHrefsDependOnTheScopeKind(t *testing.T) {
	require.Equal(t, "/tasks/"+taskID.String()+"/terminal", TerminalHref(models.TaskScope(taskID)))
	require.Equal(t, "/workspaces/"+workspaceID.String()+"/terminal", TerminalHref(models.WorkspaceScope(workspaceID)))

	_, hasEdit := EditHref(models.TaskScope(taskID))
	require.False(t, hasEdit, "tasks have no edit page")
	edit, hasEdit := EditHref(models.WorkspaceScope(workspaceID))
	require.True(t, hasEdit)
	require.Equal(t, "/workspaces/"+workspaceID.String()+"/edit", edit)
}

func TestIDIsStableAndDependsOnEveryPart(t *testing.T) {
	require.Equal(t, ID("pt-file", "w", "a.rs"), ID("pt-file", "w", "a.rs"))
	require.NotEqual(t, ID("pt-file", "w", "a.rs"), ID("pt-file", "w", "b.rs"))
	require.NotEqual(t, ID("pt-file", "w", "a.rs"), ID("pt-git", "w", "a.rs"))
}

func TestURLsCarryTheScopeAndPathEncoded(t *testing.T) {
	require.Equal(t, "/project-tree/directory?path=src%2Fui&scope=w1", DirectoryURL("w1", "src/ui"))
	require.Equal(t, "/project-tree/tree?scope=w1", TreeURL("w1"))
	require.Equal(t, "/project-tree/file/body?path=a+b.rs&scope=task%3At1", FileBodyURL("task:t1", "a b.rs"))
}

func TestPaddingGrowsWithDepthLikeTheRustStyle(t *testing.T) {
	require.Equal(t, "padding-left: 0.75rem", PaddingStyle(0))
	require.Equal(t, "padding-left: 1.875rem", PaddingStyle(1))
	require.Equal(t, "padding-left: 3rem", PaddingStyle(2))
}

func TestDepthCountsSeparators(t *testing.T) {
	require.Equal(t, 0, DepthOf("README.md"))
	require.Equal(t, 1, DepthOf("src/main.rs"))
	require.Equal(t, 2, DepthOf("a/b/c.rs"))
}

func TestTreeRowsLinkFilesAndExpandFolders(t *testing.T) {
	scope := models.WorkspaceScope(workspaceID)
	entries := []models.TreeEntry{
		{Name: "src", Path: "src", Kind: models.TreeEntryDirectory},
		{Name: "README.md", Path: "README.md", Kind: models.TreeEntryFile},
	}
	statuses := map[string]models.GitChangeKind{"README.md": models.GitChangeModified}

	rows := NewRootView(scope, "demo", entries, statuses).Rows

	require.Len(t, rows, 2)
	require.True(t, rows[0].IsDir)
	require.Empty(t, rows[0].Href, "a folder does not open a tab")
	require.Equal(t, "/workspaces/"+workspaceID.String()+"/files/README.md", rows[1].Href)
	require.Equal(t, "git-modified", rows[1].StatusClass)
	require.Empty(t, rows[0].StatusClass)
}

func TestEmptyChildrenIndentTheirMessageUnderTheFolder(t *testing.T) {
	view := NewChildrenView(models.WorkspaceScope(workspaceID), "a/b", nil, nil)

	require.Empty(t, view.Rows)
	require.Equal(t, PaddingStyle(3), view.EmptyStyle)
}

func TestStatusMapFoldsFoldersIntoTheMostSevereStatus(t *testing.T) {
	statuses := StatusMap([]models.GitChange{
		change("src/a/new.rs", models.GitChangeUntracked, false, true),
		change("src/a/edit.rs", models.GitChangeModified, false, true),
		change("docs/x.md", models.GitChangeAdded, true, false),
	})

	require.Equal(t, models.GitChangeModified, statuses["src"])
	require.Equal(t, models.GitChangeModified, statuses["src/a"])
	require.Equal(t, models.GitChangeUntracked, statuses["src/a/new.rs"])
	require.Equal(t, models.GitChangeAdded, statuses["docs"])
	_, known := statuses["tests"]
	require.False(t, known)
}

func TestConflictsWinOverEverything(t *testing.T) {
	statuses := StatusMap([]models.GitChange{
		change("a/m.rs", models.GitChangeModified, false, true),
		change("a/c.rs", models.GitChangeConflicted, false, true),
	})

	require.Equal(t, models.GitChangeConflicted, statuses["a"])
}

func TestClassesGroupStatusesByColor(t *testing.T) {
	require.Empty(t, StatusClass(models.GitChangeModified, false))
	require.Equal(t, "git-added", StatusClass(models.GitChangeUntracked, true))
	require.Equal(t, "git-modified", StatusClass(models.GitChangeRenamed, true))
	require.Equal(t, "git-deleted", StatusClass(models.GitChangeDeleted, true))
}

func TestDeletedFilesAreNotOpenable(t *testing.T) {
	require.False(t, Openable(models.GitChangeDeleted))
	require.True(t, Openable(models.GitChangeModified))
}

func TestChangesAreGroupedByWhatACommitWouldDo(t *testing.T) {
	groups := GroupChanges([]models.GitChange{
		change("z.rs", models.GitChangeModified, false, true),
		change("a.rs", models.GitChangeAdded, true, false),
		change("n.txt", models.GitChangeUntracked, false, true),
		change("both.rs", models.GitChangeModified, true, true),
	})

	require.Equal(t, []string{"a.rs", "both.rs"}, PathsOf(groups.Staged))
	require.Equal(t, []string{"both.rs", "z.rs"}, PathsOf(groups.Unstaged))
	require.Equal(t, []string{"n.txt"}, PathsOf(groups.Untracked))
	require.False(t, groups.Empty())
	require.True(t, GroupChanges(nil).Empty())
}

func TestFileAndDirectoryNamesSplitOnSlashes(t *testing.T) {
	require.Equal(t, "b.rs", FileName("src/a/b.rs"))
	require.Equal(t, "src/a", ParentDir("src/a/b.rs"))
	require.Equal(t, "b.rs", FileName("b.rs"))
	require.Empty(t, ParentDir("b.rs"))
}

func TestKindLettersFollowTheStatus(t *testing.T) {
	require.Equal(t, "M", KindLetter(models.GitChangeModified))
	require.Equal(t, "U", KindLetter(models.GitChangeUntracked))
	require.Equal(t, "!", KindLetter(models.GitChangeConflicted))
}

func TestDirtyOnlyWhenTheTextDiffers(t *testing.T) {
	require.False(t, IsDirty("a", "a"))
	require.True(t, IsDirty("a ", "a"))
}

func TestMarksMarkTheDraftAgainstTheHeadBase(t *testing.T) {
	base := "hola base\n"
	marks := MarksFor(&base, "hola base\nlínea nueva\n")

	require.Equal(t, coreui.LineMark(""), MarkAt(marks, 0))
	require.Equal(t, coreui.LineAdded, MarkAt(marks, 1))
}

func TestNoBaseMeansNoMarks(t *testing.T) {
	require.Empty(t, MarksFor(nil, "anything"))
	require.Equal(t, coreui.LineMark(""), MarkAt(nil, 3))
}

func TestFileViewNameKeyAndDirtiness(t *testing.T) {
	view := FileView{Scope: models.WorkspaceScope(workspaceID), Path: "src/main.rs", Draft: "x", Loaded: "y"}

	require.Equal(t, "main.rs", view.Name())
	require.True(t, view.Dirty())
	require.Equal(t, ID("pt-file", workspaceID.String(), "src/main.rs"), view.Key())
}
