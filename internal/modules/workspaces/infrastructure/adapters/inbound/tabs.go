//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"

	"github.com/a-h/templ"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/workspaces/presentation/components"
	"iworkflow/internal/modules/workspaces/presentation/pages"
	"iworkflow/internal/modules/workspaces/presentation/state"
)

// ListTab renders the workspaces tab: registration, the picker and the list.
func (h *Handlers) ListTab(ctx context.Context, _ dock.Params) templ.Component {
	return pages.WorkspacesPage(state.WorkspacesPage{
		Form:  h.registerForm(ctx, dto.RegisterWorkspaceDTO{}),
		Table: h.table(ctx, nil),
	})
}

// EditTab renders the edit tab of one workspace. A missing or unknown workspace shows its message.
func (h *Handlers) EditTab(ctx context.Context, params dock.Params) templ.Component {
	id, err := models.ParseWorkspaceID(params["id"])
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	workspace, err := h.uc.Get.Execute(ctx, id)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	page := state.EditPage{
		ID:         workspace.ID.String(),
		Name:       workspace.Name,
		Path:       workspace.Path,
		BaseBranch: workspace.BaseBranch,
	}
	options, listErr := h.uc.ListAgentsFiles.Execute(ctx)
	if listErr != nil {
		page.OptionsError = components.AgentsLoadFailedText(ctx, listErr)
	}
	page.Options = agentsOptions(options)
	return pages.WorkspaceEditPage(page)
}
