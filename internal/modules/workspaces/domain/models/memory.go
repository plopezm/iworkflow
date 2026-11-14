package models

// MemoryState tells what the memory chip of a workspace can show.
type MemoryState string

const (
	// MemoryAvailable means the memory provider answered and the workspace has memories.
	MemoryAvailable MemoryState = "available"
	// MemoryEmpty means the memory provider answered but the workspace has no memories yet.
	MemoryEmpty MemoryState = "empty"
	// MemoryUnavailable means the memory provider could not be read. It is a normal state, not an error.
	MemoryUnavailable MemoryState = "unavailable"
	// MemoryDisabled means no memory provider is wired, so the row shows no memory chip at all.
	MemoryDisabled MemoryState = "disabled"
)

// MemorySummary is what the memory provider remembers about one workspace, without any content.
type MemorySummary struct {
	State MemoryState
	Count int
}

// WorkspaceWithMemory pairs a registered workspace with its memory summary.
type WorkspaceWithMemory struct {
	Workspace Workspace
	Memory    MemorySummary
}
