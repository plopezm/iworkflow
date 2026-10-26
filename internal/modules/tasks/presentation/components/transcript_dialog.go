package components

import (
	"encoding/json"

	"github.com/a-h/templ"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Texts of the controls of a dialog, copied from the Rust dialog_panel.rs Msg constants.
var (
	msgDialogYes    = i18n.Msg{En: "Yes", Es: "Sí", Zh: "是", De: "Ja", Fr: "Oui"}
	msgDialogNo     = i18n.Msg{En: "No", Es: "No", Zh: "否", De: "Nein", Fr: "Non"}
	msgDialogCancel = i18n.Msg{En: "Cancel dialog", Es: "Cancelar diálogo", Zh: "取消对话框", De: "Dialog abbrechen", Fr: "Annuler la boîte de dialogue"}
)

// The kinds of answer the dialog form reads. They are the wire values of the answer handler (dto/dialog_form.go).
const (
	dialogKindValue     = "value"
	dialogKindConfirmed = "confirmed"
	dialogKindCancelled = "cancelled"
)

// dialogAnswerWire is an answer as a dialog control posts it. Only the field of its kind is set.
type dialogAnswerWire struct {
	Kind      string  `json:"kind"`
	Value     *string `json:"value,omitempty"`
	Confirmed *bool   `json:"confirmed,omitempty"`
}

// valueAnswer is the answer of a select option, of an input or of an editor.
func valueAnswer(value string) dialogAnswerWire {
	return dialogAnswerWire{Kind: dialogKindValue, Value: &value}
}

// confirmAnswer is the answer of a confirm dialog.
func confirmAnswer(confirmed bool) dialogAnswerWire {
	return dialogAnswerWire{Kind: dialogKindConfirmed, Confirmed: &confirmed}
}

// cancelAnswer is the answer that closes a dialog without answering it.
func cancelAnswer() dialogAnswerWire {
	return dialogAnswerWire{Kind: dialogKindCancelled}
}

// answerAttrs are the attributes of a control that posts an answer to the dialog. The answer replaces the dialog element,
// and the control drops the clicks that arrive while its answer is in flight, so one answer is sent.
func answerAttrs(taskID string, dialogID string, answer dialogAnswerWire) templ.Attributes {
	return templ.Attributes{
		"hx-post":   state.DialogAnswerPath(taskID, dialogID),
		"hx-vals":   dialogAnswerJSON(answer),
		"hx-target": "closest [data-dialog]",
		"hx-swap":   "outerHTML",
		"hx-sync":   "this:drop",
	}
}

// answerFormAttrs are the attributes of a form that posts its answer. The submit button is disabled while the answer is
// in flight, and the form drops the submits that arrive meanwhile.
func answerFormAttrs(taskID string, dialogID string) templ.Attributes {
	return templ.Attributes{
		"hx-post":         state.DialogAnswerPath(taskID, dialogID),
		"hx-target":       "closest [data-dialog]",
		"hx-swap":         "outerHTML",
		"hx-sync":         "this:drop",
		"hx-disabled-elt": "find button[type=submit]",
	}
}

// dialogAnswerJSON encodes an answer for hx-vals. The fields are plain values, so an encoding failure is not expected;
// it answers an empty object, which the server refuses.
func dialogAnswerJSON(answer dialogAnswerWire) string {
	encoded, err := json.Marshal(answer)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// valueOr is the text of an optional field of a dialog, empty when it is not set.
func valueOr(text *string) string {
	if text == nil {
		return ""
	}
	return *text
}
