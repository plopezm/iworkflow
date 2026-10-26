//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// msgDialogDelivery answers an answer the agent did not take. The runtime reports its text in Spanish and that text is
// internal, so the answer is the same in every language and the error is logged instead.
var msgDialogDelivery = i18n.Msg{
	En: "The answer could not be delivered to the agent.",
	Es: "No se pudo enviar la respuesta al agente.",
	Zh: "无法把回答发送给智能体。",
	De: "Die Antwort konnte nicht an den Agenten gesendet werden.",
	Fr: "La réponse n’a pas pu être envoyée à l’agent.",
}

// msgDialogAnswerMismatch refuses an answer that is not of the kind the dialog asks, like a confirmation to a select.
var msgDialogAnswerMismatch = i18n.Msg{
	En: "The answer does not match the dialog.",
	Es: "La respuesta no corresponde al diálogo.",
	Zh: "回答与对话框不匹配。",
	De: "Die Antwort passt nicht zu dem Dialog.",
	Fr: "La réponse ne correspond pas à la boîte de dialogue.",
}

// DialogAnswerer answers a dialog the agent opened. *application.AnswerDialog satisfies it.
type DialogAnswerer interface {
	Execute(ctx context.Context, id models.TaskID, dialogID string, answer models.DialogAnswer) error
}

// answerDialog takes the answer of a dialog control and answers the dialog in its place. The dialog is looked up in the
// transcript first: one the transcript does not hold, or one already answered, is never delivered to the agent. A dialog
// whose turn is closed, or whose agent is gone, is answered as closed, without an error.
func (h *ComposerHandlers) answerDialog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := models.ParseTaskID(chi.URLParam(r, "id"))
	if err != nil {
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgInvalidTask.T(ctx)))
		return
	}
	dialogID := chi.URLParam(r, "dialogId")
	// The history is read under the dialog's lock: an answer that wins the lock records its resolution before the next one
	// reads. Answers to other dialogs do not wait for it.
	release := h.answers.lock(dialogKey{task: id.String(), dialog: dialogID})
	defer release()
	transcript, _, err := foldHistory(ctx, h.uc.History, id)
	if err != nil {
		slog.Error("could not read the history of the dialog", "error", err)
		web.Render(w, r, http.StatusOK, shared.ErrorText(msgHistoryUnavailable.T(ctx)))
		return
	}
	item, found := transcript.Dialog(dialogID)
	if !found {
		web.Render(w, r, http.StatusOK, templ.NopComponent)
		return
	}
	if item.Resolved {
		web.Render(w, r, http.StatusOK, components.TranscriptDialog(id.String(), item, ""))
		return
	}
	item, notice := h.takeAnswer(w, r, id, &transcript, dialogID, item)
	web.Render(w, r, http.StatusOK, components.TranscriptDialog(id.String(), item, notice))
}

// answerFits reports whether the dialog takes the answer: a value for a select, input or editor, a confirmation for a
// confirm dialog. A cancel closes any dialog.
func answerFits(kind dto.TranscriptDialogKind, answer models.DialogAnswer) bool {
	switch answer.(type) {
	case models.CancelledAnswer:
		return true
	case models.ValueAnswer:
		switch kind.(type) {
		case dto.TranscriptSelectDialog, dto.TranscriptInputDialog, dto.TranscriptEditorDialog:
			return true
		}
		return false
	case models.ConfirmedAnswer:
		_, isConfirm := kind.(dto.TranscriptConfirmDialog)
		return isConfirm
	default:
		return false
	}
}

// takeAnswer reads the answer of the form and hands it to the agent. It returns the dialog as it stands after the answer,
// and the notice to show with it: a refused answer or one the agent did not take keeps the dialog open.
func (h *ComposerHandlers) takeAnswer(w http.ResponseWriter, r *http.Request, id models.TaskID, transcript *state.Transcript, dialogID string, item state.DialogItem) (state.DialogItem, string) {
	ctx := r.Context()
	answer, err := dto.ParseDialogAnswerForm(w, r)
	if err != nil {
		return item, formText(ctx, err)
	}
	if !answerFits(item.Request.Kind, answer) {
		return item, msgDialogAnswerMismatch.T(ctx)
	}
	err = h.uc.Answer.Execute(ctx, id, dialogID, answer)
	switch {
	case err == nil:
		item.Resolved = true
		return item, ""
	case closedTurn(err):
		// The agent is gone, so its turn ended: the transcript closes the dialog with the turn, as it does for the history.
		transcript.CloseTurn()
		closed, _ := transcript.Dialog(dialogID)
		return closed, ""
	case errors.Is(err, werrors.AnswerNotRecorded()):
		// The agent has the answer; only its record failed. The dialog is answered and the failure is logged.
		slog.Error("could not record the answer of the dialog", "task_id", id.String(), "dialog_id", dialogID, "error", err)
		item.Resolved = true
		return item, ""
	default:
		slog.Error("could not deliver the answer of the dialog", "task_id", id.String(), "dialog_id", dialogID, "error", err)
		return item, msgDialogDelivery.T(ctx)
	}
}

// closedTurn reports whether the answer found no live agent. Then the turn is closed and the dialog closes with it.
func closedTurn(err error) bool {
	return errors.Is(err, werrors.AgentNotRunning()) || errors.Is(err, werrors.InvalidState("", ""))
}
