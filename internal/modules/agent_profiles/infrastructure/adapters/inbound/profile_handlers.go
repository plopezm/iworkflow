package inbound

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strings"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/presentation/components"
	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

// serveSelect selects a profile in the list: the list is the swap target, the detail goes out of band.
func (h *Handlers) serveSelect(w http.ResponseWriter, r *http.Request) {
	view := h.profilesView(r.Context(), r.FormValue("selected"), false)
	respond(w, r, http.StatusOK, components.ProfilesList(view.List, false), components.ProfileDetail(view.Detail, true))
}

// serveDetail renders the card of one profile. ?confirm=delete opens the delete confirmation.
func (h *Handlers) serveDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name, err := profileName(r)
	if err != nil {
		respondDetailError(w, r, err)
		return
	}
	view := h.profilesView(ctx, name.String(), r.URL.Query().Get("confirm") == "delete")
	if view.List.Error != "" {
		respond(w, r, http.StatusInternalServerError, components.DetailError(view.List.Error))
		return
	}
	if !view.Detail.Present || view.Detail.Name != name.String() {
		respondDetailError(w, r, agenterrors.ProfileNotFound(name.String()))
		return
	}
	respond(w, r, http.StatusOK, components.ProfileDetail(view.Detail, false))
}

func (h *Handlers) serveNewDialog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	respond(w, r, http.StatusOK, components.NameDialog(nameDialogView(ctx, state.DialogNewTitle, profilesURL(), "", "", "")))
}

func (h *Handlers) serveDuplicateDialog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	source, err := models.ParseProfileName(r.URL.Query().Get("source"))
	if err != nil {
		respond(w, r, statusFor(err), coreui.ErrorText(err.Error()))
		return
	}
	copyName := state.CopySuffix.Fmt(ctx, map[string]string{"name": source.String()})
	respond(w, r, http.StatusOK, components.NameDialog(nameDialogView(ctx, state.DialogDuplicateTitle, duplicateURL(source.String()), copyName, r.URL.Query().Get("selected"), "")))
}

func (h *Handlers) serveRenameDialog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	source, err := models.ParseProfileName(r.URL.Query().Get("source"))
	if err != nil {
		respond(w, r, statusFor(err), coreui.ErrorText(err.Error()))
		return
	}
	respond(w, r, http.StatusOK, components.NameDialog(nameDialogView(ctx, state.DialogRenameTitle, renameURL(source.String()), source.String(), r.URL.Query().Get("selected"), "")))
}

func (h *Handlers) serveImportDialog(w http.ResponseWriter, r *http.Request) {
	respond(w, r, http.StatusOK, components.ImportDialog(state.ImportDialogView{ID: profilesDialogID, Action: importURL()}))
}

// serveRoleDialog opens the role dialog for the screen named by ?scope.
func (h *Handlers) serveRoleDialog(w http.ResponseWriter, r *http.Request) {
	scope, err := parseScope(r.URL.Query().Get("scope"))
	if err != nil {
		respond(w, r, statusFor(err), coreui.ErrorText(err.Error()))
		return
	}
	view := roleDialogView(r.Context(), scope, r.URL.Query().Get("selected"), "", "", "", "")
	respond(w, r, http.StatusOK, components.RoleDialog(view))
}

// serveNameHint answers the live hint of a profile name while the user types.
func (h *Handlers) serveNameHint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.FormValue("name")
	respond(w, r, http.StatusOK,
		components.NameHintText(profilesScope, hintIfTyped(ctx, name)),
		components.NameSubmit(profilesScope, state.AcceptLabel.T(ctx), state.SlugProblemFound(name), true),
	)
}

// serveRoleHint answers the live hint of a role name while the user types.
func (h *Handlers) serveRoleHint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scope, err := parseScope(r.FormValue("scope"))
	if err != nil {
		respond(w, r, statusFor(err), coreui.ErrorText(err.Error()))
		return
	}
	name := r.FormValue("name")
	prefix := scope + "-role"
	respond(w, r, http.StatusOK,
		components.NameHintText(prefix, hintIfTyped(ctx, name)),
		components.NameSubmit(prefix, state.CreateRoleLabel.T(ctx), state.SlugProblemFound(name), true),
	)
}

func (h *Handlers) serveCreateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	typed := r.PostFormValue("name")
	name, err := models.ParseProfileName(typed)
	if err != nil {
		renderNameDialogError(w, r, state.DialogNewTitle, profilesURL(), typed, "", err)
		return
	}
	created, err := h.uses.CreateProfile.Execute(ctx, name)
	if err != nil {
		renderNameDialogError(w, r, state.DialogNewTitle, profilesURL(), typed, "", err)
		return
	}
	h.respondProfilesChanged(w, r, created.Name.String(), state.ProfileCreated)
}

func (h *Handlers) serveDuplicateProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	source, err := profileName(r)
	if err != nil {
		respondDetailError(w, r, err)
		return
	}
	typed := r.PostFormValue("name")
	target, err := models.ParseProfileName(typed)
	if err == nil {
		_, err = h.uses.DuplicateProfile.Execute(ctx, source, target)
	}
	if err != nil {
		renderNameDialogError(w, r, state.DialogDuplicateTitle, duplicateURL(source.String()), typed, r.PostFormValue("selected"), err)
		return
	}
	h.respondProfilesChanged(w, r, target.String(), state.ProfileDuplicated)
}

func (h *Handlers) serveRenameProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	from, err := profileName(r)
	if err != nil {
		respondDetailError(w, r, err)
		return
	}
	typed := r.PostFormValue("name")
	selected := r.PostFormValue("selected")
	to, err := models.ParseProfileName(typed)
	if err == nil {
		_, err = h.uses.RenameProfile.Execute(ctx, from, to)
	}
	if err != nil {
		renderNameDialogError(w, r, state.DialogRenameTitle, renameURL(from.String()), typed, selected, err)
		return
	}
	next := state.NextSelection(selected, nil, state.SelectionEvent{Kind: state.SelectionRenamed, From: from.String(), To: to.String()})
	h.respondProfilesChanged(w, r, next, state.ProfileRenamed)
}

// serveDeleteProfile deletes a profile. The selection moves to the neighbour the Rust screen chose.
func (h *Handlers) serveDeleteProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name, err := profileName(r)
	if err != nil {
		respondDetailError(w, r, err)
		return
	}
	store, err := h.uses.ListProfiles.Execute(ctx)
	if err != nil {
		respondDetailError(w, r, err)
		return
	}
	names, _ := profileNames(store)
	if err := h.uses.DeleteProfile.Execute(ctx, name); err != nil {
		view := h.profilesView(ctx, name.String(), false)
		view.Detail.Error = err.Error()
		respond(w, r, statusFor(err), components.ProfileDetail(view.Detail, false))
		return
	}
	next := state.NextSelection(r.FormValue("selected"), names, state.SelectionEvent{Kind: state.SelectionDeleted, From: name.String()})
	view := h.profilesView(ctx, next, false)
	respond(w, r, http.StatusOK, components.ProfileDetail(view.Detail, false), components.ProfilesList(view.List, true), toast(state.ProfileDeleted.T(ctx)))
}

// serveSaveProfileRouting saves the routing of a profile. A failed save keeps the rows the user typed.
func (h *Handlers) serveSaveProfileRouting(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name, err := profileName(r)
	if err != nil {
		respondDetailError(w, r, err)
		return
	}
	if !parseForm(w, r) {
		return
	}
	rows := rowsFromForm(r)
	cat := h.loadCatalog(ctx)
	profile, err := h.uses.SetProfileRouting.Execute(ctx, name, routingFromView(state.RoutingFromRows(rows)))
	if err != nil {
		editor := profileEditor(name.String(), rows, cat)
		editor.Error = err.Error()
		respond(w, r, statusFor(err), components.RoutingEditor(editor))
		return
	}
	saved := state.RowsFromRouting(routingToView(profile.Routing))
	respond(w, r, http.StatusOK, components.RoutingEditor(profileEditor(name.String(), saved, cat)), toast(state.RoutingSaved.T(ctx)))
}

// serveProfileRoutingRows adds or removes a row of the routing editor, keeping the typed draft.
func (h *Handlers) serveProfileRoutingRows(w http.ResponseWriter, r *http.Request) {
	name, err := profileName(r)
	if err != nil {
		respondDetailError(w, r, err)
		return
	}
	if !parseForm(w, r) {
		return
	}
	rows := applyRowAction(r, rowsFromForm(r))
	respond(w, r, http.StatusOK, components.RoutingEditor(profileEditor(name.String(), rows, h.loadCatalog(r.Context()))))
}

// serveExport answers the download of a profile: the JSON envelope, named <name>.profile.json.
func (h *Handlers) serveExport(w http.ResponseWriter, r *http.Request) {
	name, err := profileName(r)
	if err != nil {
		http.Error(w, err.Error(), statusFor(err))
		return
	}
	content, err := h.uses.ExportProfile.Execute(r.Context(), name)
	if err != nil {
		http.Error(w, err.Error(), statusFor(err))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": exportFilename(name)}))
	_, _ = io.WriteString(w, content) // the status is already sent: a failed write cannot be reported
}

// serveExportNotice is sent by the Export link. The browser downloads the file; this reply shows the
// "Exported" toast, or the error when the export fails.
func (h *Handlers) serveExportNotice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name, err := profileName(r)
	if err == nil {
		_, err = h.uses.ExportProfile.Execute(ctx, name)
	}
	if err != nil {
		respond(w, r, http.StatusOK, toast(err.Error()))
		return
	}
	respond(w, r, http.StatusOK, toast(state.Exported.Fmt(ctx, map[string]string{"file": exportFilename(name)})))
}

// serveJSON shows the exported JSON in a read-only box, for copying by hand.
func (h *Handlers) serveJSON(w http.ResponseWriter, r *http.Request) {
	name, err := profileName(r)
	if err != nil {
		respondError(w, r, err)
		return
	}
	content, err := h.uses.ExportProfile.Execute(r.Context(), name)
	if err != nil {
		respondError(w, r, err)
		return
	}
	respond(w, r, http.StatusOK, components.JSONPanel(content))
}

// serveImportProfile imports a profile from the uploaded file or the pasted JSON.
func (h *Handlers) serveImportProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input, err := readImportForm(r)
	if err != nil {
		respond(w, r, http.StatusBadRequest, components.ImportDialog(importDialogView(input, err)))
		return
	}
	var renameTo *models.ProfileName
	if trimmed := strings.TrimSpace(input.RenameTo); trimmed != "" {
		parsed, parseErr := models.ParseProfileName(trimmed)
		if parseErr != nil {
			respond(w, r, statusFor(parseErr), components.ImportDialog(importDialogView(input, parseErr)))
			return
		}
		renameTo = &parsed
	}
	imported, err := h.uses.ImportProfile.Execute(ctx, input.Raw, renameTo)
	if err != nil {
		respond(w, r, statusFor(err), components.ImportDialog(importDialogView(input, err)))
		return
	}
	h.respondProfilesChanged(w, r, imported.Name.String(), state.ProfileImported)
}

// respondProfilesChanged answers a change of the profile list: the dialog closes, the list and the
// selected profile are refreshed out of band, and the notice is shown.
func (h *Handlers) respondProfilesChanged(w http.ResponseWriter, r *http.Request, selected string, message i18n.Msg) {
	view := h.profilesView(r.Context(), selected, false)
	respond(w, r, http.StatusOK,
		components.DialogHost(profilesDialogID),
		components.ProfilesList(view.List, true),
		components.ProfileDetail(view.Detail, true),
		toast(message.T(r.Context())),
	)
}

// renderNameDialogError re-renders the name dialog with the message of the failure.
func renderNameDialogError(w http.ResponseWriter, r *http.Request, title i18n.Msg, action string, value string, selected string, err error) {
	view := nameDialogView(r.Context(), title, action, value, selected, err.Error())
	respond(w, r, statusFor(err), components.NameDialog(view))
}

// respondDetailError replaces the detail card with the message of the failure.
func respondDetailError(w http.ResponseWriter, r *http.Request, err error) {
	respond(w, r, statusFor(err), components.DetailError(err.Error()))
}

// nameDialogView builds the dialog that asks for a profile name. The hint is shown only once the
// user has typed something; the submit button is disabled from the start while the name is invalid.
func nameDialogView(ctx context.Context, title i18n.Msg, action string, value string, selected string, errText string) state.NameDialogView {
	return state.NameDialogView{
		ID:          profilesDialogID,
		Title:       title.T(ctx),
		SubmitLabel: state.AcceptLabel.T(ctx),
		Action:      action,
		HintURL:     nameHintURL(),
		Value:       value,
		Selected:    selected,
		Hint:        hintIfTyped(ctx, value),
		Invalid:     state.SlugProblemFound(value),
		Error:       errText,
	}
}

// importDialogView keeps what the user typed when the import fails.
func importDialogView(input importInput, err error) state.ImportDialogView {
	return state.ImportDialogView{
		ID:       profilesDialogID,
		Action:   importURL(),
		Raw:      input.Raw,
		RenameTo: input.RenameTo,
		Error:    err.Error(),
	}
}

// hintIfTyped is the hint of a name: empty while nothing is typed.
func hintIfTyped(ctx context.Context, value string) string {
	if value == "" {
		return ""
	}
	return state.SlugHint(ctx, value)
}

// exportFilename is the download name of a profile.
func exportFilename(name models.ProfileName) string {
	return name.String() + ".profile.json"
}

// respondError answers a failure with its status and its message, for requests that have no region
// of their own to re-render.
func respondError(w http.ResponseWriter, r *http.Request, err error) {
	respond(w, r, statusFor(err), coreui.ErrorText(err.Error()))
}

// parseForm reads the form of a request; a malformed body is a 400.
func parseForm(w http.ResponseWriter, r *http.Request) bool {
	if err := r.ParseForm(); err != nil {
		respond(w, r, http.StatusBadRequest, coreui.ErrorText(err.Error()))
		return false
	}
	return true
}
