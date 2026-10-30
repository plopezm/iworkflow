package state

import (
	"strings"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// rank orders the statuses by severity: a folder shows the most severe status below it (pure).
func rank(kind models.GitChangeKind) uint8 {
	switch kind {
	case models.GitChangeConflicted:
		return 5
	case models.GitChangeDeleted:
		return 4
	case models.GitChangeModified:
		return 3
	case models.GitChangeRenamed:
		return 2
	default:
		return 1
	}
}

// StatusMap gives the status of every path, folders included: a folder takes the most severe status
// below it (pure).
func StatusMap(changes []models.GitChange) map[string]models.GitChangeKind {
	statuses := make(map[string]models.GitChangeKind)
	put := func(path string, kind models.GitChangeKind) {
		current, known := statuses[path]
		if !known || rank(kind) > rank(current) {
			statuses[path] = kind
		}
	}
	for _, change := range changes {
		put(change.Path, change.Kind)
		for index := range len(change.Path) {
			if change.Path[index] == '/' {
				put(change.Path[:index], change.Kind)
			}
		}
	}
	return statuses
}

// StatusClass is the CSS modifier of a status; empty when the path is clean (pure).
func StatusClass(kind models.GitChangeKind, known bool) string {
	if !known {
		return ""
	}
	switch kind {
	case models.GitChangeModified, models.GitChangeRenamed:
		return "git-modified"
	case models.GitChangeAdded, models.GitChangeUntracked:
		return "git-added"
	case models.GitChangeDeleted:
		return "git-deleted"
	case models.GitChangeConflicted:
		return "git-conflicted"
	default:
		return ""
	}
}

// Openable reports whether a changed file can be opened: a deleted one has nothing left to show.
func Openable(kind models.GitChangeKind) bool {
	return kind != models.GitChangeDeleted
}

// KindLetter is the one-letter status shown at the end of a change row (pure).
func KindLetter(kind models.GitChangeKind) string {
	switch kind {
	case models.GitChangeModified:
		return "M"
	case models.GitChangeAdded:
		return "A"
	case models.GitChangeUntracked:
		return "U"
	case models.GitChangeDeleted:
		return "D"
	case models.GitChangeRenamed:
		return "R"
	case models.GitChangeConflicted:
		return "!"
	default:
		return ""
	}
}

// FileName is the last segment of a `/`-separated path (pure).
func FileName(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[index+1:]
	}
	return path
}

// ParentDir is the directory part of a `/`-separated path, empty for a file at the root (pure).
func ParentDir(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[:index]
	}
	return ""
}
