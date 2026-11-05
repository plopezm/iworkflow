package components

import (
	"strconv"

	"iworkflow/internal/core/i18n"
)

// User-facing texts of the global dashboard.
var (
	msgDashRunning       = i18n.Msg{En: "Running", Es: "En ejecución", Zh: "运行中", De: "Laufend", Fr: "En cours"}
	msgDashPending       = i18n.Msg{En: "Pending", Es: "Pendientes", Zh: "待处理", De: "Ausstehend", Fr: "En attente"}
	msgDashProjects      = i18n.Msg{En: "Projects", Es: "Proyectos", Zh: "项目", De: "Projekte", Fr: "Projets"}
	msgDashNothing       = i18n.Msg{En: "No pending or running tasks.", Es: "Sin tareas pendientes ni en ejecución.", Zh: "没有待处理或运行中的任务。", De: "Keine ausstehenden oder laufenden Aufgaben.", Fr: "Aucune tâche en attente ou en cours."}
	msgDashWorkflows     = i18n.Msg{En: "Workflows", Es: "Workflows", Zh: "工作流", De: "Workflows", Fr: "Workflows"}
	msgDashWorkflowsHint = i18n.Msg{En: "Chain tasks of different projects: a task starts when the ones it depends on are merged.", Es: "Encadena tareas de distintos proyectos: una tarea empieza cuando se integran aquellas de las que depende.", Zh: "串联不同项目的任务：当所依赖的任务合并后，该任务才会开始。", De: "Verkette Aufgaben verschiedener Projekte: Eine Aufgabe startet, sobald die Aufgaben, von denen sie abhängt, integriert sind.", Fr: "Enchaînez des tâches de différents projets : une tâche démarre quand celles dont elle dépend sont intégrées."}
)

// itoa renders a count for the templates.
func itoa(value int) string {
	return strconv.Itoa(value)
}
