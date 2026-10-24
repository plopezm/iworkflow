//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"

	"github.com/a-h/templ"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/project_tree/presentation/components"
	"iworkflow/internal/modules/project_tree/presentation/pages"
	"iworkflow/internal/modules/project_tree/presentation/state"
)

// TreeTab is the dock route of a workspace root: /workspaces/{id}.
func (h *Handlers) TreeTab() dock.Route {
	return dock.Route{
		Name:    "project-tree",
		Pattern: "/workspaces/{id}",
		Icon:    "▤",
		Title: func(ctx context.Context, _ dock.Params) string {
			return pages.TabTitle.T(ctx)
		},
		Render: func(ctx context.Context, params dock.Params) templ.Component {
			return h.RenderTree(ctx, params["id"])
		},
	}
}

// FileTab is the dock route of one file of a workspace: /workspaces/{id}/files/{path...}. The tab takes
// the name of the file.
func (h *Handlers) FileTab() dock.Route {
	return dock.Route{
		Name:    "project-file",
		Pattern: "/workspaces/{id}/files/{path...}",
		Icon:    "◫",
		Title: func(_ context.Context, params dock.Params) string {
			return state.FileName(params["path"])
		},
		Render: func(ctx context.Context, params dock.Params) templ.Component {
			return h.RenderFile(ctx, params["id"], params["path"])
		},
	}
}

// RenderTree is the page of a project root. It takes the wire form of the scope, as the route does.
func (h *Handlers) RenderTree(ctx context.Context, rawScope string) templ.Component {
	scope, err := dto.ParseScope(rawScope)
	if err != nil {
		return coreui.ErrorText(err.Error())
	}
	// A failed listing is shown inside the tree, so its error is not a second failure here.
	view, _ := h.rootView(ctx, scope)
	return pages.ProjectTreePage(state.NewTreePage(scope, view))
}

// RenderFile is the page of one file of a scope. A file that cannot be read shows its message.
func (h *Handlers) RenderFile(ctx context.Context, rawScope string, path string) templ.Component {
	scope, err := dto.ParseScope(rawScope)
	if err != nil {
		return coreui.ErrorText(err.Error())
	}
	view, err := h.fileView(ctx, scope, path, nil)
	if err != nil {
		return pages.ProjectFilePage(state.FileView{Scope: scope, Path: path, LoadError: err.Error()})
	}
	return pages.ProjectFilePage(view)
}

// RenderChanges is the changes panel of a scope, for screens that embed it (task views).
func (h *Handlers) RenderChanges(ctx context.Context, scope models.ProjectScope) templ.Component {
	return components.GitChangesPanel(components.GitChangesPanelProps{View: h.gitView(ctx, scope)})
}
