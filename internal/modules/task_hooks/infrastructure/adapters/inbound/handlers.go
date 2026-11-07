// Package inbound holds the HTTP adapter of the task_hooks module: the Hooks tab of a workspace and the htmx endpoints
// that change it. Every response is the tab with status 200; the outcome of an action, a rejected hook included, is
// rendered inside it. A malformed form is the only 400. The tab is one fragment whether or not the request came from
// htmx: the dock loads it as a page and the actions swap it in place.
//
//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
	hookcomponents "iworkflow/internal/modules/task_hooks/presentation/components"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// UseCases are the use cases the handlers call.
type UseCases struct {
	List       *application.ListHooks
	Save       *application.SaveHook
	Delete     *application.DeleteHook
	Reorder    *application.ReorderHooks
	SetEnabled *application.SetHookEnabled
	Test       *application.TestHook
}

// Handlers serves the Hooks tab and its actions.
type Handlers struct {
	uc UseCases
}

// NewHandlers builds the handlers over the use cases.
func NewHandlers(uc UseCases) *Handlers { return &Handlers{uc: uc} }

// Mount registers the endpoints. The tab itself is a dock route (Tab), so its address is not mounted here.
//
//	GET  /workspaces/{id}/hooks/panel             the tab without the editor
//	GET  /workspaces/{id}/hooks/editor            the editor: a new hook, a stored one (?hook=) or a changed kind
//	POST /workspaces/{id}/hooks                   save the editor form (create, or edit when it carries an id)
//	POST /workspaces/{id}/hooks/reorder           set the whole order of the hooks
//	POST /workspaces/{id}/hooks/{hook}/enabled    enable the hook when the form carries enabled, disable it otherwise
//	POST /workspaces/{id}/hooks/{hook}/delete     delete the hook
//	POST /workspaces/{id}/hooks/{hook}/test       run the hook once as a test and show the outcome in its row
func (h *Handlers) Mount(r chi.Router) {
	r.Get("/workspaces/{id}/hooks/panel", h.panel)
	r.Get("/workspaces/{id}/hooks/editor", h.editor)
	r.Post("/workspaces/{id}/hooks", h.save)
	r.Post("/workspaces/{id}/hooks/reorder", h.reorder)
	r.Post("/workspaces/{id}/hooks/{hook}/enabled", h.setEnabled)
	r.Post("/workspaces/{id}/hooks/{hook}/delete", h.remove)
	r.Post("/workspaces/{id}/hooks/{hook}/test", h.test)
}

// Tab renders the Hooks tab of the workspace of the route.
func (h *Handlers) Tab(ctx context.Context, params dock.Params) templ.Component {
	return hookcomponents.Panel(h.load(ctx, params["id"], state.View{}, nil))
}

func (h *Handlers) panel(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, h.load(r.Context(), chi.URLParam(r, "id"), state.View{}, nil))
}

// editor opens the editor. Without a kind in the query it starts from the stored hook (?hook=) or from a default
// new hook of the group (?event=&phase=). With a kind, the query is the form as the user left it, so a change of kind
// keeps what was typed.
func (h *Handlers) editor(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	ctx := r.Context()
	query := r.URL.Query()
	var view state.View
	switch {
	case query.Has(dto.FieldKind):
		form := formFromValues(query)
		view = h.load(ctx, workspaceID, state.View{Editor: &form}, nil)
	case query.Get("hook") != "":
		hook, err := h.findHook(ctx, workspaceID, query.Get("hook"))
		if err != nil {
			view = h.load(ctx, workspaceID, state.View{Failure: err.Error()}, nil)
			break
		}
		form := formFromHook(hook)
		view = h.load(ctx, workspaceID, state.View{Editor: &form}, nil)
	default:
		form := defaultForm(query.Get(dto.FieldEvent), query.Get(dto.FieldPhase))
		view = h.load(ctx, workspaceID, state.View{Editor: &form}, nil)
	}
	h.render(w, r, view)
}

func (h *Handlers) save(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	workspaceID := chi.URLParam(r, "id")
	input, err := dto.SaveHookFromForm(r.PostForm, workspaceID)
	notice := state.NoticeSaved
	if err == nil {
		if input.ID != "" {
			notice = state.NoticeUpdated
		}
		_, err = h.uc.Save.Execute(ctx, input)
	}
	if err != nil {
		// The editor stays open with the values as typed, so the user corrects them instead of typing them again.
		form := formFromValues(r.PostForm)
		h.render(w, r, h.load(ctx, workspaceID, state.View{Editor: &form, Failure: err.Error()}, nil))
		return
	}
	h.render(w, r, h.load(ctx, workspaceID, state.View{Notice: notice}, nil))
}

func (h *Handlers) reorder(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	workspaceID := chi.URLParam(r, "id")
	view := state.View{Notice: state.NoticeReordered}
	if err := h.uc.Reorder.Execute(ctx, workspaceID, dto.ReorderFromForm(r.PostForm)); err != nil {
		view = state.View{Failure: err.Error()}
	}
	h.render(w, r, h.load(ctx, workspaceID, view, nil))
}

func (h *Handlers) setEnabled(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	workspaceID := chi.URLParam(r, "id")
	view := state.View{Notice: state.NoticeUpdated}
	if err := h.uc.SetEnabled.Execute(ctx, application.SetHookEnabledInput{
		WorkspaceID: workspaceID, HookID: chi.URLParam(r, "hook"), Enabled: r.PostForm.Has(dto.FieldEnabled),
	}); err != nil {
		view = state.View{Failure: err.Error()}
	}
	h.render(w, r, h.load(ctx, workspaceID, view, nil))
}

func (h *Handlers) remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID := chi.URLParam(r, "id")
	view := state.View{Notice: state.NoticeDeleted}
	if err := h.uc.Delete.Execute(ctx, workspaceID, chi.URLParam(r, "hook")); err != nil {
		view = state.View{Failure: err.Error()}
	}
	h.render(w, r, h.load(ctx, workspaceID, view, nil))
}

// test runs the hook once. A run that could not start is shown in the row with its reason; any other refusal (an
// unknown hook, a hook of another workspace) is the failure of the tab.
func (h *Handlers) test(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID := chi.URLParam(r, "id")
	hookID := chi.URLParam(r, "hook")
	outcome, err := h.uc.Test.Execute(ctx, application.TestHookInput{WorkspaceID: workspaceID, HookID: hookID})
	var view state.View
	switch {
	case errors.Is(err, hookerrors.ErrRunNotStarted):
		view = h.load(ctx, workspaceID, state.View{}, &testReport{hookID: hookID, result: state.TestResult{
			Status: state.ResultError, ExitCode: -1, Failure: err.Error(),
		}})
	case err != nil:
		view = state.View{Failure: err.Error()}
	default:
		view = h.load(ctx, workspaceID, state.View{}, &testReport{hookID: hookID, result: state.TestResult{
			Status: resultStatus(outcome.Status), ExitCode: outcome.ExitCode, Output: outcome.Output,
		}})
	}
	h.render(w, r, view)
}

// load fills the view with the hooks of the workspace in position order and the outcome of a test. A failure to read
// the hooks is shown in the tab, which then has no sections.
func (h *Handlers) load(ctx context.Context, workspaceID string, view state.View, report *testReport) state.View {
	view.WorkspaceID = workspaceID
	hooks, err := h.uc.List.Execute(ctx, workspaceID)
	if err != nil {
		if view.Failure == "" {
			view.Failure = err.Error()
		}
		return view
	}
	view.Sections = sections(workspaceID, hooks, report)
	return view
}

// findHook returns the stored hook of the workspace with the id. An unknown id is ErrHookNotFound.
func (h *Handlers) findHook(ctx context.Context, workspaceID string, hookID string) (models.Hook, error) {
	hooks, err := h.uc.List.Execute(ctx, workspaceID)
	if err != nil {
		return models.Hook{}, fmt.Errorf("find hook: %w", err)
	}
	for _, hook := range hooks {
		if hook.ID() == hookID {
			return hook, nil
		}
	}
	return models.Hook{}, hookerrors.ErrHookNotFound
}

// render answers the tab. The fragment carries its own status, so the response is always 200.
func (h *Handlers) render(w http.ResponseWriter, r *http.Request, view state.View) {
	web.Render(w, r, http.StatusOK, hookcomponents.Panel(view))
}
