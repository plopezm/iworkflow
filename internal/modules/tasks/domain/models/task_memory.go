package models

import "time"

// TaskMemory is one memory of the Engram project of a task's workspace, as the detail page shows it: its type, its
// title and when it was last seen. It carries no content: the page never renders the body of a memory.
type TaskMemory struct {
	Type       string
	Title      string
	LastSeenAt time.Time
}

// TaskMemoryFeed is the memory panel of a task. Available is false when Engram cannot answer; Memories are newest first
// and empty when the project has none.
type TaskMemoryFeed struct {
	Available bool
	Memories  []TaskMemory
}
