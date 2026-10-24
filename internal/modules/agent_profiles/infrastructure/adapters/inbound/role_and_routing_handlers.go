// Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
//
//nolint:wrapcheck // see the package comment of handlers.go
package inbound

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/presentation/components"
	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

// serveCreateRole creates an agent role. The reply closes the dialog and refreshes the screen that
// opened it: the roles card on the profiles screen, and the "agent to add" select of the editor.
func (h *Handlers) serveCreateRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !parseForm(w, r) {
		return
	}
	scope, err := parseScope(r.PostFormValue("scope"))
	if err != nil {
		respondError(w, r, err)
		return
	}
	selected := r.PostFormValue("selected")
	name := strings.TrimSpace(r.PostFormValue("name"))
	description := strings.TrimSpace(r.PostFormValue("description"))
	prompt := r.PostFormValue("prompt")
	roleName, err := models.ParseRoleName(name)
	if err == nil {
		err = h.uses.CreateAgentRole.Execute(ctx, models.NewAgentRole{Name: roleName, Description: description, Prompt: prompt})
	}
	if err != nil {
		respond(w, r, statusFor(err), components.RoleDialog(roleDialogView(ctx, scope, selected, name, description, prompt, err.Error())))
		return
	}
	parts := []templ.Component{components.DialogHost(scopeDialogID(scope))}
	if scope == profilesScope {
		view := h.profilesView(ctx, selected, false)
		parts = append(parts, components.RolesCard(view.Roles, true))
		if view.Detail.Present {
			parts = append(parts, components.AddAgentSelect(view.Detail.Routing, true))
		}
	} else {
		parts = append(parts, components.AddAgentSelect(h.modelsView(ctx).Editor, true))
	}
	parts = append(parts, toast(state.RoleCreated.T(ctx)))
	respond(w, r, http.StatusOK, parts...)
}

// serveSaveGlobalRouting saves the global routing. The notice gives the number of agents saved.
func (h *Handlers) serveSaveGlobalRouting(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !parseForm(w, r) {
		return
	}
	rows := rowsFromForm(r)
	cat := h.loadCatalog(ctx)
	routing := routingFromView(state.RoutingFromRows(rows))
	if err := h.uses.SetGlobalRouting.Execute(ctx, routing); err != nil {
		editor := globalEditor(rows, cat)
		editor.Error = err.Error()
		respond(w, r, statusFor(err), components.RoutingEditor(editor))
		return
	}
	saved := state.RowsFromRouting(routingToView(routing))
	respond(w, r, http.StatusOK, components.RoutingEditor(globalEditor(saved, cat)), toast(savedRoutingMessage(ctx, len(routing))))
}

// serveGlobalRoutingRows adds or removes a row of the global routing, keeping the typed draft.
func (h *Handlers) serveGlobalRoutingRows(w http.ResponseWriter, r *http.Request) {
	if !parseForm(w, r) {
		return
	}
	rows := applyRowAction(r, rowsFromForm(r))
	respond(w, r, http.StatusOK, components.RoutingEditor(globalEditor(rows, h.loadCatalog(r.Context()))))
}

// savedRoutingMessage is the notice of a saved global routing, in the grammatical number of agents.
func savedRoutingMessage(ctx context.Context, agents int) string {
	switch agents {
	case 0:
		return state.GlobalRoutingSavedNone.T(ctx)
	case 1:
		return state.GlobalRoutingSavedOne.T(ctx)
	default:
		return state.GlobalRoutingSavedMany.Fmt(ctx, map[string]string{"n": strconv.Itoa(agents)})
	}
}

// parseScope accepts the two screens that own a role dialog. Anything else is a bad request.
func parseScope(scope string) (string, error) {
	switch scope {
	case profilesScope, modelsScope:
		return scope, nil
	default:
		return "", agenterrors.InvalidRouting("ámbito desconocido: " + scope)
	}
}

// scopeDialogID is the dialog host of the screen that owns the scope.
func scopeDialogID(scope string) string {
	if scope == modelsScope {
		return modelsDialogID
	}
	return profilesDialogID
}

// roleDialogView builds the role dialog for the screen named by scope.
func roleDialogView(ctx context.Context, scope string, selected string, name string, description string, prompt string, errText string) state.RoleDialogView {
	return state.RoleDialogView{
		ID:          scopeDialogID(scope),
		Scope:       scope,
		SubmitLabel: state.CreateRoleLabel.T(ctx),
		Action:      rolesURL(),
		HintURL:     roleHintURL(),
		Selected:    selected,
		Name:        name,
		Description: description,
		Prompt:      prompt,
		Hint:        hintIfTyped(ctx, name),
		Invalid:     state.SlugProblemFound(name),
		Error:       errText,
	}
}
