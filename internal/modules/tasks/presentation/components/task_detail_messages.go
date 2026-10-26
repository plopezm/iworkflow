package components

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// Texts of the task detail page, copied from the Rust task_detail_page.rs Msg constants.
var (
	msgTask                = i18n.Msg{En: "Task", Es: "Tarea", Zh: "任务", De: "Aufgabe", Fr: "Tâche"}
	msgTaskGone            = i18n.Msg{En: "This task no longer exists.", Es: "Esta tarea ya no existe.", Zh: "此任务已不存在。", De: "Diese Aufgabe existiert nicht mehr.", Fr: "Cette tâche n'existe plus."}
	msgBackToBoard         = i18n.Msg{En: "Back to the board", Es: "Volver al tablero", Zh: "返回看板", De: "Zurück zum Board", Fr: "Retour au tableau"}
	msgBranchPrefix        = i18n.Msg{En: "Branch ", Es: "Rama ", Zh: "分支 ", De: "Branch ", Fr: "Branche "}
	msgBranchFrom          = i18n.Msg{En: " from ", Es: " desde ", Zh: " 基于 ", De: " von ", Fr: " depuis "}
	msgResumesAt           = i18n.Msg{En: "Resumes automatically at", Es: "Se reanudará automáticamente a las", Zh: "将自动恢复于", De: "Wird automatisch fortgesetzt um", Fr: "Reprise automatique à"}
	msgDetailConfirmDelete = i18n.Msg{En: "Delete this task from the to-do list?", Es: "¿Borrar esta tarea de la lista de pendientes?", Zh: "要从待办列表中删除此任务吗？", De: "Diese Aufgabe aus der To-do-Liste löschen?", Fr: "Supprimer cette tâche de la liste des tâches à faire ?"}
)

// DetailTitle is the tab title of a task whose title is not known.
func DetailTitle(ctx context.Context) string {
	return msgTask.T(ctx)
}

// TaskGoneText is the message of a task that no longer exists, for a view that shows its failure as text.
func TaskGoneText(ctx context.Context) string {
	return msgTaskGone.T(ctx)
}
