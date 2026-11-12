package components

import "iworkflow/internal/core/i18n"

// Labels of the Jira panel of a task. "Jira" itself is the product name and is not translated.
var (
	msgLinkHint = i18n.Msg{
		En: "Link this task to a Jira issue by its key.",
		Es: "Vincula esta tarea con un issue de Jira por su clave.",
		Zh: "通过 Jira 问题键将此任务与问题关联。",
		De: "Verknüpfe diese Aufgabe über ihren Schlüssel mit einem Jira-Issue.",
		Fr: "Liez cette tâche à un ticket Jira à l'aide de sa clé.",
	}
	msgIssueKey = i18n.Msg{
		En: "Issue key",
		Es: "Clave del issue",
		Zh: "问题键",
		De: "Issue-Schlüssel",
		Fr: "Clé du ticket",
	}
	msgLinkAction  = i18n.Msg{En: "Link", Es: "Vincular", Zh: "关联", De: "Verknüpfen", Fr: "Lier"}
	msgRefresh     = i18n.Msg{En: "Refresh", Es: "Refrescar", Zh: "刷新", De: "Aktualisieren", Fr: "Actualiser"}
	msgStatus      = i18n.Msg{En: "Status", Es: "Estado", Zh: "状态", De: "Status", Fr: "Statut"}
	msgPriority    = i18n.Msg{En: "Priority", Es: "Prioridad", Zh: "优先级", De: "Priorität", Fr: "Priorité"}
	msgAssignee    = i18n.Msg{En: "Assignee", Es: "Asignado", Zh: "经办人", De: "Bearbeiter", Fr: "Assigné"}
	msgDescription = i18n.Msg{
		En: "Description",
		Es: "Descripción",
		Zh: "描述",
		De: "Beschreibung",
		Fr: "Description",
	}
	msgNoDescription = i18n.Msg{
		En: "(no description)",
		Es: "(sin descripción)",
		Zh: "（无描述）",
		De: "(keine Beschreibung)",
		Fr: "(sans description)",
	}
	msgComments = i18n.Msg{
		En: "Latest comments",
		Es: "Últimos comentarios",
		Zh: "最新评论",
		De: "Neueste Kommentare",
		Fr: "Derniers commentaires",
	}
	msgNoComments     = i18n.Msg{En: "No comments.", Es: "Sin comentarios.", Zh: "没有评论。", De: "Keine Kommentare.", Fr: "Aucun commentaire."}
	msgTransition     = i18n.Msg{En: "Change status", Es: "Cambiar estado", Zh: "更改状态", De: "Status ändern", Fr: "Changer le statut"}
	msgPickTransition = i18n.Msg{
		En: "Choose…",
		Es: "Elegir…",
		Zh: "选择…",
		De: "Auswählen…",
		Fr: "Choisir…",
	}
	msgApplyTransition = i18n.Msg{En: "Apply", Es: "Aplicar", Zh: "应用", De: "Anwenden", Fr: "Appliquer"}
	msgNoTransitions   = i18n.Msg{
		En: "No status changes are available.",
		Es: "No hay cambios de estado disponibles.",
		Zh: "没有可用的状态更改。",
		De: "Keine Statusänderungen verfügbar.",
		Fr: "Aucun changement de statut disponible.",
	}
	msgWorklogTitle = i18n.Msg{
		En: "Log time",
		Es: "Registrar tiempo",
		Zh: "记录工时",
		De: "Zeit erfassen",
		Fr: "Saisir le temps",
	}
	msgBanner = i18n.Msg{
		En: "Task finished, log %s?",
		Es: "Task terminada, ¿registrar %s?",
		Zh: "任务已完成，记录 %s？",
		De: "Aufgabe beendet, %s erfassen?",
		Fr: "Tâche terminée, saisir %s ?",
	}
	msgTimeLabel          = i18n.Msg{En: "Time", Es: "Tiempo", Zh: "时间", De: "Zeit", Fr: "Temps"}
	msgWorklogDescription = i18n.Msg{
		En: "Description",
		Es: "Descripción",
		Zh: "描述",
		De: "Beschreibung",
		Fr: "Description",
	}
	msgAlsoComment = i18n.Msg{
		En: "Also comment on the issue",
		Es: "Comentar también en el issue",
		Zh: "同时在问题中评论",
		De: "Auch im Issue kommentieren",
		Fr: "Commenter aussi le ticket",
	}
	msgRegister = i18n.Msg{
		En: "Log in Jira",
		Es: "Registrar en Jira",
		Zh: "记录到 Jira",
		De: "In Jira erfassen",
		Fr: "Saisir dans Jira",
	}
	msgRegistered = i18n.Msg{
		En: "Logged: %s",
		Es: "Registrado: %s",
		Zh: "已记录：%s",
		De: "Erfasst: %s",
		Fr: "Saisi : %s",
	}
	msgNothingPending = i18n.Msg{
		En: "No time left to log.",
		Es: "No queda tiempo por registrar.",
		Zh: "没有待记录的时间。",
		De: "Keine Zeit mehr offen.",
		Fr: "Aucun temps restant à saisir.",
	}
	msgPreviousWorklogs = i18n.Msg{
		En: "Previous worklogs",
		Es: "Registros anteriores",
		Zh: "之前的工时记录",
		De: "Frühere Erfassungen",
		Fr: "Saisies précédentes",
	}
)
