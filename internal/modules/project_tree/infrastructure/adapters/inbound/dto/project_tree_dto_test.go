package dto

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestMapsGitChangeToDTO(t *testing.T) {
	result := FromGitChange(models.GitChange{Path: "a.rs", Kind: models.GitChangeUntracked, Staged: false, Unstaged: true})

	require.Equal(t, GitChangeDTO{Path: "a.rs", Kind: GitChangeKindUntracked, Staged: false, Unstaged: true}, result)
}

func TestMapsEveryGitChangeKindToItsWireName(t *testing.T) {
	cases := map[models.GitChangeKind]GitChangeKindDTO{
		models.GitChangeModified:   GitChangeKindModified,
		models.GitChangeAdded:      GitChangeKindAdded,
		models.GitChangeUntracked:  GitChangeKindUntracked,
		models.GitChangeDeleted:    GitChangeKindDeleted,
		models.GitChangeRenamed:    GitChangeKindRenamed,
		models.GitChangeConflicted: GitChangeKindConflicted,
	}
	for kind, want := range cases {
		require.Equal(t, want, FromGitChange(models.GitChange{Kind: kind}).Kind)
	}
	require.Empty(t, FromGitChange(models.GitChange{Kind: 0}).Kind)
}

func TestMapsFileToDTO(t *testing.T) {
	result := FromProjectFile(models.ProjectFile{Name: "a.rs", Path: "src/a.rs", Content: "x"})

	require.Equal(t, ProjectFileDTO{Name: "a.rs", Path: "src/a.rs", Content: "x"}, result)
}

func TestMapsDirectoryToDTO(t *testing.T) {
	directory := models.ProjectDirectory{
		WorkspaceName: "demo",
		Contents: models.DirectoryContents{
			Path: "src",
			Entries: []models.TreeEntry{
				{Name: "ui", Path: "src/ui", Kind: models.TreeEntryDirectory},
				{Name: "main.rs", Path: "src/main.rs", Kind: models.TreeEntryFile},
			},
		},
	}

	result := FromProjectDirectory(directory)

	require.Equal(t, ProjectDirectoryDTO{
		WorkspaceName: "demo",
		Path:          "src",
		Entries: []TreeEntryDTO{
			{Name: "ui", Path: "src/ui", IsDir: true},
			{Name: "main.rs", Path: "src/main.rs", IsDir: false},
		},
	}, result)
}

func TestEmptyDirectoryMapsToAnEmptySliceNotNil(t *testing.T) {
	result := FromProjectDirectory(models.ProjectDirectory{WorkspaceName: "demo"})

	require.NotNil(t, result.Entries)
	require.Empty(t, result.Entries)
}
