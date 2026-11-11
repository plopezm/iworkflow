package application

import (
	"context"
	"fmt"
)

// ProjectOption is a project a hook can be assigned to: its id and the name shown to the user.
type ProjectOption struct {
	ID   string
	Name string
}

// WorkspaceLister gives the projects of the workspaces module. The composition root binds it, so the hooks module
// never imports the workspaces module. It is defined here, where the use cases consume it.
type WorkspaceLister interface {
	Projects(ctx context.Context) ([]ProjectOption, error)
}

// ListProjects returns the projects a hook can be assigned to, for the project choices of the Hooks screen.
type ListProjects struct {
	projects WorkspaceLister
}

// NewListProjects builds the use case.
func NewListProjects(projects WorkspaceLister) *ListProjects {
	return &ListProjects{projects: projects}
}

// Execute returns the projects in the order the workspaces module gives them.
func (u *ListProjects) Execute(ctx context.Context) ([]ProjectOption, error) {
	projects, err := u.projects.Projects(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return projects, nil
}
