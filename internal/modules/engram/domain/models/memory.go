// Package models holds the value objects of the engram module. Values are read-only once built:
// constructors copy their slice inputs and nothing in the module mutates them afterwards.
package models

import (
	"slices"
	"time"
)

// MemoryState tells whether Engram could answer for a workspace.
type MemoryState string

const (
	// StateAvailable means Engram answered and a project matched the workspace.
	StateAvailable MemoryState = "available"
	// StateUnavailable means the Engram API could not be used.
	StateUnavailable MemoryState = "unavailable"
	// StateNoMatch means Engram answered but no project matches the workspace.
	StateNoMatch MemoryState = "no-match"
)

// MemoryProject is an Engram project as reported by its API. Only the fields the workspace mapping
// and the summaries need are kept.
type MemoryProject struct {
	Name             string
	ObservationCount int
	Directories      []string
}

// NewMemoryProject builds a project and copies the directories.
func NewMemoryProject(name string, observationCount int, directories []string) MemoryProject {
	return MemoryProject{
		Name:             name,
		ObservationCount: observationCount,
		Directories:      slices.Clone(directories),
	}
}

// Memory is one observation without its content. Content is never part of the model.
type Memory struct {
	ID         int
	Type       string
	Title      string
	LastSeenAt time.Time
}

// NewMemory builds a memory summary.
func NewMemory(id int, memoryType string, title string, lastSeenAt time.Time) Memory {
	return Memory{ID: id, Type: memoryType, Title: title, LastSeenAt: lastSeenAt}
}

// Workspace identifies an iworkflow workspace by its name and its absolute directory.
type Workspace struct {
	Name string
	Path string
}

// NewWorkspace builds a workspace reference.
func NewWorkspace(name string, path string) Workspace {
	return Workspace{Name: name, Path: path}
}

// WorkspaceMemory is the Engram summary shown next to a workspace. Count is the memory count of the matched
// project; it is zero when no project matches or Engram cannot answer.
type WorkspaceMemory struct {
	State MemoryState
	Count int
}

// MemoryFeed is the list of recent memories of a workspace, newest first.
type MemoryFeed struct {
	State    MemoryState
	Memories []Memory
}
