package dto

import (
	"net/http"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Wire values of the answers a dialog form posts: a value (select, input and editor), a confirmation, or a cancel.
const (
	dialogAnswerValue     = "value"
	dialogAnswerConfirmed = "confirmed"
	dialogAnswerCancelled = "cancelled"
)

// msgDialogAnswerInvalid is the refusal of an answer the dialog form does not describe.
var msgDialogAnswerInvalid = i18n.Msg{
	En: "The answer to the dialog is not valid.",
	Es: "La respuesta al diálogo no es válida.",
	Zh: "对话框的回答无效。",
	De: "Die Antwort auf den Dialog ist ungültig.",
	Fr: "La réponse à la boîte de dialogue n’est pas valide.",
}

// ParseDialogAnswerForm reads the answer a dialog form posts. The body is capped like the other forms. A confirmation
// needs a confirmed field of "true" or "false"; anything else is refused with a FormError that has its text in every
// language.
func ParseDialogAnswerForm(w http.ResponseWriter, r *http.Request) (models.DialogAnswer, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	values, _, err := readFormValues(r)
	if err != nil {
		return nil, err
	}
	switch first(values, "kind") {
	case dialogAnswerValue:
		// An empty input posts its field with an empty value; a missing field is not an answer.
		if _, sent := values["value"]; sent {
			return models.ValueAnswer{Value: first(values, "value")}, nil
		}
	case dialogAnswerCancelled:
		return models.CancelledAnswer{}, nil
	case dialogAnswerConfirmed:
		switch first(values, "confirmed") {
		case "true":
			return models.ConfirmedAnswer{Confirmed: true}, nil
		case "false":
			return models.ConfirmedAnswer{Confirmed: false}, nil
		}
	}
	return nil, formErr(msgDialogAnswerInvalid, nil)
}
