// Package workspaceroot resolves project scopes to root directories through the lookups that the
// composition root wires from the workspaces and tasks modules. This module never imports them.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package workspaceroot

import (
	"context"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// ResolverAdapter implements WorkspaceRootPort by delegating to one lookup per scope kind.
type ResolverAdapter struct {
	workspaces outbound.WorkspaceRootLookup
	tasks      outbound.WorkspaceRootLookup
}

var _ outbound.WorkspaceRootPort = (*ResolverAdapter)(nil)

// NewResolverAdapter builds the adapter. Both lookups are required.
func NewResolverAdapter(workspaces outbound.WorkspaceRootLookup, tasks outbound.WorkspaceRootLookup) *ResolverAdapter {
	return &ResolverAdapter{workspaces: workspaces, tasks: tasks}
}

// Resolve asks the lookup of the scope kind. A failing lookup is a WorkspaceLookup error; an
// unknown identifier is WorkspaceNotFound.
func (a *ResolverAdapter) Resolve(ctx context.Context, scope models.ProjectScope) (models.WorkspaceRoot, error) {
	lookup := a.workspaces
	if scope.IsTask() {
		lookup = a.tasks
	}
	if lookup == nil {
		return models.WorkspaceRoot{}, treeerrors.WorkspaceLookup("no hay búsqueda configurada")
	}
	found, err := lookup.Find(ctx, scope.ID())
	if err != nil {
		return models.WorkspaceRoot{}, treeerrors.WorkspaceLookup(err.Error())
	}
	if found == nil {
		return models.WorkspaceRoot{}, treeerrors.WorkspaceNotFound()
	}
	return *found, nil
}
