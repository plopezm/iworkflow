package components

import "iworkflow/internal/core/i18n"

// Texts of the subagents panel, copied from the Rust subagents_panel.rs and task_actions.rs Msg constants.
var (
	msgSubagents        = i18n.Msg{En: "Subagents", Es: "Subagentes", Zh: "子智能体", De: "Subagents", Fr: "Sous-agents"}
	msgSubagentsEmpty   = i18n.Msg{En: "This agent has not launched any subagents.", Es: "Este agente no ha lanzado subagentes.", Zh: "此智能体尚未启动任何子智能体。", De: "Dieser Agent hat noch keine Subagents gestartet.", Fr: "Cet agent n’a lancé aucun sous-agent."}
	msgStatusQueued     = i18n.Msg{En: "Queued", Es: "En cola", Zh: "排队中", De: "In Warteschlange", Fr: "En file d’attente"}
	msgStatusRunning    = i18n.Msg{En: "Running", Es: "Ejecutando", Zh: "运行中", De: "Läuft", Fr: "En cours d’exécution"}
	msgStatusWaiting    = i18n.Msg{En: "Waiting", Es: "Esperando", Zh: "等待中", De: "Wartet", Fr: "En attente"}
	msgStatusCompleted  = i18n.Msg{En: "Completed", Es: "Completado", Zh: "已完成", De: "Abgeschlossen", Fr: "Terminé"}
	msgStatusFailed     = i18n.Msg{En: "Failed", Es: "Fallido", Zh: "失败", De: "Fehlgeschlagen", Fr: "Échoué"}
	msgStatusCancelled  = i18n.Msg{En: "Cancelled", Es: "Cancelado", Zh: "已取消", De: "Abgebrochen", Fr: "Annulé"}
	msgStatusTimedOut   = i18n.Msg{En: "Timed out", Es: "Tiempo agotado", Zh: "已超时", De: "Zeitüberschreitung", Fr: "Délai dépassé"}
	msgStatusUnknown    = i18n.Msg{En: "Unknown", Es: "Desconocido", Zh: "未知", De: "Unbekannt", Fr: "Inconnu"}
	msgActivitySummary  = i18n.Msg{En: "{turns} turns · {tools} tools", Es: "{turns} turnos · {tools} herramientas", Zh: "{turns} 轮 · {tools} 个工具", De: "{turns} Durchgänge · {tools} Werkzeuge", Fr: "{turns} tours · {tools} outils"}
	msgBadgeRunning     = i18n.Msg{En: "running", Es: "ejecutando", Zh: "运行中", De: "läuft", Fr: "en cours"}
	msgBadgeError       = i18n.Msg{En: "error", Es: "error", Zh: "错误", De: "Fehler", Fr: "erreur"}
	msgSubagentActivity = i18n.Msg{En: "Activity", Es: "Actividad", Zh: "活动", De: "Aktivität", Fr: "Activité"}
)
