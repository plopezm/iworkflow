package models

// ProjectRef is a project (folder) that can receive an AGENTS.md.
type ProjectRef struct {
	ID   string
	Name string
	Path string
}
