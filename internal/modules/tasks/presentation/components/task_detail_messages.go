package components

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// Texts of the task detail page, copied from the Rust task_detail_page.rs Msg constants.
var (
	msgTask        = i18n.Msg{En: "Task", Es: "Tarea", Zh: "任务", De: "Aufgabe", Fr: "Tâche"}
	msgTaskGone    = i18n.Msg{En: "This task no longer exists.", Es: "Esta tarea ya no existe.", Zh: "此任务已不存在。", De: "Diese Aufgabe existiert nicht mehr.", Fr: "Cette tâche n'existe plus."}
	msgBackToBoard = i18n.Msg{En: "Back to the board", Es: "Volver al tablero", Zh: "返回看板", De: "Zurück zum Board", Fr: "Retour au tableau"}
)

// DetailTitle is the tab title of a task whose title is not known.
func DetailTitle(ctx context.Context) string {
	return msgTask.T(ctx)
}
