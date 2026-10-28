//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// TaskDiffRouteName is the dock route name of the diff tab of a task.
const TaskDiffRouteName = "task_diff"

// msgDiffUnavailable answers a diff that cannot be read. The read error is logged, not shown: its text is internal.
var msgDiffUnavailable = i18n.Msg{
	En: "The diff of the task could not be read.",
	Es: "No se pudo leer el diff de la tarea.",
	Zh: "无法读取任务的差异。",
	De: "Der Diff der Aufgabe konnte nicht gelesen werden.",
	Fr: "Le diff de la tâche n’a pas pu être lu.",
}

// msgDiffTabPlain names the diff tab when the task cannot be read, so the title does not show a failure.
var msgDiffTabPlain = i18n.Msg{En: "Diff", Es: "Diff", Zh: "差异", De: "Diff", Fr: "Diff"}

// msgInvalidReportAction answers a report action that is not one of the reports of the task.
var msgInvalidReportAction = i18n.Msg{
	En: "Unknown report action.",
	Es: "Acción de informe desconocida.",
	Zh: "未知的变更报告操作。",
	De: "Unbekannte Berichtsaktion.",
	Fr: "Action de rapport inconnue.",
}

// msgChangesReport names the diff report when the task it belongs to cannot be read, so the report does not show
// an empty title.
var msgChangesReport = i18n.Msg{En: "Changes report", Es: "Informe de cambios", Zh: "变更报告", De: "Änderungsbericht", Fr: "Rapport de modifications"}

// msgExplanationNotSent answers a report with explanations whose ask could not be delivered to the agent. The send
// error is logged, not shown: its text is internal.
var msgExplanationNotSent = i18n.Msg{
	En: "The agent could not be asked for the explanation. Close this and try again.",
	Es: "No se pudo pedir la explicación al agente. Cierra esto e inténtalo de nuevo.",
	Zh: "无法向智能体请求说明。请关闭后重试。",
	De: "Der Agent konnte nicht um die Erklärung gebeten werden. Schließe dies und versuche es erneut.",
	Fr: "L’agent n’a pas pu être sollicité pour l’explication. Fermez ceci et réessayez.",
}

// TaskGetter reads a task: the title of the tab and whether the task exists. The application use case satisfies it.
type TaskGetter interface {
	Execute(ctx context.Context, id models.TaskID) (models.Task, error)
}

// TaskDiffGetter reads the diff of a task against its base. *application.GetTaskDiff satisfies it.
type TaskDiffGetter interface {
	Execute(ctx context.Context, id models.TaskID) (models.TaskDiff, error)
}

// DiffUseCases are what the diff tab reads. Send asks the agent to explain the changes when the user asks for a
// report with explanations; it is the send the composer uses for a message. History is where the answer of the
// agent is read from.
type DiffUseCases struct {
	Get     TaskGetter
	Diff    TaskDiffGetter
	Send    MessageSender
	History HistoryReader
}

// DiffHandlers serves the diff tab of a task and the view that its layout controls swap.
type DiffHandlers struct {
	uc DiffUseCases
}

// NewDiffHandlers builds the handlers over the use cases.
func NewDiffHandlers(uc DiffUseCases) *DiffHandlers {
	return &DiffHandlers{uc: uc}
}

// Tabs returns the dock route of the diff tab. The module registers it with the other tabs of the tasks.
func (h *DiffHandlers) Tabs() []dock.Route {
	return []dock.Route{
		{
			Name:    TaskDiffRouteName,
			Pattern: "/tasks/{id}/diff",
			Icon:    "±",
			Title: func(ctx context.Context, params dock.Params) string {
				return h.title(ctx, params["id"])
			},
			Render: func(ctx context.Context, params dock.Params) templ.Component {
				return h.tab(ctx, params["id"])
			},
		},
	}
}

// Mount registers the view of the diff and the actions of its printable report. The layout controls answer the
// view with a layout in the query; the report actions answer the report that swaps the view.
func (h *DiffHandlers) Mount(r chi.Router) {
	r.Get("/tasks/{id}/diff/view", h.view)
	r.Get("/tasks/{id}/diff/report", h.report)
}

// title is the tab title: the title of the task, or the plain name when the task cannot be read.
func (h *DiffHandlers) title(ctx context.Context, rawID string) string {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return msgDiffTabPlain.T(ctx)
	}
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil || task.Title == "" {
		return msgDiffTabPlain.T(ctx)
	}
	return pages.DiffTabTitle(ctx, task.Title)
}

// tab renders the diff tab of a task in the default layout. A task that no longer exists says so.
func (h *DiffHandlers) tab(ctx context.Context, rawID string) templ.Component {
	id, err := models.ParseTaskID(rawID)
	if err != nil {
		return shared.ErrorText(msgInvalidTask.T(ctx))
	}
	if _, err := h.uc.Get.Execute(ctx, id); err != nil {
		return diffFailure(ctx, id, err)
	}
	view, err := h.load(ctx, id, newScope(), state.DefaultDiffLayout())
	if err != nil {
		return diffFailure(ctx, id, err)
	}
	return pages.TaskDiffPage(view)
}

// view answers the diff view in the layout of the query, for the instance named by its scope.
func (h *DiffHandlers) view(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidTask.T(ctx)))
		return
	}
	query := r.URL.Query()
	scope := state.SanitizeScope(query.Get("scope"))
	layout := state.ParseDiffLayout(query)
	view, err := h.load(ctx, id, scope, layout)
	if err != nil {
		// The region answers with its controls and the failure in them, so a failed read does not take the layout away.
		failed := state.DiffView{Scope: scope, TaskID: id.String(), Layout: layout, Failure: diffFailureText(ctx, id, err)}
		web.Render(w, r, http.StatusOK, components.DiffView(failed))
		return
	}
	web.Render(w, r, http.StatusOK, components.DiffView(view))
}

// report answers a printable report action of the task: the plain report renders the changes without
// explanations, the explained one asks the agent to explain them and waits for its answer, and the close stops the
// report. Each action answers the view of the task, so the overlay of the report swaps the region that asked.
func (h *DiffHandlers) report(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidTask.T(ctx)))
		return
	}
	switch action := r.URL.Query().Get("action"); action {
	case "plain":
		web.Render(w, r, http.StatusOK, h.plain(ctx, id, state.SanitizeScope(r.URL.Query().Get("scope"))))
	case "explained":
		web.Render(w, r, http.StatusOK, h.explained(ctx, id, state.SanitizeScope(r.URL.Query().Get("scope"))))
	case "poll":
		web.Render(w, r, http.StatusOK, h.polled(ctx, id, state.SanitizeScope(r.URL.Query().Get("scope")), r.URL.Query().Get("since")))
	case "close":
		web.Render(w, r, http.StatusOK, h.closed(ctx, id, state.SanitizeScope(r.URL.Query().Get("scope"))))
	default:
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidReportAction.T(ctx)))
	}
}

// plain renders the printable report without the explanation of the agent: the title, the commits, the summary and
// the patch grouped per file of the diff.
func (h *DiffHandlers) plain(ctx context.Context, id models.TaskID, scope string) templ.Component {
	view, err := h.load(ctx, id, scope, state.DefaultDiffLayout())
	if err != nil {
		return diffFailure(ctx, id, err)
	}
	view.Report = state.DiffReport{TaskID: id.String(), Request: state.ReportRequestPlain}
	return components.DiffView(view)
}

// explained asks the agent to explain the changes and answers with the report that waits for its answer: the
// report renders behind the progress dialog until the agent answers, or fails when the ask cannot be delivered.
func (h *DiffHandlers) explained(ctx context.Context, id models.TaskID, scope string) templ.Component {
	view, err := h.load(ctx, id, scope, state.DefaultDiffLayout())
	if err != nil {
		return diffFailure(ctx, id, err)
	}
	transcript, _, err := foldHistory(ctx, h.uc.History, id)
	if err != nil {
		return diffFailure(ctx, id, err)
	}
	report := state.DiffReport{TaskID: id.String()}
	report = report.RequestExplanation(transcript)
	report.Version++
	if err := h.uc.Send.Execute(ctx, id, state.ExplanationPrompt, nil, models.MessageModeFollowUp); err != nil {
		slog.Error("could not ask the agent for the explanation of the diff", "task", id.String(), "error", err)
		report = report.WithSendError(msgExplanationNotSent.T(ctx))
	}
	view.Report = report
	return components.DiffView(view)
}

// polled answers the report that waits for the agent: the transcript is read again, and the report renders once the
// agent answered the prompt after the length of the transcript at the time of the ask (since).
func (h *DiffHandlers) polled(ctx context.Context, id models.TaskID, scope string, rawSince string) templ.Component {
	view, err := h.load(ctx, id, scope, state.DefaultDiffLayout())
	if err != nil {
		return diffFailure(ctx, id, err)
	}
	transcript, _, err := foldHistory(ctx, h.uc.History, id)
	if err != nil {
		return diffFailure(ctx, id, err)
	}
	since, err := strconv.Atoi(rawSince)
	if err != nil || since < 0 {
		since = 0
	}
	view.Report = state.DiffReport{TaskID: id.String(), Request: state.ReportRequestExplained, Since: since, Transcript: transcript}
	return components.DiffView(view)
}

// closed answers the task with its diff again, without the report: the close of the report stops it, and the
// diff view that replaces it answers again with the summary and the file cards of the task diff.
func (h *DiffHandlers) closed(ctx context.Context, id models.TaskID, scope string) templ.Component {
	view, err := h.load(ctx, id, scope, state.DefaultDiffLayout())
	if err != nil {
		return diffFailure(ctx, id, err)
	}
	view.Report = view.Report.Close()
	return components.DiffView(view)
}

// load reads the task and its diff, and builds the view of them. The files can be edited while the task has its worktree.
func (h *DiffHandlers) load(ctx context.Context, id models.TaskID, scope string, layout state.DiffLayout) (state.DiffView, error) {
	task, err := h.uc.Get.Execute(ctx, id)
	if err != nil {
		return state.DiffView{}, fmt.Errorf("read the task of the diff: %w", err)
	}
	diff, err := h.uc.Diff.Execute(ctx, id)
	if err != nil {
		return state.DiffView{}, fmt.Errorf("read the diff of the task: %w", err)
	}
	view := state.NewDiffView(scope, id.String(), dto.TaskDiffFromDomain(diff), layout)
	view.CanEdit = task.Status.HasWorktree()
	view.Title = task.Title
	if view.Title == "" {
		view.Title = msgChangesReport.T(ctx)
	}
	return view, nil
}

// diffFailure answers why a diff cannot be shown in the full tab: a task that no longer exists says so, and any other
// failure is answered in the request language.
func diffFailure(ctx context.Context, id models.TaskID, err error) templ.Component {
	if errors.Is(err, werrors.NotFound("")) {
		return components.TaskGone()
	}
	return shared.ErrorText(diffFailureText(ctx, id, err))
}

// diffFailureText is the message of a diff that cannot be shown, in the request language. A read failure is logged
// with the task it belongs to; its text is internal, so it is not shown.
func diffFailureText(ctx context.Context, id models.TaskID, err error) string {
	if errors.Is(err, werrors.NotFound("")) {
		return components.TaskGoneText(ctx)
	}
	slog.Error("could not read the diff of the task", "task", id.String(), "error", err)
	return msgDiffUnavailable.T(ctx)
}
