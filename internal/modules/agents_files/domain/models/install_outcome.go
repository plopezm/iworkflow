package models

// InstallOutcome is the result of writing an AGENTS.md into a folder.
type InstallOutcome struct {
	// Committed is true when the file was committed to a git repository.
	Committed bool
}
