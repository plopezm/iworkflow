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

	msgFilterAllProjects  = i18n.Msg{En: "All projects", Es: "Todos los proyectos", Zh: "所有项目", De: "Alle Projekte", Fr: "Tous les projets"}
	msgFilterProject      = i18n.Msg{En: "Filter by project", Es: "Filtrar por proyecto", Zh: "按项目过滤", De: "Nach Projekt filtern", Fr: "Filtrer par projet"}
	msgSearchPlaceholder  = i18n.Msg{En: "Filter tasks by keyword...", Es: "Filtrar tareas por palabra clave...", Zh: "按关键词过滤任务...", De: "Aufgaben nach Stichwort filtern...", Fr: "Filtrer les tâches par mot-clé..."}
	msgNoMatchingTasks    = i18n.Msg{En: "No tasks match the active filters.", Es: "No hay tareas que coincidan con los filtros.", Zh: "没有符合当前筛选条件的任务。", De: "Keine Aufgaben entsprechen den aktiven Filtern.", Fr: "Aucune tâche ne correspond aux filtres actifs."}
	msgEmptyRunning       = i18n.Msg{En: "No running tasks.", Es: "No hay tareas en ejecución.", Zh: "没有正在运行的任务。", De: "Keine laufenden Aufgaben.", Fr: "Aucune tâche en cours."}
	msgEmptyPending       = i18n.Msg{En: "No pending tasks.", Es: "No hay tareas pendientes.", Zh: "没有待处理的任务。", De: "Keine ausstehenden Aufgaben.", Fr: "Aucune tâche en attente."}
	msgRunningHint        = i18n.Msg{En: "Agents working or awaiting attention.", Es: "Agentes trabajando o esperando atención.", Zh: "智能体正在运行或等待处理。", De: "Agenten in Arbeit oder warten auf Feedback.", Fr: "Agents en cours ou en attente d'attention."}
	msgPendingHint        = i18n.Msg{En: "Ordered by urgency.", Es: "Ordenadas por urgencia.", Zh: "按紧急程度排序。", De: "Sortiert nach Dringlichkeit.", Fr: "Triées par urgence."}
	msgInGraph            = i18n.Msg{En: "In workflow", Es: "En workflow", Zh: "在工作流中", De: "Im Workflow", Fr: "Dans le workflow"}
	msgDragToWorkflowHint = i18n.Msg{En: "Drag to workflow canvas or click to add", Es: "Arrastra al grafo o pulsa para añadir", Zh: "拖动到工作流或点击添加", De: "Ins Workflow-Canvas ziehen oder zum Hinzufügen klicken", Fr: "Glissez vers le workflow ou cliquez pour ajouter"}
	msgEmptyGraphHint     = i18n.Msg{En: "The workflow graph is empty. Drag tasks here or tap '+ To graph' to chain dependencies across projects.", Es: "El grafo de workflows está vacío. Arrastra tareas aquí o pulsa '+ Al grafo' para encadenar dependencias entre proyectos.", Zh: "工作流图为空。将任务拖到此处或点击“+ 到工作流”以跨项目串联依赖项。", De: "Der Workflow-Graph ist leer. Ziehe Aufgaben hierher oder tippe auf '+ Zum Graph', um Abhängigkeiten über Projekte hinweg zu verketten.", Fr: "Le graphe est vide. Glissez des tâches ici ou appuyez sur '+ Au workflow' pour enchaîner des dépendances entre projets."}
	msgTasksInGraph       = i18n.Msg{En: "in graph", Es: "en grafo", Zh: "在图中", De: "im Graph", Fr: "dans le graphe"}
	msgDragHintGraph      = i18n.Msg{En: "Drag tasks to the canvas to chain them", Es: "Arrastra tareas al lienzo para conectarlas", Zh: "将任务拖到画布以进行连接", De: "Ziehe Aufgaben auf die Leinwand, um sie zu verketten", Fr: "Glissez des tâches sur le canevas pour les connecter"}
)

// itoa renders a count for the templates.
func itoa(value int) string {
	return strconv.Itoa(value)
}
