// Package models holds the value objects of the logs module.
package models

// LogTail is the end of the application log: the most recent lines, oldest first.
type LogTail struct {
	// Lines are the most recent log lines, oldest first.
	Lines []string
	// Truncated is true when older lines exist that the tail does not include.
	Truncated bool
}
