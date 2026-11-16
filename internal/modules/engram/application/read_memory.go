package application

import (
	"context"
	"errors"
	"fmt"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// ReadMemory reads one memory with its content, only when it belongs to the project of the workspace.
type ReadMemory struct {
	store outbound.MemoryStore
}

// NewReadMemory builds the use case.
func NewReadMemory(store outbound.MemoryStore) *ReadMemory {
	return &ReadMemory{store: store}
}

// Execute returns the memory. A memory of another project is reported as engramerrors.ErrNotFound, exactly like a
// missing one, so a workspace can never read the memories of another. An unreachable Engram yields StateUnavailable.
func (u *ReadMemory) Execute(ctx context.Context, workspace models.Workspace, id int) (models.MemoryEntry, error) {
	project, state, err := resolveWorkspace(ctx, u.store, workspace)
	if err != nil {
		return models.MemoryEntry{}, err
	}
	if state != models.StateAvailable {
		return models.MemoryEntry{State: state}, nil
	}
	detail, err := u.store.Observation(ctx, id)
	if err != nil {
		if isUnavailable(err) {
			return models.MemoryEntry{State: models.StateUnavailable}, nil
		}
		if errors.Is(err, engramerrors.ErrNotFound) {
			return models.MemoryEntry{}, fmt.Errorf("memory %d: %w", id, engramerrors.ErrNotFound)
		}
		return models.MemoryEntry{}, fmt.Errorf("read engram memory %d: %w", id, err)
	}
	if detail.Project != project.Name {
		return models.MemoryEntry{}, fmt.Errorf("memory %d: %w", id, engramerrors.ErrNotFound)
	}
	return models.MemoryEntry{State: models.StateAvailable, Detail: detail}, nil
}
