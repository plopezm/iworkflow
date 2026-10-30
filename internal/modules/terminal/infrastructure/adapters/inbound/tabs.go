//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"net/url"

	"github.com/a-h/templ"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/terminal/presentation/pages"
	"iworkflow/internal/modules/terminal/presentation/state"
)

// tabIcon is the glyph of every terminal tab, as in the Rust version.
const tabIcon string = "❯"

// Tabs returns the dock routes of the terminal screens: the sessions list, one session, and the new
// terminal of a workspace or of a task. The new-terminal routes open the session from their placeholder
// and then redirect their tab to the session.
func (h *Handlers) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "terminals",
			Pattern: state.TerminalsPath,
			Icon:    tabIcon,
			Title: func(ctx context.Context, _ dock.Params) string {
				return pages.TerminalsTitle(ctx)
			},
			Render: func(ctx context.Context, _ dock.Params) templ.Component {
				return pages.TerminalsPage(h.sessionsView(ctx, nil))
			},
		},
		{
			Name:    "terminal_session",
			Pattern: state.TerminalsPath + "/{id}",
			Icon:    tabIcon,
			Title: func(ctx context.Context, params dock.Params) string {
				return state.TabTitle(ctx, params["id"])
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.sessionTab(ctx, params["id"])
			},
		},
		{
			Name:    "workspace_terminal",
			Pattern: "/workspaces/{id}/terminal",
			Icon:    tabIcon,
			Title: func(ctx context.Context, _ dock.Params) string {
				return state.TerminalTitle(ctx)
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return pages.Opening(ctx, "/terminal/workspaces/"+url.PathEscape(params["id"])+"/open")
			},
		},
		{
			Name:    "task_terminal",
			Pattern: "/tasks/{id}/terminal",
			Icon:    tabIcon,
			Title: func(ctx context.Context, _ dock.Params) string {
				return state.TerminalTitle(ctx)
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return pages.Opening(ctx, "/terminal/tasks/"+url.PathEscape(params["id"])+"/open")
			},
		},
	}
}

// sessionTab renders one session. The header shows the workspace name when the session is still in the
// list; otherwise it falls back to the short id, as the Rust version did.
func (h *Handlers) sessionTab(ctx context.Context, id string) templ.Component {
	name := ""
	if sessions, err := h.uc.List.Execute(ctx); err == nil {
		for _, session := range dto.FromTerminalSessions(sessions) {
			if session.ID == id {
				name = session.WorkspaceName
				break
			}
		}
	}
	return pages.SessionPage(state.SessionPage{ID: id, Title: state.SessionTitle(ctx, name, id)})
}
