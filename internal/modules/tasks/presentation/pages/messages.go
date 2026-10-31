package pages

import "iworkflow/internal/core/i18n"

// Texts of the edit and detail pages, copied from the Rust task_edit_page.rs, task_detail_page.rs and task_modal.rs Msg constants.
var (
	msgCancel      = i18n.Msg{En: "Cancel", Es: "Cancelar", Zh: "取消", De: "Abbrechen", Fr: "Annuler"}
	msgSaveChanges = i18n.Msg{En: "Save changes", Es: "Guardar cambios", Zh: "保存更改", De: "Änderungen speichern", Fr: "Enregistrer les modifications"}
	msgCreateTask  = i18n.Msg{En: "Create task", Es: "Crear tarea", Zh: "创建任务", De: "Aufgabe erstellen", Fr: "Créer la tâche"}
	msgPanel       = i18n.Msg{En: "Panel", Es: "Panel", Zh: "面板", De: "Panel", Fr: "Panneau"}
	msgTaskPanel   = i18n.Msg{En: "Task panel", Es: "Panel de la tarea", Zh: "任务面板", De: "Aufgabenpanel", Fr: "Panneau de la tâche"}
	msgTask        = i18n.Msg{En: "Task", Es: "Tarea", Zh: "任务", De: "Aufgabe", Fr: "Tâche"}
	msgClose       = i18n.Msg{En: "Close", Es: "Cerrar", Zh: "关闭", De: "Schließen", Fr: "Fermer"}
)
