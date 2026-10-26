//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/tasks/domain/models"
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

// ComposerUseCases are what the composer reads and changes.
type ComposerUseCases struct {
	Send    MessageSender
	Mode    ModeSetter
	History HistoryReader
}

// ComposerHandlers serves the message box of the detail page of a task.
type ComposerHandlers struct {
	uc    ComposerUseCases
	files FileSearch
}

// NewComposerHandlers builds the handlers over their use cases and the file search of the @ completion.
func NewComposerHandlers(uc ComposerUseCases, files FileSearch) *ComposerHandlers {
	return &ComposerHandlers{uc: uc, files: files}
}

// Mount registers the endpoints of the composer: the message, the mode switch and the file suggestions.
func (h *ComposerHandlers) Mount(r chi.Router) {
	r.Post("/tasks/{id}/messages", h.sendMessage)
	r.Post("/tasks/{id}/mode", h.setMode)
	r.Get("/tasks/{id}/composer/files", h.fileSuggestions)
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
	view := h.composerView(r.Context(), scope, id)
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
	view := h.composerView(r.Context(), scope, id)
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

// fileSuggestions answers the files the @ completion offers for a query, one item per path.
func (h *ComposerHandlers) fileSuggestions(w http.ResponseWriter, r *http.Request) {
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidTask.T(r.Context())))
		return
	}
	var paths []string
	if h.files != nil {
		found, searchErr := h.files(r.Context(), id, r.URL.Query().Get("q"))
		if searchErr != nil {
			web.Render(w, r, http.StatusOK, shared.ErrorText(formText(r.Context(), searchErr)))
			return
		}
		paths = found
	}
	web.Render(w, r, http.StatusOK, components.ComposerFileItems(paths))
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
// the mode unknown: the switch stays hidden and a message is an ordinary message.
func (h *ComposerHandlers) composerView(ctx context.Context, scope string, id models.TaskID) state.ComposerView {
	transcript, _, err := foldHistory(ctx, h.uc.History, id)
	if err != nil {
		return state.ComposerView{Scope: scope, TaskID: id.String()}
	}
	return state.NewComposerView(scope, id.String(), transcript)
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
