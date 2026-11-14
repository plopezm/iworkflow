package application

import (
	"context"
	"errors"
	"fmt"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// listProjects reads the Engram projects once. It returns the state to show: unavailable when the API cannot
// answer (with a nil error), available with the projects otherwise. Any other failure is returned wrapped.
func listProjects(ctx context.Context, store outbound.MemoryStore) ([]models.MemoryProject, models.MemoryState, error) {
	projects, err := store.Projects(ctx)
	if err != nil {
		if isUnavailable(err) {
			return nil, models.StateUnavailable, nil
		}
		return nil, "", fmt.Errorf("list engram projects: %w", err)
	}
	return projects, models.StateAvailable, nil
}

// resolveWorkspace finds the Engram project of one workspace. It returns the state to show:
// unavailable when the API cannot answer (with a nil error), no-match when no project matches, and
// available with the matched project. Any other failure is returned wrapped.
func resolveWorkspace(
	ctx context.Context,
	store outbound.MemoryStore,
	workspace models.Workspace,
) (models.MemoryProject, models.MemoryState, error) {
	projects, state, err := listProjects(ctx, store)
	if err != nil || state != models.StateAvailable {
		return models.MemoryProject{}, state, err
	}
	project, ok := models.MatchProject(projects, workspace)
	if !ok {
		return models.MemoryProject{}, models.StateNoMatch, nil
	}
	return project, models.StateAvailable, nil
}

// isUnavailable reports whether err means the Engram API cannot serve the request.
func isUnavailable(err error) bool {
	return errors.Is(err, engramerrors.ErrUnavailable)
}
