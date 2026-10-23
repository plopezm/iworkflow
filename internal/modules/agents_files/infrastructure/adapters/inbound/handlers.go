// Package inbound adapts HTTP to the agents_files use cases. Handlers decode the request, call one use
// case and render the result as an htmx fragment. They hold no business logic.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package inbound

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/agents_files/application"
	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/agents_files/presentation/components"
	"iworkflow/internal/modules/agents_files/presentation/pages"
	"iworkflow/internal/modules/agents_files/presentation/state"
)

// pagePath is the address of the screen; it is also the path of its dock tab.
const pagePath = "/agents-files"

// Handlers serves the AGENTS.md tab and the htmx fragments that swap parts of it.
//
// Error responses carry the status of the error and the error text in a fragment. The page relies on
// htmx swapping error responses (responseHandling in the shell), so each fragment reaches its target.
type Handlers struct {
	list         *application.ListAgentsFiles
	listProjects *application.ListProjects
	upload       *application.UploadAgentsFile
	remove       *application.DeleteAgentsFile
	apply        *application.ApplyAgentsFileToProject
}

// NewHandlers builds the handlers on top of the use cases.
func NewHandlers(
	list *application.ListAgentsFiles,
	listProjects *application.ListProjects,
	upload *application.UploadAgentsFile,
	remove *application.DeleteAgentsFile,
	apply *application.ApplyAgentsFileToProject,
) *Handlers {
	return &Handlers{list: list, listProjects: listProjects, upload: upload, remove: remove, apply: apply}
}

// Tab is the dock route of the screen. The page is rendered with the state of the moment it opens.
func (h *Handlers) Tab() dock.Route {
	return dock.Route{
		Icon:    "§",
		Pattern: pagePath,
		Title: func(ctx context.Context, _ dock.Params) string {
			return pages.TabTitle.T(ctx)
		},
		Render: func(ctx context.Context, _ dock.Params) templ.Component {
			return pages.AgentsFilesPage(state.Page{Files: h.loadFileList(ctx)})
		},
	}
}

// Routes mounts the fragment endpoints. Each one maps to one of the former Rust server functions:
//
//	POST   /agents-files/upload               upload_agents_file
//	DELETE /agents-files/{id}                 delete_agents_file (returns the list)
//	POST   /agents-files/{id}/apply           apply_agents_file (returns the card)
//	GET    /agents-files/{id}/card            the card without feedback (close, cancel)
//	GET    /agents-files/{id}/apply-button    the "Add to project" button for the selected project
//
// The list (list_agents_files) and the projects (list_agents_file_projects) are rendered with the
// page and with the list and card fragments, so they have no endpoint of their own.
func (h *Handlers) Routes(r chi.Router) {
	r.Post("/agents-files/upload", h.serveUpload)
	r.Delete("/agents-files/{id}", h.serveDelete)
	r.Post("/agents-files/{id}/apply", h.serveApply)
	r.Get("/agents-files/{id}/card", h.serveCard)
	r.Get("/agents-files/{id}/apply-button", h.serveApplyButton)
}

func (h *Handlers) serveUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body, err := ReadUpload(r)
	if err != nil {
		form := state.UploadForm{Description: body.Description, Error: state.ErrorText(err)}
		web.Render(w, r, statusFor(err), components.UploadSection(form))
		return
	}
	if _, err := h.upload.Execute(ctx, body.NewAgentsFile()); err != nil {
		form := state.UploadForm{Description: body.Description, Error: state.ErrorText(err)}
		web.Render(w, r, statusFor(err), components.UploadSection(form))
		return
	}
	web.Render(w, r, http.StatusOK, components.UploadDone(h.loadFileList(ctx)))
}

func (h *Handlers) serveDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := agentsFileID(r)
	if err == nil {
		err = h.remove.Execute(ctx, id)
	}
	list := h.loadFileList(ctx)
	if err != nil {
		list.Error = state.ErrorText(err)
		web.Render(w, r, statusFor(err), components.FileList(list))
		return
	}
	web.Render(w, r, http.StatusOK, components.FileList(list))
}

func (h *Handlers) serveApply(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := agentsFileID(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		web.Render(w, r, http.StatusBadRequest, coreui.ErrorText(err.Error()))
		return
	}
	input := dto.ApplyAgentsFileDTO{
		AgentsFileID: id.String(),
		ProjectID:    r.PostForm.Get("project_id"),
		Overwrite:    r.PostForm.Get("overwrite") == "true",
	}
	outcome, applyErr := h.apply.Execute(ctx, id, input.ProjectID, input.Overwrite)
	result, mapErr := dto.ApplyOutcome(outcome, applyErr, input.Overwrite)
	item, err := h.item(ctx, id)
	if err != nil {
		renderError(w, r, err)
		return
	}
	item.SelectedProjectID = input.ProjectID
	if mapErr != nil {
		item.ApplyError = state.ErrorText(mapErr)
		web.Render(w, r, statusFor(mapErr), components.FileItem(item))
		return
	}
	item.Feedback = state.FeedbackFromOutcome(result)
	web.Render(w, r, http.StatusOK, components.FileItem(item))
}

func (h *Handlers) serveCard(w http.ResponseWriter, r *http.Request) {
	id, err := agentsFileID(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	item, err := h.item(r.Context(), id)
	if err != nil {
		renderError(w, r, err)
		return
	}
	item.SelectedProjectID = r.URL.Query().Get("project_id")
	web.Render(w, r, http.StatusOK, components.FileItem(item))
}

func (h *Handlers) serveApplyButton(w http.ResponseWriter, r *http.Request) {
	id, err := agentsFileID(r)
	if err != nil {
		renderError(w, r, err)
		return
	}
	web.Render(w, r, http.StatusOK, components.ApplyButton(id.String(), r.URL.Query().Get("project_id")))
}

// loadFileList reads the stored files with their projects. A failure becomes the list error.
func (h *Handlers) loadFileList(ctx context.Context) state.FileList {
	items, err := h.items(ctx)
	if err != nil {
		return state.FileList{Error: state.ErrorText(err)}
	}
	return state.FileList{Items: items}
}

// items builds the view of every stored file, each with the list of projects.
func (h *Handlers) items(ctx context.Context) ([]state.FileItem, error) {
	files, err := h.list.Execute(ctx)
	if err != nil {
		return nil, err
	}
	projects, err := h.listProjects.Execute(ctx)
	if err != nil {
		return nil, err
	}
	options := make([]state.ProjectOption, 0, len(projects))
	for _, project := range projects {
		mapped := dto.FromProjectRef(project)
		options = append(options, state.ProjectOption{ID: mapped.ID, Name: mapped.Name})
	}
	items := make([]state.FileItem, 0, len(files))
	for _, file := range files {
		mapped := dto.FromAgentsFile(file)
		items = append(items, state.FileItem{
			ID:          mapped.ID,
			Description: mapped.Description,
			Path:        mapped.Path,
			CreatedAt:   mapped.CreatedAt,
			Projects:    options,
		})
	}
	return items, nil
}

// item returns the view of one stored file, or NotFound when it does not exist.
func (h *Handlers) item(ctx context.Context, id models.AgentsFileID) (state.FileItem, error) {
	items, err := h.items(ctx)
	if err != nil {
		return state.FileItem{}, err
	}
	for _, item := range items {
		if item.ID == id.String() {
			return item, nil
		}
	}
	return state.FileItem{}, agentserrors.NotFound(id.String())
}

// agentsFileID reads the {id} segment of the path.
func agentsFileID(r *http.Request) (models.AgentsFileID, error) {
	return models.ParseAgentsFileID(chi.URLParam(r, "id"))
}

// renderError writes the error as a fragment with the status its kind maps to.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	web.Render(w, r, statusFor(err), coreui.ErrorText(state.ErrorText(err)))
}

// statusFor maps a business error to its HTTP status. Validation errors are 400, a missing record
// or project 404, an existing target 409, and anything else (storage, persistence, install) 500.
func statusFor(err error) int {
	var malformed malformedUploadError
	switch {
	case errors.As(err, &malformed),
		errors.Is(err, agentserrors.InvalidDescription()),
		errors.Is(err, agentserrors.EmptyContent()),
		errors.Is(err, agentserrors.NotUTF8()),
		errors.Is(err, agentserrors.TooLarge()),
		errors.Is(err, agentserrors.InvalidID("")):
		return http.StatusBadRequest
	case errors.Is(err, agentserrors.NotFound("")), errors.Is(err, agentserrors.ProjectNotFound("")):
		return http.StatusNotFound
	case errors.Is(err, agentserrors.TargetExists("")):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
