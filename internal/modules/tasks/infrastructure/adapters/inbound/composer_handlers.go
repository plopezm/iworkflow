//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// msgInvalidTask answers a task identifier that is not a UUID. The handlers share it: the domain text is not shown
// here, so the answer is the same in every language.
var msgInvalidTask = i18n.Msg{
	En: "The task identifier is not valid.",
	Es: "El identificador de la tarea no es válido.",
	Zh: "任务标识无效。",
	De: "Die Kennung der Aufgabe ist ungültig.",
	Fr: "L’identifiant de la tâche n’est pas valide.",
}

// msgHistoryUnavailable answers a history that cannot be read. The read error is logged, not shown: its text is internal.
var msgHistoryUnavailable = i18n.Msg{
	En: "The history of the task could not be read.",
	Es: "No se pudo leer el historial de la tarea.",
	Zh: "无法读取任务的历史记录。",
	De: "Der Verlauf der Aufgabe konnte nicht gelesen werden.",
	Fr: "L’historique de la tâche n’a pas pu être lu.",
}

// MessageSender is the send use case as the composer uses it. *application.SendMessage satisfies it.
type MessageSender interface {
	Execute(ctx context.Context, id models.TaskID, message string, images []models.ImageAttachment, mode models.MessageMode) error
}

// ModeSetter is the mode switch as the composer uses it. *application.SetAgentMode satisfies it.
type ModeSetter interface {
	Execute(ctx context.Context, id models.TaskID, mode models.AgentMode) error
}

// HistoryReader reads the persisted events of a task. *application.GetTaskEvents satisfies it.
type HistoryReader interface {
	Execute(ctx context.Context, id models.TaskID, afterSeq *int64) ([]models.TaskEvent, error)
}

// FileSearch lists the project files of a task whose path matches query. The composition root supplies it; without
// one the @ completion finds nothing. The tasks module does not import the module that searches the files.
type FileSearch func(ctx context.Context, id models.TaskID, query string) ([]string, error)

// WorkspaceFileSearch lists the project files of a workspace whose path matches query.
type WorkspaceFileSearch func(ctx context.Context, workspaceID string, query string) ([]string, error)

// TaskReader loads a task. *application.GetTask satisfies it.
type TaskReader interface {
	Execute(ctx context.Context, id models.TaskID) (models.Task, error)
}

// StoredRelatedProjects lists the related projects of a task, in the order they were stored. *application.ListTaskRelatedProjects
// satisfies it.
type StoredRelatedProjects interface {
	Execute(ctx context.Context, id models.TaskID) ([]models.WorkspaceRef, error)
}

// relatedProjectLimit caps the related projects whose files the @ completion offers. The first ones in order win.
const relatedProjectLimit = 10

// ComposerUseCases are what the composer reads and changes. Answer is what the dialogs of the transcript answer to;
// without it the answer route is not mounted. Task and RelatedProjects are read by the @ completion: the workspace the
// task belongs to, which is left out of its related projects, and the related projects the task stores.
type ComposerUseCases struct {
	Send            MessageSender
	Mode            ModeSetter
	History         HistoryReader
	Answer          DialogAnswerer
	Task            TaskReader
	RelatedProjects StoredRelatedProjects
}

// ComposerHandlers serves the message box of the detail page of a task. answers serializes the answers to each dialog:
// the check that a dialog is still open and its delivery are one step, and only for that dialog.
type ComposerHandlers struct {
	uc             ComposerUseCases
	files          FileSearch
	workspaceFiles WorkspaceFileSearch
	workspaces     outbound.WorkspaceLookup
	answers        dialogLocks
}

// NewComposerHandlers builds the handlers over their use cases, the file searches of the @ completion and the lookup
// that resolves the related projects a form asks for.
func NewComposerHandlers(uc ComposerUseCases, files FileSearch, workspaceFiles WorkspaceFileSearch, workspaces outbound.WorkspaceLookup) *ComposerHandlers {
	return &ComposerHandlers{uc: uc, files: files, workspaceFiles: workspaceFiles, workspaces: workspaces}
}

// Mount registers the endpoints of the composer: the message, the mode switch, the file suggestions and the answer of
// a dialog of the transcript.
func (h *ComposerHandlers) Mount(r chi.Router) {
	r.Post("/tasks/{id}/messages", h.sendMessage)
	r.Post("/tasks/{id}/mode", h.setMode)
	r.Get("/tasks/{id}/composer/files", h.fileSuggestions)
	r.Get("/board/tasks/composer/files", h.workspaceFileSuggestions)
	if h.uc.Answer != nil {
		r.Post("/tasks/{id}/dialogs/{dialogId}/answer", h.answerDialog)
	}
}

// sendMessage reads the message of the composer and answers its body. A refused form or a refused use case is the
// answer, with its text: nothing is stored from a refused message.
func (h *ComposerHandlers) sendMessage(w http.ResponseWriter, r *http.Request) {
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidTask.T(r.Context())))
		return
	}
	view, historyErr := h.composerView(r.Context(), scope, id)
	if historyErr != nil {
		slog.Error("could not read the history of the composer", "error", historyErr)
		view.Notice = msgHistoryUnavailable.T(r.Context())
	}
	form, err := dto.ParseMessageForm(w, r)
	if err != nil {
		view.Notice = formText(r.Context(), err)
		h.renderBody(w, r, view)
		return
	}
	input, err := form.ToDomain()
	if err != nil {
		view.Notice = formText(r.Context(), err)
		view.Draft = form.Message
		h.renderBody(w, r, view)
		return
	}
	h.renderBody(w, r, h.deliver(r.Context(), id, input, view))
}

// deliver applies a message to the task and returns the view to answer with. A /plan or /agent message with no images
// switches the mode when the runtime reports one, as the Rust composer does; anything else goes to the send use case.
func (h *ComposerHandlers) deliver(ctx context.Context, id models.TaskID, input dto.SendMessageInput, view state.ComposerView) state.ComposerView {
	command, isCommand := state.ModeCommand(input.Message)
	if isCommand && len(input.Images) == 0 && view.HasMode() {
		if err := h.switchMode(ctx, id, command); err != nil {
			view.Notice = formText(ctx, err)
			view.Draft = input.Message
			return view
		}
		view.Mode = command
		return view
	}
	if err := h.uc.Send.Execute(ctx, id, input.Message, input.Images, input.Mode); err != nil {
		view.Notice = formText(ctx, err)
		view.Draft = input.Message
	}
	return view
}

// setMode switches the agent mode the mode switch names and answers the switch. A refused switch keeps the mode the
// runtime reports and says why in the switch, so the text typed in the composer stays where it is.
func (h *ComposerHandlers) setMode(w http.ResponseWriter, r *http.Request) {
	scope := state.SanitizeScope(r.URL.Query().Get("scope"))
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidTask.T(r.Context())))
		return
	}
	view, historyErr := h.composerView(r.Context(), scope, id)
	if historyErr != nil {
		slog.Error("could not read the history of the composer", "error", historyErr)
		view.ModeNotice = msgHistoryUnavailable.T(r.Context())
	}
	wire, err := dto.ParseModeForm(w, r)
	if err != nil {
		view.ModeNotice = formText(r.Context(), err)
	} else if err := h.switchMode(r.Context(), id, wire); err != nil {
		view.ModeNotice = formText(r.Context(), err)
	} else {
		view.Mode = wire
	}
	web.Render(w, r, http.StatusOK, components.ComposerModeContents(view))
}

// fileSuggestions answers the files the @ completion offers for a query: the files of the task's project, then those
// of its related projects. The related projects are the ones the form checks (related_form), or the stored list when
// the composer has no form to check (the detail composer).
func (h *ComposerHandlers) fileSuggestions(w http.ResponseWriter, r *http.Request) {
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidTask.T(r.Context())))
		return
	}
	query := r.URL.Query()
	var items []components.ComposerFileItem
	if h.files != nil {
		found, searchErr := h.files(r.Context(), id, query.Get("q"))
		if searchErr != nil {
			web.Render(w, r, http.StatusOK, shared.ErrorText(formText(r.Context(), searchErr)))
			return
		}
		items = currentFileItems(found)
	}
	related := h.taskRelatedProjects(r.Context(), id, query)
	items = append(items, h.relatedFileItems(r.Context(), related, query.Get("q"))...)
	web.Render(w, r, http.StatusOK, components.ComposerFileItems(items))
}

// workspaceFileSuggestions answers the files the @ completion offers for a workspace query: the files of the workspace
// of the form, then those of the related projects the form checks (related_form). Without related_form there are none.
func (h *ComposerHandlers) workspaceFileSuggestions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	workspaceID := query.Get("workspace_id")
	var items []components.ComposerFileItem
	if h.workspaceFiles != nil && workspaceID != "" {
		found, searchErr := h.workspaceFiles(r.Context(), workspaceID, query.Get("q"))
		if searchErr != nil {
			web.Render(w, r, http.StatusOK, shared.ErrorText(formText(r.Context(), searchErr)))
			return
		}
		items = currentFileItems(found)
	}
	var related []models.WorkspaceRef
	if query.Get("related_form") == "1" {
		related = h.requestedRelatedProjects(r.Context(), query["related"], workspaceID)
	}
	items = append(items, h.relatedFileItems(r.Context(), related, query.Get("q"))...)
	web.Render(w, r, http.StatusOK, components.ComposerFileItems(items))
}

// taskRelatedProjects are the related projects the @ completion of a task offers, without the task's own workspace.
// The form decides them when it has related checkboxes (related_form=1, an empty set means none); otherwise the stored
// list does. A task whose workspace cannot be read has no related projects: its own one could not be left out.
func (h *ComposerHandlers) taskRelatedProjects(ctx context.Context, id models.TaskID, query url.Values) []models.WorkspaceRef {
	own, err := h.taskWorkspace(ctx, id)
	if err != nil {
		slog.Warn("could not read the workspace of the task for the @ completion", "task_id", id.String(), "error", err)
		return nil
	}
	if query.Get("related_form") == "1" {
		return h.requestedRelatedProjects(ctx, query["related"], own)
	}
	return h.storedRelatedProjects(ctx, id, own)
}

// taskWorkspace is the workspace the task belongs to, or empty when the composer cannot read tasks.
func (h *ComposerHandlers) taskWorkspace(ctx context.Context, id models.TaskID) (string, error) {
	if h.uc.Task == nil {
		return "", nil
	}
	task, err := h.uc.Task.Execute(ctx, id)
	if err != nil {
		return "", fmt.Errorf("read the workspace of the task: %w", err)
	}
	return task.WorkspaceID, nil
}

// requestedRelatedProjects resolves the related ids a form sent, in order. Empty ids, repeats, the own workspace and the
// ids that do not name a workspace are skipped; a lookup that fails is logged and skipped too. It stops at the limit.
func (h *ComposerHandlers) requestedRelatedProjects(ctx context.Context, ids []string, own string) []models.WorkspaceRef {
	if h.workspaces == nil {
		return nil
	}
	seen := map[string]bool{own: true}
	var refs []models.WorkspaceRef
	for _, workspaceID := range ids {
		if len(refs) == relatedProjectLimit {
			break
		}
		if workspaceID == "" || seen[workspaceID] {
			continue
		}
		seen[workspaceID] = true
		found, err := h.workspaces.Find(ctx, workspaceID)
		if err != nil {
			slog.Warn("could not read a related project for the @ completion", "workspace_id", workspaceID, "error", err)
			continue
		}
		if found != nil {
			refs = append(refs, *found)
		}
	}
	return refs
}

// storedRelatedProjects is the stored list of related projects of a task, without its own workspace, repeats and the
// projects beyond the limit. A list that cannot be read offers no related projects.
func (h *ComposerHandlers) storedRelatedProjects(ctx context.Context, id models.TaskID, own string) []models.WorkspaceRef {
	if h.uc.RelatedProjects == nil {
		return nil
	}
	stored, err := h.uc.RelatedProjects.Execute(ctx, id)
	if err != nil {
		slog.Warn("could not read the related projects of the task for the @ completion", "task_id", id.String(), "error", err)
		return nil
	}
	seen := map[string]bool{own: true}
	var refs []models.WorkspaceRef
	for _, ref := range stored {
		if len(refs) == relatedProjectLimit {
			break
		}
		if seen[ref.ID] {
			continue
		}
		seen[ref.ID] = true
		refs = append(refs, ref)
	}
	return refs
}

// currentFileItems are the files of the current project, in the order the search gave them.
func currentFileItems(paths []string) []components.ComposerFileItem {
	items := make([]components.ComposerFileItem, 0, len(paths))
	for _, path := range paths {
		items = append(items, components.ComposerFileItem{Insert: path, Label: path})
	}
	return items
}

// relatedFileItems are the files of the related projects, project by project in the order of refs. A project whose
// search fails is logged and skipped, so the rest of the menu still answers.
func (h *ComposerHandlers) relatedFileItems(ctx context.Context, refs []models.WorkspaceRef, query string) []components.ComposerFileItem {
	if h.workspaceFiles == nil || len(refs) == 0 {
		return nil
	}
	labels := models.RelatedProjectLabels(refs)
	var items []components.ComposerFileItem
	for index, ref := range refs {
		found, err := h.workspaceFiles(ctx, ref.ID, query)
		if err != nil {
			slog.Warn("could not search the files of a related project for the @ completion", "workspace_id", ref.ID, "error", err)
			continue
		}
		for _, path := range found {
			items = append(items, components.ComposerFileItem{
				Insert:  labels[index] + ":" + path,
				Label:   path,
				Project: labels[index],
			})
		}
	}
	return items
}

// switchMode parses the wire name of a mode and asks the agent to switch to it.
func (h *ComposerHandlers) switchMode(ctx context.Context, id models.TaskID, wire string) error {
	mode, err := dto.ParseAgentModeForm(wire)
	if err != nil {
		return err //nolint:wrapcheck // the refusal text is user-visible and must stay identical
	}
	return h.uc.Mode.Execute(ctx, id, mode) //nolint:wrapcheck // the domain error text is user-visible and must stay identical
}

// composerView is the composer of the task as its transcript reports the mode. A history that cannot be read leaves
// the mode unknown, so the switch stays hidden and a message is an ordinary message; the read error is returned so the
// answer can say so.
func (h *ComposerHandlers) composerView(ctx context.Context, scope string, id models.TaskID) (state.ComposerView, error) {
	transcript, _, err := foldHistory(ctx, h.uc.History, id)
	if err != nil {
		return state.ComposerView{Scope: scope, TaskID: id.String()}, err
	}
	return state.NewComposerView(scope, id.String(), transcript), nil
}

// renderBody answers the body of the composer, which is what a send replaces.
func (h *ComposerHandlers) renderBody(w http.ResponseWriter, r *http.Request, view state.ComposerView) {
	web.Render(w, r, http.StatusOK, components.ComposerBody(view))
}

// formText is the text of an error in the language of the request. A form refusal has its text in every language;
// any other error shows its own text. A nil error has no text.
func formText(ctx context.Context, err error) string {
	if err == nil {
		return ""
	}
	var formErr *dto.FormError
	if errors.As(err, &formErr) {
		return formErr.Text(ctx)
	}
	return err.Error()
}
