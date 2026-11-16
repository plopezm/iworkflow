package application

import (
	"context"
	"fmt"
	"slices"

	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// MaxListedMemories is how many memories the memories page of a workspace lists.
const MaxListedMemories = 200

// ListMemories lists the memories of the project matched to a workspace, newest first, without their content.
type ListMemories struct {
	store outbound.MemoryStore
}

// NewListMemories builds the use case.
func NewListMemories(store outbound.MemoryStore) *ListMemories {
	return &ListMemories{store: store}
}

// Execute returns up to MaxListedMemories memories, newest first. An unreachable Engram yields StateUnavailable and a
// workspace without a project yields StateNoMatch, neither is an error.
func (u *ListMemories) Execute(ctx context.Context, workspace models.Workspace) (models.MemoryFeed, error) {
	project, state, err := resolveWorkspace(ctx, u.store, workspace)
	if err != nil {
		return models.MemoryFeed{}, err
	}
	if state != models.StateAvailable {
		return models.MemoryFeed{State: state}, nil
	}
	recent, err := u.store.Recent(ctx, project.Name, MaxListedMemories)
	if err != nil {
		if isUnavailable(err) {
			return models.MemoryFeed{State: models.StateUnavailable}, nil
		}
		return models.MemoryFeed{}, fmt.Errorf("list engram memories: %w", err)
	}
	memories := slices.Clone(recent)
	slices.SortStableFunc(memories, func(a, b models.Memory) int {
		return b.LastSeenAt.Compare(a.LastSeenAt)
	})
	if len(memories) > MaxListedMemories {
		memories = memories[:MaxListedMemories]
	}
	return models.MemoryFeed{State: models.StateAvailable, Memories: memories}, nil
}
