package models

import (
	"path/filepath"
	"strings"
)

// MatchProject returns the Engram project of a workspace. A project matches first when one of its
// directories equals the workspace path or lies under it; only when no directory matches does the
// project name equal the workspace name, ignoring case. The first matching project in input order wins.
func MatchProject(projects []MemoryProject, workspace Workspace) (MemoryProject, bool) {
	if project, ok := matchByDirectory(projects, workspace.Path); ok {
		return project, true
	}
	return matchByName(projects, workspace.Name)
}

func matchByDirectory(projects []MemoryProject, workspacePath string) (MemoryProject, bool) {
	if workspacePath == "" {
		return MemoryProject{}, false
	}
	root := filepath.Clean(workspacePath)
	for _, project := range projects {
		for _, directory := range project.Directories {
			if directory == "" {
				continue
			}
			if isWithin(filepath.Clean(directory), root) {
				return project, true
			}
		}
	}
	return MemoryProject{}, false
}

func matchByName(projects []MemoryProject, workspaceName string) (MemoryProject, bool) {
	if workspaceName == "" {
		return MemoryProject{}, false
	}
	for _, project := range projects {
		if strings.EqualFold(project.Name, workspaceName) {
			return project, true
		}
	}
	return MemoryProject{}, false
}

// isWithin reports whether dir equals root or is a descendant of it. Whole path elements only:
// "/a/apple" is not within "/a/app".
func isWithin(dir string, root string) bool {
	if dir == root {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(dir, prefix)
}
