// Package inbound adapts HTTP to the agent_profiles use cases. Handlers decode the request, call one
// or more use cases and render an htmx fragment. They hold no business logic: validation and rules
// stay in the domain and the application layer.
//
//nolint:wrapcheck // Domain errors are rendered by their text, which is user-visible and must stay identical to the Rust version.
package inbound

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/modules/agent_profiles/application"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/presentation/pages"
)

// UseCases are the use cases the handlers drive. The composition root builds them.
type UseCases struct {
	ListProfiles      *application.ListProfiles
	CreateProfile     *application.CreateProfile
	DuplicateProfile  *application.DuplicateProfile
	RenameProfile     *application.RenameProfile
	DeleteProfile     *application.DeleteProfile
	SetProfileRouting *application.SetProfileRouting
	ExportProfile     *application.ExportProfile
	ImportProfile     *application.ImportProfile
	ListAgentRoles    *application.ListAgentRoles
	CreateAgentRole   *application.CreateAgentRole
	GetGlobalRouting  *application.GetGlobalRouting
	SetGlobalRouting  *application.SetGlobalRouting
	ListModelOptions  *application.ListModelOptions
}

// Handlers serves the profiles and models tabs and the htmx fragments that swap parts of them.
//
// Error responses carry the status of the error and re-render the region the request targeted, with
// the message in it. The shell swaps 4xx and 5xx fragments (responseHandling in ui.js), so the message
// reaches the user without an error page.
type Handlers struct {
	uses UseCases
}

// NewHandlers builds the handlers on top of the use cases.
func NewHandlers(uses UseCases) *Handlers {
	return &Handlers{uses: uses}
}

// Routes mounts the fragment endpoints. Each one replaces a Rust server function or a client-side
// action of the Rust screens:
//
//	GET    /agent-profiles/select                 chip selection (list + detail)
//	GET    /agent-profiles/dialogs/{kind}         name, import and role dialogs
//	GET    /agent-profiles/name-hint              live hint of a profile name
//	GET    /agent-profiles/role-hint              live hint of a role name
//	POST   /agent-profiles/profiles               create_profile
//	POST   /agent-profiles/profiles/import        import_profile (file or pasted JSON)
//	GET    /agent-profiles/profiles/{name}/detail the detail card (with ?confirm=delete)
//	POST   /agent-profiles/profiles/{name}/duplicate  duplicate_profile
//	POST   /agent-profiles/profiles/{name}/rename     rename_profile
//	DELETE /agent-profiles/profiles/{name}         delete_profile
//	POST   /agent-profiles/profiles/{name}/routing set_profile_routing
//	POST   /agent-profiles/profiles/{name}/routing/rows  add or remove a row
//	GET    /agent-profiles/profiles/{name}/export  export_profile (download)
//	GET    /agent-profiles/profiles/{name}/export-notice  the "Exported" toast
//	GET    /agent-profiles/profiles/{name}/json   the JSON panel (copy fallback)
//	POST   /agent-profiles/roles                  create_agent_role
//	POST   /agent-profiles/models/routing         set_global_routing
//	POST   /agent-profiles/models/routing/rows    add or remove a row of the global routing
//
// get_global_routing, list_profiles_store, list_agent_roles, list_model_options and
// list_profile_thinking_levels are rendered inside the screens, so they need no endpoint.
func (h *Handlers) Routes(r chi.Router) {
	r.Get(selectURL(), h.serveSelect)
	r.Get(basePath+"/dialogs/new", h.serveNewDialog)
	r.Get(basePath+"/dialogs/duplicate", h.serveDuplicateDialog)
	r.Get(basePath+"/dialogs/rename", h.serveRenameDialog)
	r.Get(basePath+"/dialogs/import", h.serveImportDialog)
	r.Get(basePath+"/dialogs/role", h.serveRoleDialog)
	r.Get(nameHintURL(), h.serveNameHint)
	r.Get(roleHintURL(), h.serveRoleHint)
	r.Post(profilesURL(), h.serveCreateProfile)
	r.Post(importURL(), h.serveImportProfile)
	r.Get(basePath+"/profiles/{name}/detail", h.serveDetail)
	r.Post(basePath+"/profiles/{name}/duplicate", h.serveDuplicateProfile)
	r.Post(basePath+"/profiles/{name}/rename", h.serveRenameProfile)
	r.Delete(basePath+"/profiles/{name}", h.serveDeleteProfile)
	r.Post(basePath+"/profiles/{name}/routing", h.serveSaveProfileRouting)
	r.Post(basePath+"/profiles/{name}/routing/rows", h.serveProfileRoutingRows)
	r.Get(basePath+"/profiles/{name}/export", h.serveExport)
	r.Get(basePath+"/profiles/{name}/export-notice", h.serveExportNotice)
	r.Get(basePath+"/profiles/{name}/json", h.serveJSON)
	r.Post(rolesURL(), h.serveCreateRole)
	r.Post(modelsRoutingURL(), h.serveSaveGlobalRouting)
	r.Post(modelsRoutingRowsURL(), h.serveGlobalRoutingRows)
}

// Tabs returns the dock routes of the two screens. The page is rendered with the state of the moment
// it opens.
func (h *Handlers) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    "agent-profiles",
			Pattern: ProfilesPath,
			Icon:    "☰",
			Title: func(ctx context.Context, _ dock.Params) string {
				return pages.TabTitle.T(ctx)
			},
			Render: func(ctx context.Context, _ dock.Params) templ.Component {
				return pages.ProfilesPage(h.profilesView(ctx, "", false))
			},
		},
		{
			Name:    "agent-models",
			Pattern: ModelsPath,
			Icon:    "◇",
			Title: func(ctx context.Context, _ dock.Params) string {
				return pages.ModelsTitle.T(ctx)
			},
			Render: func(ctx context.Context, _ dock.Params) templ.Component {
				return pages.ModelsPage(h.modelsView(ctx))
			},
		},
	}
}

// profileName reads the {name} segment of the path as a validated profile name.
func profileName(r *http.Request) (models.ProfileName, error) {
	return models.ParseProfileName(chi.URLParam(r, "name"))
}
