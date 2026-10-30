//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/workspaces/presentation/components"
	"iworkflow/internal/modules/workspaces/presentation/pages"
	"iworkflow/internal/modules/workspaces/presentation/state"
)

// fragmentStatus is the status of every htmx response, including the domain errors. htmx 2 does not
// swap 4xx or 5xx responses by default, so an error would never reach the user; the message is the
// fragment itself. Changing it needs a responseHandling rule in web/static/js/ui.js first.
const fragmentStatus = http.StatusOK

// Handlers serves the workspaces tabs and their htmx requests.
type Handlers struct {
	uc UseCases
}

// NewHandlers builds the handlers over the use cases.
func NewHandlers(uc UseCases) *Handlers {
	return &Handlers{uc: uc}
}

// Mount registers the htmx endpoints. The tab pages are served by the dock, see ListTab and EditTab.
func (h *Handlers) Mount(r chi.Router) {
	r.Post("/workspaces/register", h.register)
	r.Get("/workspaces/register/dismiss", h.dismissPrompt)
	r.Post("/workspaces/{id}/update", h.update)
	r.Post("/workspaces/{id}/delete", h.delete)
	r.Get("/workspaces/directories", h.openPicker)
	r.Get("/workspaces/directories/entries", h.pickerEntries)
	r.Get("/workspaces/directories/select", h.selectDirectory)
	r.Post("/workspaces/directories/create", h.createDirectory)
}

// register registers the folder. It answers with the registration slot (form, notices, confirmation)
// and, on success, with the refreshed table as an out-of-band swap.
func (h *Handlers) register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		writeFragments(w, r, components.WorkspaceRegister(state.RegisterForm{Error: err.Error()}))
		return
	}
	input := dto.RegisterWorkspaceFromForm(r.PostForm)
	workspace, err := h.uc.Register.Execute(ctx, input.ToNewWorkspace())
	outcome, err := dto.FromRegisterOutcome(workspace, err)
	form := h.registerForm(ctx, input)

	switch {
	case err != nil:
		form.Error = err.Error()
		writeFragments(w, r, components.WorkspaceRegister(form))
	case outcome.NeedsInitialization != nil:
		form.Prompt = &state.InitializationPrompt{
			Path:         outcome.NeedsInitialization.Path,
			Reason:       string(outcome.NeedsInitialization.Reason),
			Name:         input.Name,
			BaseBranch:   deref(input.BaseBranch),
			AgentsFileID: deref(input.AgentsFileID),
		}
		writeFragments(w, r, components.WorkspaceRegister(form))
	default:
		table := h.table(ctx, nil)
		table.OutOfBand = true
		writeFragments(w, r,
			components.WorkspaceRegister(h.registerForm(ctx, dto.RegisterWorkspaceDTO{})),
			components.WorkspaceTable(table))
	}
}

// dismissPrompt removes the initialization prompt without sending anything.
func (h *Handlers) dismissPrompt(w http.ResponseWriter, r *http.Request) {
	writeFragments(w, r, components.EmptyRegisterPrompt())
}

// update edits a workspace. Success replaces the edit tab with the list; failure shows the message.
func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeFragments(w, r, shared.ErrorText(err.Error()))
		return
	}
	id, err := models.ParseWorkspaceID(chi.URLParam(r, "id"))
	if err == nil {
		input := dto.UpdateWorkspaceFromForm(r.PostForm)
		_, err = h.uc.Update.Execute(r.Context(), id, input.ToWorkspaceUpdate())
	}
	if err != nil {
		writeFragments(w, r, shared.ErrorText(err.Error()))
		return
	}
	writeFragments(w, r, pages.EditSaved())
}

// delete removes a workspace and answers with the new table, with the error of the attempt if it failed.
func (h *Handlers) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := models.ParseWorkspaceID(chi.URLParam(r, "id"))
	if err == nil {
		err = h.uc.Delete.Execute(ctx, id)
	}
	writeFragments(w, r, components.WorkspaceTable(h.table(ctx, err)))
}

// openPicker opens the folder picker at the requested path (the home folder when it is empty).
func (h *Handlers) openPicker(w http.ResponseWriter, r *http.Request) {
	picker := h.picker(r.Context(), strings.TrimSpace(r.URL.Query().Get("path")))
	writeFragments(w, r, components.DirectoryPicker(picker))
}

// pickerEntries answers with the sub-folders of a path, filtered by name. It replaces only the list,
// so the filter input keeps its focus while the user types.
func (h *Handlers) pickerEntries(w http.ResponseWriter, r *http.Request) {
	picker := h.picker(r.Context(), strings.TrimSpace(r.URL.Query().Get("path")))
	picker.Entries = state.FilterEntries(picker.Entries, r.URL.Query().Get("filter"))
	writeFragments(w, r, components.PickerEntries(picker))
}

// selectDirectory closes the picker and sets the chosen path in the registration form.
func (h *Handlers) selectDirectory(w http.ResponseWriter, r *http.Request) {
	chosen := strings.TrimSpace(r.URL.Query().Get("path"))
	writeFragments(w, r, components.DirectoryPicker(state.DirectoryPicker{}), components.PathInput(chosen, true))
}

// createDirectory creates a folder inside the open one and navigates to it. On failure the picker
// stays on the parent and shows the message.
func (h *Handlers) createDirectory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		writeFragments(w, r, components.DirectoryPicker(state.DirectoryPicker{Open: true, Error: err.Error()}))
		return
	}
	parent := strings.TrimSpace(r.PostForm.Get("parent"))
	created, err := h.uc.CreateDirectory.Execute(ctx, parent, r.PostForm.Get("name"))
	if err != nil {
		picker := h.picker(ctx, parent)
		picker.Error = err.Error()
		writeFragments(w, r, components.DirectoryPicker(picker))
		return
	}
	writeFragments(w, r, components.DirectoryPicker(h.picker(ctx, created)))
}

// picker lists path for the picker. A failure is shown in the picker instead of being returned.
func (h *Handlers) picker(ctx context.Context, path string) state.DirectoryPicker {
	listing, err := h.uc.BrowseDirectory.Execute(ctx, path)
	if err != nil {
		return state.DirectoryPicker{Open: true, Path: path, Error: err.Error()}
	}
	view := dto.FromDirectoryListing(listing)
	entries := make([]state.DirectoryEntry, 0, len(view.Entries))
	for _, entry := range view.Entries {
		entries = append(entries, state.DirectoryEntry{Name: entry.Name, Path: entry.Path, IsGitRepo: entry.IsGitRepo})
	}
	return state.DirectoryPicker{Open: true, Path: view.Path, Parent: deref(view.Parent), Entries: entries}
}

// table lists the workspaces. deleteErr is the error of the attempt that produced this table, if any.
func (h *Handlers) table(ctx context.Context, deleteErr error) state.TableView {
	view := state.TableView{}
	if deleteErr != nil {
		view.Error = deleteErr.Error()
	}
	workspaces, err := h.uc.List.Execute(ctx)
	if err != nil {
		view.Error = err.Error()
		return view
	}
	for _, workspace := range workspaces {
		view.Rows = append(view.Rows, workspaceRow(dto.FromWorkspace(workspace)))
	}
	return view
}

// registerForm builds the registration form, echoing the submitted values and offering the stored
// AGENTS.md files. A failure to list them leaves the options empty, as the Rust version did.
func (h *Handlers) registerForm(ctx context.Context, input dto.RegisterWorkspaceDTO) state.RegisterForm {
	return state.RegisterForm{
		Name:         input.Name,
		Path:         input.Path,
		BaseBranch:   deref(input.BaseBranch),
		AgentsFileID: deref(input.AgentsFileID),
		Options:      h.agentsOptions(ctx),
	}
}

func (h *Handlers) agentsOptions(ctx context.Context) []state.AgentsOption {
	options, err := h.uc.ListAgentsFiles.Execute(ctx)
	if err != nil {
		return nil
	}
	return agentsOptions(options)
}

// agentsOptions maps the stored AGENTS.md files to the drop-down options.
func agentsOptions(options []models.AgentsFileOption) []state.AgentsOption {
	converted := make([]state.AgentsOption, 0, len(options))
	for _, option := range options {
		view := dto.FromAgentsFileOption(option)
		converted = append(converted, state.AgentsOption{Value: view.ID, Label: view.Description})
	}
	return converted
}

func workspaceRow(workspace dto.WorkspaceDTO) state.WorkspaceRow {
	return state.WorkspaceRow{
		ID:           workspace.ID,
		Name:         workspace.Name,
		Path:         workspace.Path,
		BaseBranch:   workspace.BaseBranch,
		EditHref:     "/workspaces/" + workspace.ID + "/edit",
		TerminalHref: "/workspaces/" + workspace.ID + "/terminal",
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// writeFragments renders the components one after another as a single htmx response.
func writeFragments(w http.ResponseWriter, r *http.Request, fragments ...templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(fragmentStatus)
	for _, fragment := range fragments {
		if err := fragment.Render(r.Context(), w); err != nil {
			return
		}
	}
}
