// Package state builds the view-models of the agent settings screens from the data the handlers
// read. It holds no business logic: the choices and validation live in the use cases.
package state

import "iworkflow/internal/core/i18n"

// User-facing texts. Each one copies the Rust Msg constant of the same name
// (src/modules/agent_settings/presentation/**), so every language keeps the original wording.
var (
	msgAppliesToNewTasksHint = i18n.Msg{
		En: "Changes apply to newly started tasks, not to ones already running or being resumed.",
		Es: "Los cambios se aplican a las tareas nuevas que se inicien, no a las que ya están en marcha ni a las que se reanudan.",
		Zh: "更改适用于新启动的任务，不适用于已在运行或正在恢复的任务。",
		De: "Änderungen gelten für neu gestartete Aufgaben, nicht für bereits laufende oder fortgesetzte.",
		Fr: "Les modifications s'appliquent aux nouvelles tâches démarrées, pas à celles déjà en cours ni à celles qui sont reprises.",
	}
	msgAgent             = i18n.Msg{En: "Agent", Es: "Agente", Zh: "智能体", De: "Agent", Fr: "Agent"}
	msgModel             = i18n.Msg{En: "Model", Es: "Modelo", Zh: "模型", De: "Modell", Fr: "Modèle"}
	msgProfile           = i18n.Msg{En: "Profile", Es: "Perfil", Zh: "配置文件", De: "Profil", Fr: "Profil"}
	msgThinking          = i18n.Msg{En: "Reasoning", Es: "Razonamiento", Zh: "推理", De: "Reasoning", Fr: "Raisonnement"}
	msgDefault           = i18n.Msg{En: "Default", Es: "Por defecto", Zh: "默认", De: "Standard", Fr: "Par défaut"}
	msgAppliesToNewTasks = i18n.Msg{
		En: "Applies to new tasks.",
		Es: "Se aplica a las tareas nuevas.",
		Zh: "适用于新任务。",
		De: "Gilt für neue Aufgaben.",
		Fr: "S'applique aux nouvelles tâches.",
	}
	msgActiveSuffix = i18n.Msg{En: "{name} (active)", Es: "{name} (activo)", Zh: "{name}（当前）", De: "{name} (aktiv)", Fr: "{name} (actif)"}

	msgThinkingOff     = i18n.Msg{En: "No reasoning", Es: "No reasoning", Zh: "无推理", De: "Kein Reasoning", Fr: "Sans raisonnement"}
	msgThinkingMinimal = i18n.Msg{En: "Minimal", Es: "Mínimo", Zh: "最低", De: "Minimal", Fr: "Minimal"}
	msgThinkingLow     = i18n.Msg{En: "Low", Es: "Bajo", Zh: "低", De: "Niedrig", Fr: "Faible"}
	msgThinkingMedium  = i18n.Msg{En: "Medium", Es: "Medium", Zh: "中", De: "Mittel", Fr: "Moyen"}
	msgThinkingHigh    = i18n.Msg{En: "High", Es: "Alto", Zh: "高", De: "Hoch", Fr: "Élevé"}
	msgThinkingXHigh   = i18n.Msg{En: "Very high", Es: "Very high", Zh: "非常高", De: "Sehr hoch", Fr: "Très élevé"}
	msgThinkingMax     = i18n.Msg{En: "Maximum", Es: "Maximum", Zh: "最高", De: "Maximal", Fr: "Maximum"}

	msgResourceLimits      = i18n.Msg{En: "Resource limits", Es: "Límites de recursos", Zh: "资源限制", De: "Ressourcenlimits", Fr: "Limites de ressources"}
	msgLimitAgentResources = i18n.Msg{En: "Limit agent resources", Es: "Limitar recursos del agente", Zh: "限制智能体资源", De: "Agent-Ressourcen begrenzen", Fr: "Limiter les ressources de l'agent"}
	msgNoLimit             = i18n.Msg{En: "no limit", Es: "sin límite", Zh: "无限制", De: "kein Limit", Fr: "sans limite"}
	msgLimitsHelp          = i18n.Msg{
		En: "Limits the RAM and CPU of each task's agent (Linux with systemd only). Each task can set its own values, which take precedence over these. Applies to tasks started or resumed from now on.",
		Es: "Limita la RAM y la CPU del agente de cada tarea (solo Linux con systemd). Cada tarea puede fijar sus propios valores, que prevalecen sobre estos. Se aplica a las tareas que se inicien o reanuden a partir de ahora.",
		Zh: "限制每个任务中智能体的内存和 CPU（仅限使用 systemd 的 Linux）。每个任务可以设置自己的值，并优先于此处的值。适用于从现在起启动或恢复的任务。",
		De: "Begrenzt RAM und CPU des Agents jeder Aufgabe (nur Linux mit systemd). Jede Aufgabe kann eigene Werte festlegen, die Vorrang vor diesen haben. Gilt für Aufgaben, die ab jetzt gestartet oder fortgesetzt werden.",
		Fr: "Limite la RAM et le CPU de l'agent de chaque tâche (Linux avec systemd uniquement). Chaque tâche peut définir ses propres valeurs, qui prévalent sur celles-ci. S'applique aux tâches démarrées ou reprises à partir de maintenant.",
	}
)
