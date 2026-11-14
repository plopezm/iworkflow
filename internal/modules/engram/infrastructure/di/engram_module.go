// Package di is the composition root of the engram module: it validates the configured Engram URL, builds the HTTP
// adapter and the use cases.
package di

import (
	"fmt"

	"iworkflow/internal/modules/engram/application"
	"iworkflow/internal/modules/engram/infrastructure/adapters/outbound/api"
)

// Module exposes the read-only use cases of the engram module.
type Module struct {
	ListWorkspacesMemory *application.ListWorkspacesMemory
	RecentMemories       *application.RecentMemories
}

// NewModule wires the Engram HTTP client into the use cases. baseURL must be a value ResolveBaseURL accepted: the
// module never applies a default address on its own.
func NewModule(baseURL string) (*Module, error) {
	client, err := api.NewClient(baseURL)
	if err != nil {
		return nil, fmt.Errorf("build engram module: %w", err)
	}
	return &Module{
		ListWorkspacesMemory: application.NewListWorkspacesMemory(client),
		RecentMemories:       application.NewRecentMemories(client),
	}, nil
}
