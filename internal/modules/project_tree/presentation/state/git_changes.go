package state

import (
	"slices"
	"strings"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// ChangeGroups splits the changes by what a commit would do with them.
type ChangeGroups struct {
	// Staged is part of the next commit.
	Staged []models.GitChange
	// Unstaged holds tracked files modified in the working tree and not (fully) staged.
	Unstaged []models.GitChange
	// Untracked holds new files git does not know yet.
	Untracked []models.GitChange
}

// Empty reports whether there is nothing to show.
func (g ChangeGroups) Empty() bool {
	return len(g.Staged) == 0 && len(g.Unstaged) == 0 && len(g.Untracked) == 0
}

// GroupChanges splits the changes into the three groups, each sorted by path. A partially staged file
// is in both Staged and Unstaged (pure).
func GroupChanges(changes []models.GitChange) ChangeGroups {
	var groups ChangeGroups
	for _, change := range changes {
		if change.Staged {
			groups.Staged = append(groups.Staged, change)
		}
		if change.Unstaged {
			if change.Kind == models.GitChangeUntracked {
				groups.Untracked = append(groups.Untracked, change)
			} else {
				groups.Unstaged = append(groups.Unstaged, change)
			}
		}
	}
	for _, list := range [][]models.GitChange{groups.Staged, groups.Unstaged, groups.Untracked} {
		slices.SortFunc(list, func(a models.GitChange, b models.GitChange) int {
			return strings.Compare(a.Path, b.Path)
		})
	}
	return groups
}

// GitView is what the changes panel shows: the scope, the grouped changes, and the messages of the
// last load or action.
type GitView struct {
	Scope  models.ProjectScope
	Groups ChangeGroups
	// LoadError replaces the groups when the changes could not be read.
	LoadError string
	// ActionError is the message of the last stage, unstage or commit that failed.
	ActionError string
}

// PathsOf returns the paths of a group, for the bulk actions (pure).
func PathsOf(changes []models.GitChange) []string {
	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.Path)
	}
	return paths
}
