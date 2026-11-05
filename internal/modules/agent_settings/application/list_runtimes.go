package application

import (
	"context"
	"sync"

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

// Execute returns the installed runtimes in selector order. It never returns nil. Each probe may start a
// login shell, so they run concurrently: the wait is that of the slowest one, not the sum of all.
func (u *ListRuntimes) Execute(ctx context.Context) []models.AgentRuntime {
	all := models.AllRuntimes()
	found := make([]bool, len(all))
	var wg sync.WaitGroup
	for index, runtime := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			found[index] = u.availability.Installed(ctx, runtime)
		}()
	}
	wg.Wait()
	installed := make([]models.AgentRuntime, 0, len(all))
	for index, runtime := range all {
		if found[index] {
			installed = append(installed, runtime)
		}
	}
	return installed
}
