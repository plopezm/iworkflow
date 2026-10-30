//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/modules/terminal/application"
	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/terminal/presentation/components"
	"iworkflow/internal/modules/terminal/presentation/pages"
	"iworkflow/internal/modules/terminal/presentation/state"
)

// fragmentStatus is the status of every htmx response, including the domain errors. htmx 2 does not
// swap 4xx or 5xx responses by default, so an error would never reach the user; the message is the
// fragment itself. Changing it needs a responseHandling rule in web/static/js/ui.js first.
const fragmentStatus int = http.StatusOK

// defaultSize is the size of the terminals opened from the screens: 80 columns by 24 rows, as in the
// Rust version. The values are valid by construction; a test checks them through NewTerminalSize.
var defaultSize = models.TerminalSize{Cols: 80, Rows: 24}

// UseCases are the use cases the terminal screens and their htmx endpoints need.
type UseCases struct {
	List          *application.ListTerminals
	OpenWorkspace *application.OpenTerminal
	OpenTask      *application.OpenTaskTerminal
	OpenHome      *application.OpenHomeTerminal
	Close         *application.CloseTerminal
}

// Handlers serves the terminal tabs and their htmx requests. They decode the input, call the use
// cases and render fragments; they hold no business logic.
type Handlers struct {
	uc UseCases
}

// NewHandlers builds the handlers over the use cases.
func NewHandlers(uc UseCases) *Handlers {
	return &Handlers{uc: uc}
}

// Mount registers the htmx endpoints. The tab pages are served by the dock, see Tabs.
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/terminal/sessions", h.listSessions)
	r.Post("/terminal/home", h.openHome)
	r.Post("/terminal/workspaces/{id}/open", h.openWorkspace)
	r.Post("/terminal/tasks/{id}/open", h.openTask)
	r.Post("/terminal/sessions/{session_id}/close", h.closeFromList)
	r.Post("/terminal/sessions/{session_id}/close/page", h.closeFromPage)
}

// listSessions answers with the current table of sessions.
func (h *Handlers) listSessions(w http.ResponseWriter, r *http.Request) {
	writeFragments(w, r, components.SessionsList(h.sessionsView(r.Context(), nil)))
}

// openHome opens a terminal in $HOME. Success redirects the tab to the session; failure shows the message
// in the notice above the list.
func (h *Handlers) openHome(w http.ResponseWriter, r *http.Request) {
	session, err := h.uc.OpenHome.Execute(r.Context(), defaultSize)
	if err != nil {
		writeFragments(w, r, shared.ErrorText(err.Error()))
		return
	}
	writeFragments(w, r, components.DockRedirect(state.SessionPath(session.ID.String())))
}

// openWorkspace opens a terminal in the directory of the workspace of the path.
func (h *Handlers) openWorkspace(w http.ResponseWriter, r *http.Request) {
	id, err := dto.ParseWorkspaceID(chi.URLParam(r, "id"))
	if err == nil {
		var session models.TerminalSession
		session, err = h.uc.OpenWorkspace.Execute(r.Context(), id, defaultSize)
		if err == nil {
			writeFragments(w, r, components.DockRedirect(state.SessionPath(session.ID.String())))
			return
		}
	}
	writeFragments(w, r, pages.OpenFailed(r.Context(), err.Error(), pages.BackToWorkspaces()))
}

// openTask opens a terminal in the worktree of the task of the path.
func (h *Handlers) openTask(w http.ResponseWriter, r *http.Request) {
	id, err := dto.ParseTaskID(chi.URLParam(r, "id"))
	if err == nil {
		var session models.TerminalSession
		session, err = h.uc.OpenTask.Execute(r.Context(), id, defaultSize)
		if err == nil {
			writeFragments(w, r, components.DockRedirect(state.SessionPath(session.ID.String())))
			return
		}
	}
	writeFragments(w, r, pages.OpenFailed(r.Context(), err.Error(), pages.BackToBoard()))
}

// closeFromList closes a session from the sessions table. The answer is the refreshed table, with the
// error of the attempt when it failed.
func (h *Handlers) closeFromList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := dto.ParseSessionID(chi.URLParam(r, "session_id"))
	if err == nil {
		err = h.uc.Close.Execute(ctx, id)
	}
	writeFragments(w, r, components.SessionsList(h.sessionsView(ctx, err)))
}

// closeFromPage closes the session shown in its own tab. Success returns the tab to the list; failure
// shows the message above the terminal.
func (h *Handlers) closeFromPage(w http.ResponseWriter, r *http.Request) {
	id, err := dto.ParseSessionID(chi.URLParam(r, "session_id"))
	if err == nil {
		err = h.uc.Close.Execute(r.Context(), id)
	}
	if err != nil {
		writeFragments(w, r, shared.ErrorText(err.Error()))
		return
	}
	writeFragments(w, r, components.DockRedirect(state.TerminalsPath))
}

// sessionsView reads the sessions and maps them to the table. actionErr is the error of the action that
// led here, if any; a failed read replaces the table with its message.
func (h *Handlers) sessionsView(ctx context.Context, actionErr error) state.ListView {
	sessions, err := h.uc.List.Execute(ctx)
	if err != nil {
		return state.NewListView(ctx, nil, errorText(actionErr), err.Error())
	}
	return state.NewListView(ctx, dto.FromTerminalSessions(sessions), errorText(actionErr), "")
}

// errorText is the message of err, or the empty string when there is no error.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// writeFragments answers with the fragments in order, with the fragment status.
func writeFragments(w http.ResponseWriter, r *http.Request, fragments ...templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(fragmentStatus)
	for _, fragment := range fragments {
		if err := fragment.Render(r.Context(), w); err != nil {
			return
		}
	}
}
