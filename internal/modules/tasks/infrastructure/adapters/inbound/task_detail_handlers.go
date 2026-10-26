//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// TaskDetailRouteName is the dock route name of the detail page of a task.
const TaskDetailRouteName = "task_detail"

// DetailUseCases are what the detail page reads and changes.
type DetailUseCases struct {
	Get             *application.GetTask
	Events          *application.GetTaskEvents
	ListAttachments *application.ListTaskAttachments
	Start           *application.StartTask
	Abort           *application.AbortTask
	Resume          *application.ResumeTask
	Finish          *application.FinishTask
	Reset           *application.ResetTask
	Discard         *application.DiscardTask
	Delete          *application.DeleteTask
}

// DetailHandlers serves the detail tab of a task and its actions.
type DetailHandlers struct {
	uc  DetailUseCases
	now func() time.Time
}

// NewDetailHandlers builds the handlers over the use cases. now is the clock the stage of the task is read at.
func NewDetailHandlers(uc DetailUseCases, now func() time.Time) *DetailHandlers {
	return &DetailHandlers{uc: uc, now: now}
}

// Tabs returns the dock route of the detail page. The module registers it before the not-migrated catch-all of
// /tasks/{rest...}, so only this address leaves the Rust version; diff, terminal and files still fall through.
func (h *DetailHandlers) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    TaskDetailRouteName,
			Pattern: "/tasks/{id}",
			Icon:    "●",
			Title: func(ctx context.Context, params dock.Params) string {
				return h.title(ctx, params["id"])
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.detailTab(ctx, params["id"])
			},
		},
	}
}

// Mount registers the htmx endpoints of the detail page. Start, abort, resume, finish and discard answer the page;
// reset and delete end it, so they answer the tab-closing element (see leaveAction). The transcript answers its body
// again each time the task stream reports an event.
func (h *DetailHandlers) Mount(r chi.Router) {
	r.Post("/tasks/{id}/start", h.startTask)
	r.Post("/tasks/{id}/abort", h.abortTask)
	r.Post("/tasks/{id}/resume", h.resumeTask)
	r.Post("/tasks/{id}/finish", h.finishTask)
	r.Post("/tasks/{id}/discard", h.discardTask)
	r.Post("/tasks/{id}/reset", h.resetTask)
	r.Post("/tasks/{id}/delete", h.deleteTask)
	r.Get("/tasks/{id}/transcript", h.transcript)
}

// title is the tab title: the title of the task, or the plain name when the task cannot be read.
func (h *DetailHandlers) title(ctx context.Context, rawID string) string {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return components.DetailTitle(ctx)
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil || task.Title == "" {
		return components.DetailTitle(ctx)
	}
	return task.Title
}

// detailTab renders the detail page of a task. An address that is not a task identifier shows the error; a task
// that no longer exists shows that it is gone, with a link back to the board.
func (h *DetailHandlers) detailTab(ctx context.Context, rawID string) templ.Component {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if errors.Is(err, werrors.NotFound("")) {
		return components.TaskGone()
	}
	if err != nil {
		return shared.ErrorText(err.Error())
	}
	return pages.TaskDetailPage(h.page(ctx, newScope(), task, ""))
}

// page builds the view of a task: the task as it is now, its transcript, its images and the notice of the action that
// asked for it. A failed read of the transcript or of the images is shown in their place, and the rest of the page
// still renders.
func (h *DetailHandlers) page(ctx context.Context, scope string, task models.Task, notice string) state.TaskDetailView {
	view := state.TaskDetailView{Scope: scope, Task: dto.TaskFromDomain(task, h.now()), Notice: notice, CanMessage: task.Status.AcceptsMessages()}
	transcript, cursor, transcriptErr := h.loadTranscript(ctx, task.ID)
	if transcriptErr != nil {
		view.TranscriptError = transcriptErr.Error()
	} else {
		view.Transcript = transcript
		view.TranscriptCursor = cursor
	}
	stored, err := h.uc.ListAttachments.Execute(ctx, task.ID)
	if err != nil {
		view.AttachmentsError = err.Error()
		return view
	}
	view.Attachments = make([]dto.TaskAttachmentDTO, 0, len(stored))
	for _, attachment := range stored {
		view.Attachments = append(view.Attachments, dto.TaskAttachmentFromDomain(task.ID.String(), attachment))
	}
	return view
}

// loadTranscript reads the history of a task and folds it into the transcript the page shows.
func (h *DetailHandlers) loadTranscript(ctx context.Context, id models.TaskID) (state.Transcript, *int64, error) {
	return foldHistory(ctx, h.uc.Events, id)
}

// foldHistory reads the history of a task and folds it into a transcript. The cursor is the last sequence number of
// the history, where the live stream resumes.
func foldHistory(ctx context.Context, history HistoryReader, id models.TaskID) (state.Transcript, *int64, error) {
	events, err := history.Execute(ctx, id, nil)
	if err != nil {
		return state.Transcript{}, nil, fmt.Errorf("read the history of the task: %w", err)
	}
	mapped, err := dto.TranscriptEventsFromDomain(events)
	if err != nil {
		return state.Transcript{}, nil, fmt.Errorf("map the history of the task: %w", err)
	}
	transcript, cursor := state.FoldTranscript(mapped)
	return transcript, cursor, nil
}

// transcript answers the body of the transcript of a task. A failed read is shown in its place.
func (h *DetailHandlers) transcript(w http.ResponseWriter, r *http.Request) {
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	transcript, _, err := h.loadTranscript(r.Context(), id)
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	web.Render(w, r, http.StatusOK, components.TranscriptBody(id.String(), transcript))
}

// startTask starts a backlog task from its detail.
func (h *DetailHandlers) startTask(w http.ResponseWriter, r *http.Request) {
	h.answerAction(w, r, func(ctx context.Context, id models.TaskID) error {
		_, err := h.uc.Start.Start(ctx, id)
		return err //nolint:wrapcheck // the domain error is user-visible and must stay identical
	})
}

// abortTask aborts the live agent of the task.
func (h *DetailHandlers) abortTask(w http.ResponseWriter, r *http.Request) {
	h.answerAction(w, r, h.uc.Abort.Execute)
}

// resumeTask resumes a stopped task.
func (h *DetailHandlers) resumeTask(w http.ResponseWriter, r *http.Request) {
	h.answerAction(w, r, h.uc.Resume.Execute)
}

// finishTask closes the agent of the task and consolidates its changes for review.
func (h *DetailHandlers) finishTask(w http.ResponseWriter, r *http.Request) {
	h.answerAction(w, r, h.uc.Finish.Execute)
}

// discardTask discards the task: its worktree and its branch go, and the task stays on the page as discarded.
func (h *DetailHandlers) discardTask(w http.ResponseWriter, r *http.Request) {
	h.answerAction(w, r, h.uc.Discard.Execute)
}

// resetTask returns the task to backlog. The tab closes and opens the board, as the Rust version does.
func (h *DetailHandlers) resetTask(w http.ResponseWriter, r *http.Request) {
	h.leaveAction(w, r, h.uc.Reset.Execute)
}

// deleteTask deletes a backlog task from its detail. The tab closes and opens the board of the task.
func (h *DetailHandlers) deleteTask(w http.ResponseWriter, r *http.Request) {
	h.leaveAction(w, r, h.uc.Delete.Execute)
}

// answerAction runs an action of the task and answers the page of this instance with the notice of the action. The
// board refreshes either way, because an action may have moved the task across the board.
func (h *DetailHandlers) answerAction(w http.ResponseWriter, r *http.Request, run func(context.Context, models.TaskID) error) {
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	runErr := run(r.Context(), id)
	web.Trigger(w, boardRefreshEvent)
	h.renderPage(w, r, scope, id, errorText(runErr))
}

// leaveAction runs an action that ends the page of the task. A success closes the tab and opens the board of the
// task; a refused action keeps the tab and shows why in the result slot, where its button answers.
func (h *DetailHandlers) leaveAction(w http.ResponseWriter, r *http.Request, run func(context.Context, models.TaskID) error) {
	ctx := r.Context()
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	if runErr := run(ctx, id); runErr != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(runErr.Error()))
		return
	}
	web.Trigger(w, boardRefreshEvent)
	web.Render(w, r, http.StatusOK, components.TaskSaved(state.BoardPath(task.WorkspaceID)))
}

// renderPage answers with the page of the task as it is now, carrying the notice of the action that asked for it.
func (h *DetailHandlers) renderPage(w http.ResponseWriter, r *http.Request, scope string, id models.TaskID, notice string) {
	task, err := h.uc.Get.Execute(r.Context(), id)
	if errors.Is(err, werrors.NotFound("")) {
		web.Render(w, r, http.StatusOK, components.TaskGone())
		return
	}
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(err.Error()))
		return
	}
	web.Render(w, r, http.StatusOK, pages.TaskDetailPage(h.page(r.Context(), scope, task, notice)))
}
