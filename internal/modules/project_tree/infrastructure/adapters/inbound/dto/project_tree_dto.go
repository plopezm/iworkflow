// Package dto holds the shapes of the project_tree screens that cross the HTTP boundary, and their
// mapping from the domain. Domain models never leave the module without passing through here.
package dto

import "iworkflow/internal/modules/project_tree/domain/models"

// GitChangeKindDTO is the wire name of a git change kind. The values are the Rust variant names.
type GitChangeKindDTO string

// Wire names of the git change kinds.
const (
	GitChangeKindModified   GitChangeKindDTO = "Modified"
	GitChangeKindAdded      GitChangeKindDTO = "Added"
	GitChangeKindUntracked  GitChangeKindDTO = "Untracked"
	GitChangeKindDeleted    GitChangeKindDTO = "Deleted"
	GitChangeKindRenamed    GitChangeKindDTO = "Renamed"
	GitChangeKindConflicted GitChangeKindDTO = "Conflicted"
)

// TreeEntryDTO is one child of a listed directory.
type TreeEntryDTO struct {
	Name string `json:"name"`
	// Path is relative to the project root, `/`-separated.
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// ProjectDirectoryDTO is one listed directory level.
type ProjectDirectoryDTO struct {
	WorkspaceName string `json:"workspace_name"`
	// Path is relative to the project root; empty means the root itself.
	Path    string         `json:"path"`
	Entries []TreeEntryDTO `json:"entries"`
}

// GitChangeDTO is one changed path of the project.
type GitChangeDTO struct {
	// Path is relative to the project root, `/`-separated.
	Path string           `json:"path"`
	Kind GitChangeKindDTO `json:"kind"`
	// Staged means the path is part of the next commit.
	Staged bool `json:"staged"`
	// Unstaged means the path differs from the index (or is untracked).
	Unstaged bool `json:"unstaged"`
}

// ProjectFileDTO is one text file of the project.
type ProjectFileDTO struct {
	Name string `json:"name"`
	// Path is relative to the project root, `/`-separated.
	Path    string `json:"path"`
	Content string `json:"content"`
}

// FromTreeEntry maps one directory child.
func FromTreeEntry(entry models.TreeEntry) TreeEntryDTO {
	return TreeEntryDTO{
		Name:  entry.Name,
		Path:  entry.Path,
		IsDir: entry.Kind == models.TreeEntryDirectory,
	}
}

// FromProjectDirectory maps a listed directory level. An empty listing maps to an empty slice, so
// it is sent as [] and not null.
func FromProjectDirectory(directory models.ProjectDirectory) ProjectDirectoryDTO {
	entries := make([]TreeEntryDTO, 0, len(directory.Contents.Entries))
	for _, entry := range directory.Contents.Entries {
		entries = append(entries, FromTreeEntry(entry))
	}
	return ProjectDirectoryDTO{
		WorkspaceName: directory.WorkspaceName,
		Path:          directory.Contents.Path,
		Entries:       entries,
	}
}

// FromGitChange maps one changed path.
func FromGitChange(change models.GitChange) GitChangeDTO {
	return GitChangeDTO{
		Path:     change.Path,
		Kind:     fromGitChangeKind(change.Kind),
		Staged:   change.Staged,
		Unstaged: change.Unstaged,
	}
}

// fromGitChangeKind maps the closed set of domain kinds. An unknown kind maps to "", which the
// client treats as an unknown kind rather than as a valid one.
func fromGitChangeKind(kind models.GitChangeKind) GitChangeKindDTO {
	switch kind {
	case models.GitChangeModified:
		return GitChangeKindModified
	case models.GitChangeAdded:
		return GitChangeKindAdded
	case models.GitChangeUntracked:
		return GitChangeKindUntracked
	case models.GitChangeDeleted:
		return GitChangeKindDeleted
	case models.GitChangeRenamed:
		return GitChangeKindRenamed
	case models.GitChangeConflicted:
		return GitChangeKindConflicted
	default:
		return ""
	}
}

// FromProjectFile maps one text file.
func FromProjectFile(file models.ProjectFile) ProjectFileDTO {
	return ProjectFileDTO{Name: file.Name, Path: file.Path, Content: file.Content}
}
