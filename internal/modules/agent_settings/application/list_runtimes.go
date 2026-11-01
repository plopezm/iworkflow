package application

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

// ListRuntimes lists the runtimes the user can pick: the supported ones whose executable is installed.
type ListRuntimes struct {
	availability outbound.RuntimeAvailabilityPort
}

// NewListRuntimes builds the use case.
func NewListRuntimes(availability outbound.RuntimeAvailabilityPort) *ListRuntimes {
	return &ListRuntimes{availability: availability}
}

// Execute returns the installed runtimes in selector order. It never returns nil.
func (u *ListRuntimes) Execute(ctx context.Context) []models.AgentRuntime {
	all := models.AllRuntimes()
	installed := make([]models.AgentRuntime, 0, len(all))
	for _, runtime := range all {
		if u.availability.Installed(ctx, runtime) {
			installed = append(installed, runtime)
		}
	}
	return installed
}
