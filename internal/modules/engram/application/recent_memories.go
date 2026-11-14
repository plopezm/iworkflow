package application

import (
	"context"
	"fmt"
	"slices"

	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// MaxRecentMemories is how many memories a workspace feed shows.
const MaxRecentMemories = 5

// RecentMemories lists the most recent memories of the project matched to a workspace.
type RecentMemories struct {
	store outbound.MemoryStore
}

// NewRecentMemories builds the use case.
func NewRecentMemories(store outbound.MemoryStore) *RecentMemories {
	return &RecentMemories{store: store}
}

// Execute returns up to MaxRecentMemories memories, newest first. An unreachable Engram yields
// StateUnavailable, not an error. Other failures are returned wrapped.
func (u *RecentMemories) Execute(ctx context.Context, workspace models.Workspace) (models.MemoryFeed, error) {
	project, state, err := resolveWorkspace(ctx, u.store, workspace)
	if err != nil {
		return models.MemoryFeed{}, err
	}
	if state != models.StateAvailable {
		return models.MemoryFeed{State: state}, nil
	}
	recent, err := u.store.Recent(ctx, project.Name, MaxRecentMemories)
	if err != nil {
		if isUnavailable(err) {
			return models.MemoryFeed{State: models.StateUnavailable}, nil
		}
		return models.MemoryFeed{}, fmt.Errorf("read engram recent memories: %w", err)
	}
	memories := slices.Clone(recent)
	slices.SortStableFunc(memories, func(a, b models.Memory) int {
		return b.LastSeenAt.Compare(a.LastSeenAt)
	})
	if len(memories) > MaxRecentMemories {
		memories = memories[:MaxRecentMemories]
	}
	return models.MemoryFeed{State: models.StateAvailable, Memories: memories}, nil
}
