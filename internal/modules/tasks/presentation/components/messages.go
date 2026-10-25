package components

import (
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// User-facing texts of the board. Each one copies the Rust `Msg` constant it replaces
// (src/modules/tasks/presentation/**), so every language keeps the original wording. The French
// workspace-selection text of the Rust modal is corrupted there; the correct wording is used.
var (
	msgBoard           = i18n.Msg{En: "Board", Es: "Tablero", Zh: "看板", De: "Board", Fr: "Tableau"}
	msgNewTask         = i18n.Msg{En: "New task", Es: "Nueva tarea", Zh: "新建任务", De: "Neue Aufgabe", Fr: "Nouvelle tâche"}
	msgShowCompleted   = i18n.Msg{En: "Show completed", Es: "Mostrar terminadas", Zh: "显示已完成", De: "Erledigte anzeigen", Fr: "Afficher les terminées"}
	msgWorkspace       = i18n.Msg{En: "Workspace", Es: "Workspace", Zh: "工作区", De: "Workspace", Fr: "Espace de travail"}
	msgLoadMoreDone    = i18n.Msg{En: "Load more completed", Es: "Cargar más terminadas", Zh: "加载更多已完成", De: "Mehr erledigte laden", Fr: "Charger plus de terminées"}
	msgEmptyTitle      = i18n.Msg{En: "No tasks in this workspace yet", Es: "Aún no hay tareas en este workspace", Zh: "此工作区还没有任务", De: "In diesem Workspace gibt es noch keine Aufgaben", Fr: "Aucune tâche dans cet espace de travail pour l'instant"}
	msgEmptyHint       = i18n.Msg{En: "Create the first task, then chain more tasks to it: a task starts when the ones it depends on are merged.", Es: "Crea la primera tarea y encadena más: una tarea empieza cuando se integran aquellas de las que depende.", Zh: "创建第一个任务，然后串联更多任务：当所依赖的任务合并后，该任务才会开始。", De: "Erstelle die erste Aufgabe und verkette weitere: Eine Aufgabe startet, sobald die Aufgaben, von denen sie abhängt, integriert sind.", Fr: "Créez la première tâche, puis enchaînez-en d'autres : une tâche démarre quand celles dont elle dépend sont intégrées."}
	msgCreateFirst     = i18n.Msg{En: "Create the first task", Es: "Crear la primera tarea", Zh: "创建第一个任务", De: "Erste Aufgabe erstellen", Fr: "Créer la première tâche"}
	msgNoWorkspaces    = i18n.Msg{En: "Register a workspace to start planning tasks.", Es: "Registra un workspace para empezar a planificar tareas.", Zh: "注册一个工作区以开始规划任务。", De: "Registriere einen Workspace, um Aufgaben zu planen.", Fr: "Enregistrez un espace de travail pour planifier des tâches."}
	msgGoWorkspaces    = i18n.Msg{En: "Go to Workspaces", Es: "Ir a Workspaces", Zh: "前往工作区", De: "Zu den Workspaces", Fr: "Aller aux espaces de travail"}
	msgWorkspaceGone   = i18n.Msg{En: "This workspace no longer exists. Pick another one above.", Es: "Este workspace ya no existe. Elige otro arriba.", Zh: "此工作区已不存在。请在上方选择其他工作区。", De: "Dieser Workspace existiert nicht mehr. Wähle oben einen anderen.", Fr: "Cet espace de travail n'existe plus. Choisissez-en un autre ci-dessus."}
	msgSwitchWorkspace = i18n.Msg{En: "Switch workspace", Es: "Cambiar de workspace", Zh: "切换工作区", De: "Workspace wechseln", Fr: "Changer d'espace de travail"}
	msgSelectWorkspace = i18n.Msg{En: "Select a workspace…", Es: "Selecciona un workspace…", Zh: "选择一个工作区…", De: "Workspace auswählen…", Fr: "Sélectionnez un espace de travail…"}
	msgTitleOptional   = i18n.Msg{En: "Title (optional)", Es: "Título (opcional)", Zh: "标题（可选）", De: "Titel (optional)", Fr: "Titre (facultatif)"}
	msgPrompt          = i18n.Msg{En: "Prompt", Es: "Prompt", Zh: "提示词", De: "Prompt", Fr: "Prompt"}
	msgPromptHint      = i18n.Msg{En: "Describe what the agent should do…", Es: "Describe lo que debe hacer el agente…", Zh: "描述智能体应该做什么…", De: "Beschreibe, was der Agent tun soll …", Fr: "Décrivez ce que l’agent doit faire…"}
	msgPriority        = i18n.Msg{En: "Priority", Es: "Prioridad", Zh: "优先级", De: "Priorität", Fr: "Priorité"}
	msgStartNow        = i18n.Msg{En: "Start now", Es: "Iniciar ahora", Zh: "立即开始", De: "Jetzt starten", Fr: "Démarrer maintenant"}
	msgCancel          = i18n.Msg{En: "Cancel", Es: "Cancelar", Zh: "取消", De: "Abbrechen", Fr: "Annuler"}
	msgCreateTask      = i18n.Msg{En: "Create task", Es: "Crear tarea", Zh: "创建任务", De: "Aufgabe erstellen", Fr: "Créer la tâche"}
	msgStartFailed     = i18n.Msg{En: "Task created in To do, but it could not be started: {error}", Es: "Tarea creada en Por hacer, pero no se pudo iniciar: {error}", Zh: "任务已创建在“待办”中，但无法启动：{error}", De: "Aufgabe in „Zu erledigen“ erstellt, konnte aber nicht gestartet werden: {error}", Fr: "Tâche créée dans « À faire », mais impossible de la démarrer : {error}"}
)

// Texts of the sidebar lists, copied from the Rust sidebar (src/modules/tasks/presentation/components/app_sidebar.rs).
var (
	msgWorkspaces            = i18n.Msg{En: "Workspaces", Es: "Workspaces", Zh: "工作区", De: "Workspaces", Fr: "Espaces de travail"}
	msgNoWorkspacesYet       = i18n.Msg{En: "No workspaces yet.", Es: "Aún no hay workspaces.", Zh: "还没有工作区。", De: "Noch keine Workspaces vorhanden.", Fr: "Aucun espace de travail pour l’instant."}
	msgRegisterWorkspace     = i18n.Msg{En: "Register workspace", Es: "Registrar workspace", Zh: "注册工作区", De: "Workspace registrieren", Fr: "Enregistrer un espace de travail"}
	msgNeedsAttentionSection = i18n.Msg{En: "Needs attention", Es: "Requieren atención", Zh: "需要关注", De: "Erfordern Aufmerksamkeit", Fr: "Nécessitent votre attention"}
	msgNothingNeedsAttention = i18n.Msg{En: "Nothing needs attention.", Es: "Nada requiere atención.", Zh: "没有需要关注的内容。", De: "Nichts erfordert Aufmerksamkeit.", Fr: "Rien ne nécessite votre attention."}
	msgViewFiles             = i18n.Msg{En: "View files", Es: "Ver ficheros", Zh: "查看文件", De: "Dateien anzeigen", Fr: "Voir les fichiers"}
	msgOpenTerminal          = i18n.Msg{En: "Open terminal", Es: "Abrir terminal", Zh: "打开终端", De: "Terminal öffnen", Fr: "Ouvrir le terminal"}
	msgManageWorkspaces      = i18n.Msg{En: "Manage workspaces", Es: "Gestionar workspaces", Zh: "管理工作区", De: "Workspaces verwalten", Fr: "Gérer les espaces de travail"}
	msgOpenTask              = i18n.Msg{En: "Open task", Es: "Abrir tarea", Zh: "打开任务", De: "Aufgabe öffnen", Fr: "Ouvrir la tâche"}
)

// Priority labels, as the Rust priority DTO names them.
var (
	msgPriorityLow    = i18n.Msg{En: "Low", Es: "Baja", Zh: "低", De: "Niedrig", Fr: "Basse"}
	msgPriorityNormal = i18n.Msg{En: "Normal", Es: "Normal", Zh: "普通", De: "Normal", Fr: "Normale"}
	msgPriorityHigh   = i18n.Msg{En: "High", Es: "Alta", Zh: "高", De: "Hoch", Fr: "Haute"}
	msgPriorityUrgent = i18n.Msg{En: "Urgent", Es: "Urgente", Zh: "紧急", De: "Dringend", Fr: "Urgente"}
)

// Stage labels, as the Rust stage DTO names them.
var (
	msgStageBacklog   = i18n.Msg{En: "To do", Es: "Por hacer", Zh: "待办", De: "Offen", Fr: "À faire"}
	msgStageWorking   = i18n.Msg{En: "Working", Es: "Trabajando", Zh: "工作中", De: "Arbeitet", Fr: "En cours"}
	msgStageAttention = i18n.Msg{En: "Needs attention", Es: "Necesita atención", Zh: "需要关注", De: "Braucht Aufmerksamkeit", Fr: "Nécessite une attention"}
	msgStageReview    = i18n.Msg{En: "In review", Es: "En revisión", Zh: "待审阅", De: "In Prüfung", Fr: "En revue"}
	msgStageDone      = i18n.Msg{En: "Done", Es: "Hecho", Zh: "完成", De: "Erledigt", Fr: "Terminé"}
	// The fallback group of a task whose stage is not a board stage. It has no Rust counterpart.
	msgStageUnknown = i18n.Msg{En: "Unknown status", Es: "Estado desconocido", Zh: "未知状态", De: "Unbekannter Status", Fr: "Statut inconnu"}
)

// stageLabel is the text of a stage. An unknown stage reads as done, as the Rust mapping does.
func stageLabel(stage string) i18n.Msg {
	switch stage {
	case state.StageBacklog:
		return msgStageBacklog
	case state.StageWorking:
		return msgStageWorking
	case state.StageAttention:
		return msgStageAttention
	case state.StageReview:
		return msgStageReview
	case state.StageUnknown:
		return msgStageUnknown
	default:
		return msgStageDone
	}
}

// priorityLabel is the text of a priority. An unknown priority reads as normal.
func priorityLabel(priority string) i18n.Msg {
	switch priority {
	case "low":
		return msgPriorityLow
	case "high":
		return msgPriorityHigh
	case "urgent":
		return msgPriorityUrgent
	default:
		return msgPriorityNormal
	}
}
