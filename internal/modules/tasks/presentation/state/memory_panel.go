package state

// MemoryPanel is the collapsed Memory panel of a task. Scope is the page instance, as in TaskDetailView. Rows are the
// recent memories of the project, newest first; each carries its type, its title and its date, never its content.
type MemoryPanel struct {
	Scope string
	Rows  []MemoryRow
}

// MemoryRow is one memory of the Memory panel, already formatted for the page.
type MemoryRow struct {
	Type  string
	Title string
	Date  string
}
