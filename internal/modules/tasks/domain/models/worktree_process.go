package models

// The ways a process can use the files of a worktree.
const (
	// ProcessReasonWorkingDirectory: the process runs with its current directory inside the worktree.
	ProcessReasonWorkingDirectory string = "cwd"
	// ProcessReasonExecutable: the process runs a program that lives inside the worktree.
	ProcessReasonExecutable string = "exe"
	// ProcessReasonOpenFile: the process holds a file of the worktree open.
	ProcessReasonOpenFile string = "file"
)

// WorktreeProcess is a process that uses files of a task worktree. PID and Started together identify it, so a
// selection made earlier never reaches a different process that got the same pid.
type WorktreeProcess struct {
	PID  int
	Name string
	// Path is the file that ties the process to the worktree.
	Path string
	// Reason is one of the ProcessReason constants.
	Reason string
	// Started is an opaque token of the moment the process started.
	Started string
}
