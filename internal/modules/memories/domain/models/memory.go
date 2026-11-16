// Package models holds the value objects of the memories module: a read-only view of what a memory provider remembers
// about a workspace. Nothing here names a concrete provider.
package models

import "time"

// State tells whether the memory provider could answer for a workspace.
type State string

const (
	// StateAvailable means the provider answered for the workspace.
	StateAvailable State = "available"
	// StateUnavailable means the provider is off or cannot be reached.
	StateUnavailable State = "unavailable"
	// StateEmpty means the provider answered but holds nothing for the workspace.
	StateEmpty State = "empty"
)

// Workspace identifies the workspace whose memories are read.
type Workspace struct {
	ID   string
	Name string
	Path string
}

// Memory is one memory in a list, without its content.
type Memory struct {
	ID        int
	Type      string
	Title     string
	UpdatedAt time.Time
}

// Feed is the list of memories of a workspace, newest first.
type Feed struct {
	State    State
	Memories []Memory
}

// Entry is one memory with its content. Detail is meaningful only when State is available.
type Entry struct {
	State   State
	Memory  Memory
	Content string
}

// TaskRef identifies the task whose branch memories are read. StartedAt is when the task began; Started is false
// while the task is still in the backlog and has no branch.
type TaskRef struct {
	ID          string
	WorkspaceID string
	Title       string
	Branch      string
	Started     bool
	StartedAt   time.Time
}

// TaskFeed is the memories of a task's workspace in two groups, each newest first: the ones written since the task
// started (its branch) and the earlier ones.
type TaskFeed struct {
	State   State
	Branch  []Memory
	Earlier []Memory
}

// SplitSince splits a feed at the start of the task. A memory counts as the branch's when it was written at or after
// startedAt. A task that has not started has no branch memories.
func SplitSince(feed Feed, task TaskRef) TaskFeed {
	split := TaskFeed{State: feed.State}
	for _, memory := range feed.Memories {
		if task.Started && !memory.UpdatedAt.Before(task.StartedAt) {
			split.Branch = append(split.Branch, memory)
		} else {
			split.Earlier = append(split.Earlier, memory)
		}
	}
	return split
}
