package components

import (
	"context"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Texts of the hook status strip of a task, and the Hooks entry of the workspace menu. The hooks themselves are
// configured in the Hooks tab of a workspace.
var (
	msgAddHookMenu      = i18n.Msg{En: "Add hook", Es: "Añadir hook", Zh: "添加 hook", De: "Hook hinzufügen", Fr: "Ajouter un hook"}
	msgHooksMenu        = i18n.Msg{En: "Hooks", Es: "Hooks", Zh: "钩子", De: "Hooks", Fr: "Hooks"}
	msgTaskHooks        = i18n.Msg{En: "Hooks", Es: "Hooks", Zh: "钩子", De: "Hooks", Fr: "Hooks"}
	msgHookPhasePre     = i18n.Msg{En: "Before", Es: "Antes de", Zh: "之前", De: "Vor", Fr: "Avant"}
	msgHookPhasePost    = i18n.Msg{En: "After", Es: "Después de", Zh: "之后", De: "Nach", Fr: "Après"}
	msgHookEventStart   = i18n.Msg{En: "task start", Es: "inicio de la tarea", Zh: "任务开始", De: "Aufgabenstart", Fr: "début de la tâche"}
	msgHookEventFinish  = i18n.Msg{En: "task finish", Es: "fin de la tarea", Zh: "任务结束", De: "Aufgabenende", Fr: "fin de la tâche"}
	msgHookKindScript   = i18n.Msg{En: "Script", Es: "Script", Zh: "脚本", De: "Skript", Fr: "Script"}
	msgHookKindAgent    = i18n.Msg{En: "Agent", Es: "Agente", Zh: "智能体", De: "Agent", Fr: "Agent"}
	msgHookStatusPend   = i18n.Msg{En: "Pending", Es: "Pendiente", Zh: "待执行", De: "Ausstehend", Fr: "En attente"}
	msgHookStatusRun    = i18n.Msg{En: "Running", Es: "En curso", Zh: "运行中", De: "Läuft", Fr: "En cours"}
	msgHookStatusOK     = i18n.Msg{En: "OK", Es: "Correcto", Zh: "成功", De: "OK", Fr: "OK"}
	msgHookStatusFailed = i18n.Msg{En: "Failed", Es: "Fallido", Zh: "失败", De: "Fehlgeschlagen", Fr: "Échoué"}
	msgHookStatusTime   = i18n.Msg{En: "Timed out", Es: "Tiempo agotado", Zh: "超时", De: "Zeitüberschreitung", Fr: "Délai dépassé"}
	msgHookStatusError  = i18n.Msg{En: "Error", Es: "Error", Zh: "错误", De: "Fehler", Fr: "Erreur"}
	msgHookStatusSkip   = i18n.Msg{En: "Skipped", Es: "Omitido", Zh: "已跳过", De: "Übersprungen", Fr: "Ignoré"}
	msgHookSkip         = i18n.Msg{En: "Skip", Es: "Omitir", Zh: "跳过", De: "Überspringen", Fr: "Ignorer"}
	msgHookInclude      = i18n.Msg{En: "Include", Es: "Incluir", Zh: "包含", De: "Einschließen", Fr: "Inclure"}
	msgHookLog          = i18n.Msg{En: "Log", Es: "Registro", Zh: "日志", De: "Protokoll", Fr: "Journal"}
	msgHookExitCode     = i18n.Msg{En: "Exit code", Es: "Código de salida", Zh: "退出码", De: "Exit-Code", Fr: "Code de sortie"}
	msgHookDisabled     = i18n.Msg{En: "Disabled in the workspace", Es: "Desactivado en el proyecto", Zh: "在工作区中已停用", De: "Im Workspace deaktiviert", Fr: "Désactivé dans l’espace de travail"}
)

// hookPhaseLabel names the phase of a hook: before or after the event.
func hookPhaseLabel(ctx context.Context, phase string) string {
	switch phase {
	case "pre":
		return msgHookPhasePre.T(ctx)
	case "post":
		return msgHookPhasePost.T(ctx)
	default:
		return phase
	}
}

// hookEventLabel names the task event a hook is attached to.
func hookEventLabel(ctx context.Context, event string) string {
	switch event {
	case "task_start":
		return msgHookEventStart.T(ctx)
	case "task_finish":
		return msgHookEventFinish.T(ctx)
	default:
		return event
	}
}

// hookKindLabel names how a hook does its work.
func hookKindLabel(ctx context.Context, kind string) string {
	if kind == "agent" {
		return msgHookKindAgent.T(ctx)
	}
	return msgHookKindScript.T(ctx)
}

// hookStatusLabel is the text of the status badge of a hook.
func hookStatusLabel(ctx context.Context, status state.HookStatus) string {
	switch status {
	case state.HookStatusRunning:
		return msgHookStatusRun.T(ctx)
	case state.HookStatusOK:
		return msgHookStatusOK.T(ctx)
	case state.HookStatusFailed:
		return msgHookStatusFailed.T(ctx)
	case state.HookStatusTimeout:
		return msgHookStatusTime.T(ctx)
	case state.HookStatusError:
		return msgHookStatusError.T(ctx)
	case state.HookStatusSkipped:
		return msgHookStatusSkip.T(ctx)
	default:
		return msgHookStatusPend.T(ctx)
	}
}

// hookStatusVariant is the colour of the status badge of a hook.
func hookStatusVariant(status state.HookStatus) string {
	switch status {
	case state.HookStatusRunning:
		return "info"
	case state.HookStatusOK:
		return "ok"
	case state.HookStatusTimeout:
		return "warn"
	case state.HookStatusFailed, state.HookStatusError:
		return "error"
	default:
		return "muted"
	}
}

// hookToggleLabel is the label of the skip toggle: it does the opposite of the current state.
func hookToggleLabel(ctx context.Context, skipped bool) string {
	if skipped {
		return msgHookInclude.T(ctx)
	}
	return msgHookSkip.T(ctx)
}
