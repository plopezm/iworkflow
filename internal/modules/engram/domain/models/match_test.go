package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchProject(t *testing.T) {
	projects := []MemoryProject{
		NewMemoryProject("apple", 1, []string{"/a/apple"}),
		NewMemoryProject("app", 2, []string{"/a/app/"}),
		NewMemoryProject("Shared", 3, []string{"/x/shared-dir"}),
	}
	tests := []struct {
		name      string
		workspace Workspace
		wantName  string
		wantOK    bool
	}{
		{name: "directory equal to the workspace path", workspace: NewWorkspace("zzz", "/a/app"), wantName: "app", wantOK: true},
		{name: "workspace path with a trailing slash", workspace: NewWorkspace("zzz", "/a/app/"), wantName: "app", wantOK: true},
		{name: "directory under the workspace path", workspace: NewWorkspace("zzz", "/a"), wantName: "apple", wantOK: true},
		{name: "sibling with a shared prefix is not under the path", workspace: NewWorkspace("zzz", "/a/appl"), wantOK: false},
		{name: "name fallback ignores case", workspace: NewWorkspace("shared", "/nowhere"), wantName: "Shared", wantOK: true},
		{name: "empty workspace path skips directory matching", workspace: NewWorkspace("zzz", ""), wantOK: false},
		{name: "empty workspace name skips name matching", workspace: NewWorkspace("", "/nowhere"), wantOK: false},
		{name: "no directory and no name match", workspace: NewWorkspace("nope", "/nowhere"), wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := MatchProject(projects, tc.workspace)

			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantName, got.Name)
		})
	}
}

func TestNewMemoryProjectCopiesDirectories(t *testing.T) {
	dirs := []string{"/a"}

	project := NewMemoryProject("p", 1, dirs)
	dirs[0] = "/mutated"

	assert.Equal(t, []string{"/a"}, project.Directories)
}
