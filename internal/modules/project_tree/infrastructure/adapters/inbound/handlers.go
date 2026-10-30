// Package inbound adapts HTTP to the project_tree use cases. Handlers decode the request, call the use
// cases and render the result as htmx fragments. They hold no business logic.
//
// Every fragment endpoint answers with a fragment in every case. A failure is rendered as its message
// with the status of the error: the shell swaps 4xx and 5xx fragments (responseHandling in ui.js), so the
// message reaches the user without an htmx error event.
package inbound

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/project_tree/application"
	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/presentation/state"
)

// UseCases are the use cases the handlers call, one per former server function.
type UseCases struct {
	ListDirectory *application.ListProjectDirectory
	ReadFile      *application.ReadProjectFile
	WriteFile     *application.WriteProjectFile
	GitChanges    *application.ListProjectGitChanges
	FileBase      *application.ReadProjectFileBase
	StagePaths    *application.StageProjectPaths
	UnstagePaths  *application.UnstageProjectPaths
	CommitChanges *application.CommitProjectChanges
	SearchFiles   *application.SearchProjectFiles
}

// Handlers serves the project tree, the file editor, the changes panel and the file search.
type Handlers struct {
	uc UseCases
}

// NewHandlers builds the handlers on top of the use cases.
func NewHandlers(uc UseCases) *Handlers {
	return &Handlers{uc: uc}
}

// Routes mounts the fragment endpoints. Each one replaces a former Rust server function:
//
//	GET  /project-tree/tree             list_project_directory (root, reload)
//	GET  /project-tree/directory        list_project_directory (children of a folder)
//	GET  /project-tree/file/body        read_project_file (editor as on disk: reload, discard)
//	POST /project-tree/file             write_project_file (save, then the editor)
//	POST /project-tree/file/state       read_project_file_base (line marks and toolbar while typing)
//	GET  /project-tree/git/changes      list_project_git_changes (panel body)
//	POST /project-tree/git/stage        stage_project_paths
//	POST /project-tree/git/unstage      unstage_project_paths
//	POST /project-tree/git/commit       commit_project_changes
//	GET  /project-tree/search           search_project_files (composer `@` references)
func (h *Handlers) Routes(r chi.Router) {
	r.Get(state.PathTree, h.serveTree)
	r.Get(state.PathDirectory, h.serveDirectory)
	r.Get(state.PathFileBody, h.serveFileBody)
	r.Post(state.PathFile, h.serveSaveFile)
	r.Post(state.PathFileState, h.serveFileState)
	r.Get(state.PathGitChanges, h.serveChanges)
	r.Post(state.PathGitStage, h.serveStage)
	r.Post(state.PathGitUnstage, h.serveUnstage)
	r.Post(state.PathGitCommit, h.serveCommit)
	r.Get(state.PathSearch, h.serveSearch)
}

// errorStatus is the HTTP status of a failure: missing things are 404, bad input is 400, a commit git
// refused is 422, and infrastructure failures are 500.
func errorStatus(err error) int {
	switch {
	case errors.Is(err, treeerrors.WorkspaceNotFound()), errors.Is(err, treeerrors.PathNotFound("")):
		return http.StatusNotFound
	case errors.Is(err, treeerrors.CommitFailed("")):
		return http.StatusUnprocessableEntity
	case errors.Is(err, treeerrors.IO("")), errors.Is(err, treeerrors.WorkspaceLookup("")):
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

// statusFor is the status of a fragment that may carry an error: 200 when there is none.
func statusFor(err error) int {
	if err == nil {
		return http.StatusOK
	}
	return errorStatus(err)
}

// renderError answers with the message of err in a fragment, with the status of the error.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	web.Render(w, r, errorStatus(err), coreui.ErrorText(err.Error()))
}

// statusesOf is the git status of every path of the scope. When git cannot be read the rows stay
// uncolored, as the Rust version did.
func (h *Handlers) statusesOf(ctx context.Context, scope models.ProjectScope) map[string]models.GitChangeKind {
	changes, err := h.uc.GitChanges.Execute(ctx, scope)
	if err != nil {
		return nil
	}
	return state.StatusMap(changes)
}
