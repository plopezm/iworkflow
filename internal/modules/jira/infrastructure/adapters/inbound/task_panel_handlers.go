//nolint:contextcheck,wrapcheck // templ components receive the request context at Render time; the linter cannot see through the closures. Domain and port errors are returned unwrapped: their text is shown to the user.
package inbound

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/components"
	"iworkflow/internal/modules/jira/presentation/state"
)

// msgCommentFailed tells that the worklog was registered but its optional comment was not posted.
var msgCommentFailed = i18n.Msg{
	En: "Logged, but the comment could not be posted: %s",
	Es: "Registrado, pero no se pudo publicar el comentario: %s",
	Zh: "已记录，但无法发布评论：%s",
	De: "Erfasst, aber der Kommentar konnte nicht veröffentlicht werden: %s",
	Fr: "Saisi, mais le commentaire n'a pas pu être publié : %s",
}

// msgChooseTransition asks for a status before a transition is sent to Jira.
var msgChooseTransition = i18n.Msg{
	En: "Choose a status.",
	Es: "Elige un estado.",
	Zh: "请选择一个状态。",
	De: "Wähle einen Status.",
	Fr: "Choisissez un statut.",
}

// TaskPanelUseCases are what the Jira panel of a task reads and changes.
type TaskPanelUseCases struct {
	GetTaskIssue    *application.GetTaskIssue
	LinkTask        *application.LinkTaskToIssue
	RefreshIssue    *application.RefreshIssue
	ListComments    *application.ListIssueComments
	ListTransitions *application.ListIssueTransitions
	TransitionIssue *application.TransitionIssue
	Proposal        *application.GetWorklogProposal
	ListWorklogs    *application.ListTaskWorklogs
	RegisterWorklog *application.RegisterWorklog
}

// TaskPanelHandlers serves the Jira panel of a task: the panel itself and its link, refresh, transition and worklog
// actions. Every answer is the whole panel, so the errors and the typed values stay inline.
type TaskPanelHandlers struct {
	uc TaskPanelUseCases
}

// NewTaskPanelHandlers builds the handlers over the use cases.
func NewTaskPanelHandlers(uc TaskPanelUseCases) *TaskPanelHandlers {
	return &TaskPanelHandlers{uc: uc}
}

// Mount registers the panel and its action routes. Each route answers with the whole panel of the task.
func (h *TaskPanelHandlers) Mount(r chi.Router) {
	r.Get("/jira/tasks/{id}/panel", h.panel)
	r.Post("/jira/tasks/{id}/link", h.link)
	r.Post("/jira/tasks/{id}/refresh", h.refresh)
	r.Post("/jira/tasks/{id}/transition", h.transition)
	r.Post("/jira/tasks/{id}/worklog", h.worklog)
}

// Body is the panel of a task as the tasks module embeds it. The data is read when the component renders, so a page
// that does not show the tab reads nothing from Jira.
func (h *TaskPanelHandlers) Body(taskID string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return components.JiraTaskPanel(h.load(ctx, taskID)).Render(ctx, w)
	})
}

// panel answers GET /jira/tasks/{id}/panel.
func (h *TaskPanelHandlers) panel(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	web.Render(w, r, http.StatusOK, components.JiraTaskPanel(h.load(r.Context(), taskID)))
}

// link answers POST /jira/tasks/{id}/link with the key the user typed.
func (h *TaskPanelHandlers) link(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID := chi.URLParam(r, "id")
	key := strings.TrimSpace(r.FormValue("key"))

	_, err := h.uc.LinkTask.Execute(ctx, taskID, key)
	view := h.load(ctx, taskID)
	if err != nil {
		view.ActionErr = err.Error()
		view.LinkKey = key
	}
	web.Render(w, r, http.StatusOK, components.JiraTaskPanel(view))
}

// refresh answers POST /jira/tasks/{id}/refresh: it reads the linked issue from Jira again.
func (h *TaskPanelHandlers) refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID := chi.URLParam(r, "id")

	key, err := h.linkedKey(ctx, taskID)
	if err == nil {
		_, err = h.uc.RefreshIssue.Execute(ctx, key)
	}
	view := h.load(ctx, taskID)
	if err != nil {
		view.ActionErr = err.Error()
	}
	web.Render(w, r, http.StatusOK, components.JiraTaskPanel(view))
}

// transition answers POST /jira/tasks/{id}/transition with the transition the user chose.
func (h *TaskPanelHandlers) transition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID := chi.URLParam(r, "id")
	transitionID := r.FormValue("transition_id")

	key, err := h.linkedKey(ctx, taskID)
	if err == nil && transitionID == "" {
		err = errors.New(msgChooseTransition.T(ctx))
	}
	if err == nil {
		_, err = h.uc.TransitionIssue.Execute(ctx, key, transitionID)
	}
	view := h.load(ctx, taskID)
	if err != nil {
		view.ActionErr = err.Error()
		view.SelectedTransition = transitionID
	}
	web.Render(w, r, http.StatusOK, components.JiraTaskPanel(view))
}

// worklog answers POST /jira/tasks/{id}/worklog. A refused form keeps what the user typed and changes nothing in Jira
// or in the local records.
func (h *TaskPanelHandlers) worklog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID := chi.URLParam(r, "id")
	timeText := r.FormValue("time")
	description := r.FormValue("description")
	alsoComment := formChecked(r.FormValue("also_comment"))

	result, err := h.uc.RegisterWorklog.Execute(ctx, application.RegisterWorklogInput{
		TaskID:      taskID,
		Time:        timeText,
		Description: description,
		AlsoComment: alsoComment,
	})
	view := h.load(ctx, taskID)
	switch {
	case err != nil:
		view.ActionErr = err.Error()
		view.TimeText = timeText
		view.Description = description
		view.AlsoComment = alsoComment
	case result.CommentError != "":
		view.ActionErr = fmt.Sprintf(msgCommentFailed.T(ctx), result.CommentError)
	}
	web.Render(w, r, http.StatusOK, components.JiraTaskPanel(view))
}

// load reads the whole panel of a task: its linked issue with the comments and transitions, and the worklog proposal
// with what was logged. A task with no issue shows only the field to link one.
func (h *TaskPanelHandlers) load(ctx context.Context, taskID string) state.TaskPanelView {
	view := state.TaskPanelView{TaskID: taskID}
	issue, err := h.uc.GetTaskIssue.Execute(ctx, taskID)
	if err != nil {
		view.ActionErr = err.Error()
		return view
	}
	if issue == nil {
		return view
	}
	issueDTO := dto.IssueFromDomain(*issue)
	view.Issue = &issueDTO
	h.loadIssueDetail(ctx, issue.Key(), &view)
	h.loadWorklogs(ctx, taskID, &view)
	return view
}

// loadIssueDetail reads the comments and the transitions of the issue. A failed read is shown in its own section.
func (h *TaskPanelHandlers) loadIssueDetail(ctx context.Context, key string, view *state.TaskPanelView) {
	comments, err := h.uc.ListComments.Execute(ctx, key)
	if err != nil {
		view.CommentsErr = err.Error()
	} else {
		view.Comments = dto.CommentsFromDomain(comments)
	}
	transitions, err := h.uc.ListTransitions.Execute(ctx, key)
	if err != nil {
		view.TransitionsErr = err.Error()
	} else {
		view.Transitions = dto.TransitionsFromDomain(transitions)
	}
}

// loadWorklogs fills the worklog form with the proposal and lists the worklogs already sent for the task.
func (h *TaskPanelHandlers) loadWorklogs(ctx context.Context, taskID string, view *state.TaskPanelView) {
	proposal, err := h.uc.Proposal.Execute(ctx, taskID)
	if err != nil {
		view.WorklogErr = err.Error()
	} else {
		view.Finished = proposal.Finished
		view.Pending = proposal.Time > 0
		if view.Pending {
			view.ProposedTime = models.FormatJiraDuration(proposal.Time)
		}
		view.TimeText = view.ProposedTime
		view.Description = proposal.Description
	}
	logged, err := h.uc.ListWorklogs.Execute(ctx, taskID)
	if err != nil {
		view.WorklogErr = err.Error()
		return
	}
	view.Worklogs = dto.WorklogsFromDomain(logged.Records)
	if logged.Total > 0 {
		view.Logged = models.FormatJiraDuration(logged.Total)
	}
}

// linkedKey is the key of the issue linked to the task; a task with no issue cannot take the action.
func (h *TaskPanelHandlers) linkedKey(ctx context.Context, taskID string) (string, error) {
	issue, err := h.uc.GetTaskIssue.Execute(ctx, taskID)
	if err != nil {
		return "", err
	}
	if issue == nil {
		return "", jiraerrors.TaskNotLinked()
	}
	return issue.Key(), nil
}

// formChecked reads a checkbox the way the rest of the module does: `on`, `true` or `1` are checked.
func formChecked(value string) bool {
	return value == "on" || value == "true" || value == "1"
}
