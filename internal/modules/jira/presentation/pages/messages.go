package pages

import (
	"context"
	"strings"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/state"
)

//nolint:unused // Translations prepared for current and future Jira presentation views.
var (
	// Issue detail messages
	msgLoadingIssue = i18n.Msg{
		En: "Loading issue…",
		Es: "Cargando tarea…",
		Zh: "正在加载任务…",
		De: "Aufgabe wird geladen…",
		Fr: "Chargement de la tâche…",
	}

	msgNoDescription = i18n.Msg{
		En: "(no description)",
		Es: "(sin descripción)",
		Zh: "（无描述）",
		De: "(keine Beschreibung)",
		Fr: "(sans description)",
	}

	msgAssignee = i18n.Msg{
		En: "Assignee: ",
		Es: "Responsable: ",
		Zh: "负责人：",
		De: "Bearbeiter: ",
		Fr: "Responsable : ",
	}

	msgReporter = i18n.Msg{
		En: "Reporter: ",
		Es: "Informador: ",
		Zh: "报告人：",
		De: "Melder: ",
		Fr: "Rapporteur : ",
	}

	msgUpdatedInJira = i18n.Msg{
		En: "Updated in Jira: ",
		Es: "Actualizada en Jira: ",
		Zh: "Jira 中的更新时间：",
		De: "In Jira aktualisiert: ",
		Fr: "Mise à jour dans Jira : ",
	}

	msgLabels = i18n.Msg{
		En: "Labels: ",
		Es: "Etiquetas: ",
		Zh: "标签：",
		De: "Labels: ",
		Fr: "Étiquettes : ",
	}

	msgViewIworkflowTask = i18n.Msg{
		En: "View iworkflow task",
		Es: "Ver tarea de iworkflow",
		Zh: "查看 iworkflow 任务",
		De: "iworkflow-Aufgabe ansehen",
		Fr: "Voir la tâche iworkflow",
	}

	msgConvertToTask = i18n.Msg{
		En: "Convert to task",
		Es: "Convertir en tarea",
		Zh: "转换为任务",
		De: "In Aufgabe umwandeln",
		Fr: "Convertir en tâche",
	}

	msgIgnoreAction = i18n.Msg{
		En: "Ignore",
		Es: "Ignorar",
		Zh: "忽略",
		De: "Ignorieren",
		Fr: "Ignorer",
	}

	msgRestoreAction = i18n.Msg{
		En: "Restore",
		Es: "Restaurar",
		Zh: "恢复",
		De: "Wiederherstellen",
		Fr: "Restaurer",
	}

	msgMarkAsNewAction = i18n.Msg{
		En: "Mark as new",
		Es: "Marcar como nueva",
		Zh: "标记为新建",
		De: "Als neu markieren",
		Fr: "Marquer comme nouvelle",
	}

	msgOpenInJiraAction = i18n.Msg{
		En: "Open in Jira",
		Es: "Abrir en Jira",
		Zh: "在 Jira 中打开",
		De: "In Jira öffnen",
		Fr: "Ouvrir dans Jira",
	}

	msgDescriptionTitle = i18n.Msg{
		En: "Description",
		Es: "Descripción",
		Zh: "描述",
		De: "Beschreibung",
		Fr: "Description",
	}

	msgChangeStatusTitle = i18n.Msg{
		En: "Change status in Jira",
		Es: "Cambiar estado en Jira",
		Zh: "更改 Jira 中的状态",
		De: "Status in Jira ändern",
		Fr: "Changer le statut dans Jira",
	}

	msgCommentsTitle = i18n.Msg{
		En: "Comments",
		Es: "Comentarios",
		Zh: "评论",
		De: "Kommentare",
		Fr: "Commentaires",
	}

	msgCommentPlaceholder = i18n.Msg{
		En: "Write a comment for Jira…",
		Es: "Escribe un comentario para Jira…",
		Zh: "为 Jira 撰写评论…",
		De: "Schreibe einen Kommentar für Jira…",
		Fr: "Écrivez un commentaire pour Jira…",
	}

	msgCommentInJira = i18n.Msg{
		En: "Comment in Jira",
		Es: "Comentar en Jira",
		Zh: "在 Jira 中评论",
		De: "In Jira kommentieren",
		Fr: "Commenter dans Jira",
	}

	msgNoTransitions = i18n.Msg{
		En: "Jira does not allow changing the status right now.",
		Es: "Jira no permite cambiar el estado ahora.",
		Zh: "Jira 目前不允许更改状态。",
		De: "Jira erlaubt derzeit keine Statusänderung.",
		Fr: "Jira n'autorise pas de changer le statut pour le moment.",
	}

	msgTransitionLabel = i18n.Msg{
		En: "Transition",
		Es: "Transición",
		Zh: "状态转换",
		De: "Übergang",
		Fr: "Transition",
	}

	msgPickTransition = i18n.Msg{
		En: "Pick a transition…",
		Es: "Elige una transición…",
		Zh: "选择状态转换…",
		De: "Übergang auswählen…",
		Fr: "Choisissez une transition…",
	}

	msgApplyTransition = i18n.Msg{
		En: "Apply",
		Es: "Aplicar",
		Zh: "应用",
		De: "Anwenden",
		Fr: "Appliquer",
	}

	msgNoComments = i18n.Msg{
		En: "No comments.",
		Es: "Sin comentarios.",
		Zh: "暂无评论。",
		De: "Keine Kommentare.",
		Fr: "Aucun commentaire.",
	}

	msgIssueNotFound = i18n.Msg{
		En: "Issue not found",
		Es: "Tarea no encontrada",
		Zh: "未找到任务",
		De: "Aufgabe nicht gefunden",
		Fr: "Tâche introuvable",
	}

	// Conversion messages
	msgPreparingConversion = i18n.Msg{
		En: "Preparing conversion…",
		Es: "Preparando conversión…",
		Zh: "正在准备转换…",
		De: "Konvertierung wird vorbereitet…",
		Fr: "Préparation de la conversion…",
	}

	msgConvertHeading = i18n.Msg{
		En: "Convert {key} into an iworkflow task",
		Es: "Convertir {key} en tarea de iworkflow",
		Zh: "将 {key} 转换为 iworkflow 任务",
		De: "{key} in eine iworkflow-Aufgabe umwandeln",
		Fr: "Convertir {key} en tâche iworkflow",
	}

	msgReviewHint = i18n.Msg{
		En: "Review how the Jira issue is mapped and edit the prompt before creating the task.",
		Es: "Revisa cómo se mapea la tarea de Jira y edita el prompt antes de crear la tarea.",
		Zh: "在创建任务之前，请检查 Jira 任务的映射方式并编辑提示词。",
		De: "Prüfe, wie die Jira-Aufgabe abgebildet wird, und bearbeite den Prompt, bevor du die Aufgabe erstellst.",
		Fr: "Vérifiez comment la tâche Jira est mappée et modifiez le prompt avant de créer la tâche.",
	}

	msgViewJiraIssue = i18n.Msg{
		En: "View Jira issue",
		Es: "Ver tarea de Jira",
		Zh: "查看 Jira 任务",
		De: "Jira-Aufgabe ansehen",
		Fr: "Voir la tâche Jira",
	}

	msgWorkspaceLabel = i18n.Msg{
		En: "Workspace",
		Es: "Workspace",
		Zh: "工作区",
		De: "Workspace",
		Fr: "Espace de travail",
	}

	msgPickWorkspace = i18n.Msg{
		En: "Select a workspace…",
		Es: "Selecciona un workspace…",
		Zh: "选择一个工作区…",
		De: "Workspace auswählen…",
		Fr: "Sélectionnez un espace de travail…",
	}

	msgNoWorkspacePrefix = i18n.Msg{
		En: "Register a workspace first in ",
		Es: "Registra primero un workspace en ",
		Zh: "请先在",
		De: "Registriere zuerst einen Workspace unter ",
		Fr: "Enregistrez d'abord un espace de travail dans ",
	}

	msgNoWorkspaceLink = i18n.Msg{
		En: "Workspaces",
		Es: "Workspaces",
		Zh: "工作区",
		De: "Workspaces",
		Fr: "Espaces de travail",
	}

	msgNoWorkspaceSuffix = i18n.Msg{
		En: " to convert issues.",
		Es: " para poder convertir tareas.",
		Zh: "以便转换任务。",
		De: ", um Aufgaben konvertieren zu können.",
		Fr: " pour pouvoir convertir des tâches.",
	}

	msgBaseBranchLabel = i18n.Msg{
		En: "Base branch",
		Es: "Rama base",
		Zh: "基础分支",
		De: "Basis-Branch",
		Fr: "Branche de base",
	}

	msgDefaultBranchHint = i18n.Msg{
		En: "Empty = workspace default branch",
		Es: "Vacío = rama por defecto del workspace",
		Zh: "留空 = 工作区默认分支",
		De: "Leer = Standard-Branch des Workspaces",
		Fr: "Vide = branche par défaut de l'espace de travail",
	}

	msgTitleLabel = i18n.Msg{
		En: "Title",
		Es: "Título",
		Zh: "标题",
		De: "Titel",
		Fr: "Titre",
	}

	msgPromptLabel = i18n.Msg{
		En: "Prompt",
		Es: "Prompt",
		Zh: "提示词",
		De: "Prompt",
		Fr: "Prompt",
	}

	msgPriorityLabel = i18n.Msg{
		En: "Priority",
		Es: "Prioridad",
		Zh: "优先级",
		De: "Priorität",
		Fr: "Priorité",
	}

	msgPriorityLow = i18n.Msg{
		En: "Low",
		Es: "Baja",
		Zh: "低",
		De: "Niedrig",
		Fr: "Basse",
	}

	msgPriorityNormal = i18n.Msg{
		En: "Normal",
		Es: "Normal",
		Zh: "普通",
		De: "Normal",
		Fr: "Normale",
	}

	msgPriorityHigh = i18n.Msg{
		En: "High",
		Es: "Alta",
		Zh: "高",
		De: "Hoch",
		Fr: "Haute",
	}

	msgPriorityUrgent = i18n.Msg{
		En: "Urgent",
		Es: "Urgente",
		Zh: "紧急",
		De: "Dringend",
		Fr: "Urgente",
	}

	msgStartNowLabel = i18n.Msg{
		En: "Start task immediately with the default agent",
		Es: "Arrancar la tarea inmediatamente con el agente por defecto",
		Zh: "立即使用默认智能体启动任务",
		De: "Aufgabe sofort mit dem Standard-Agenten starten",
		Fr: "Démarrer la tâche immédiatement avec l'agent par défaut",
	}

	msgCreateTaskButton = i18n.Msg{
		En: "Create task",
		Es: "Crear tarea",
		Zh: "创建任务",
		De: "Aufgabe erstellen",
		Fr: "Créer la tâche",
	}

	msgTaskCreated = i18n.Msg{
		En: "Task created",
		Es: "Tarea creada",
		Zh: "任务已创建",
		De: "Aufgabe erstellt",
		Fr: "Tâche créée",
	}

	msgStartFailed = i18n.Msg{
		En: "The task was created, but the agent could not start automatically: {error}",
		Es: "La tarea se creó, pero no se pudo arrancar el agente automáticamente: {error}",
		Zh: "任务已创建，但智能体无法自动启动：{error}",
		De: "Die Aufgabe wurde erstellt, aber der Agent konnte nicht automatisch gestartet werden: {error}",
		Fr: "La tâche a été créée, mais l'agent n'a pas pu démarrer automatiquement : {error}",
	}

	msgLinkFailed = i18n.Msg{
		En: "The task was created, but it could not be linked back to Jira: {error}",
		Es: "La tarea se creó, pero no se pudo vincular de vuelta en Jira: {error}",
		Zh: "任务已创建，但无法关联回 Jira：{error}",
		De: "Die Aufgabe wurde erstellt, konnte aber nicht mit Jira verknüpft werden: {error}",
		Fr: "La tâche a été créée, mais elle n'a pas pu être liée à Jira : {error}",
	}

	msgViewTaskButton = i18n.Msg{
		En: "View task",
		Es: "Ver tarea",
		Zh: "查看任务",
		De: "Aufgabe ansehen",
		Fr: "Voir la tâche",
	}

	msgConvertTabTitle = i18n.Msg{
		En: "Convert {key}",
		Es: "Convertir {key}",
		Zh: "转换 {key}",
		De: "{key} umwandeln",
		Fr: "Convertir {key}",
	}
)

func FormatConvertHeading(ctx context.Context, key string) string {
	return strings.ReplaceAll(msgConvertHeading.T(ctx), "{key}", key)
}

func FormatConvertTitle(ctx context.Context, key string) string {
	return strings.ReplaceAll(msgConvertTabTitle.T(ctx), "{key}", key)
}

func formatBranchOption(ctx context.Context, branch dto.ConversionBranchDTO) string {
	return state.FormatBranch(branch, i18n.StateFrom(ctx).Locale)
}
