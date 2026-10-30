package components

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// Status labels of a task, copied from the Rust TaskStatusDto::label (src/modules/tasks/infrastructure/
// adapters/inbound/dto/task_dto.rs). The keys are the persisted status names.
var statusLabels = map[string]i18n.Msg{
	"backlog":         {En: "To do", Es: "Por hacer", Zh: "待办", De: "Offen", Fr: "À faire"},
	"starting":        {En: "Starting", Es: "Arrancando", Zh: "启动中", De: "Startet", Fr: "Démarrage"},
	"running":         {En: "Working", Es: "Trabajando", Zh: "工作中", De: "Arbeitet", Fr: "En cours"},
	"idle":            {En: "Waiting", Es: "Esperando", Zh: "等待中", De: "Wartet", Fr: "En attente"},
	"finishing":       {En: "Finishing", Es: "Finalizando", Zh: "收尾中", De: "Wird abgeschlossen", Fr: "Finalisation"},
	"awaiting_review": {En: "Awaiting review", Es: "Pendiente de revisión", Zh: "待审核", De: "Wartet auf Review", Fr: "En attente de revue"},
	"merged":          {En: "Merged", Es: "Integrada", Zh: "已合并", De: "Zusammengeführt", Fr: "Fusionnée"},
	"discarded":       {En: "Discarded", Es: "Descartada", Zh: "已丢弃", De: "Verworfen", Fr: "Abandonnée"},
	"conflict":        {En: "Conflict", Es: "Conflicto", Zh: "冲突", De: "Konflikt", Fr: "Conflit"},
	"failed":          {En: "Failed", Es: "Fallida", Zh: "失败", De: "Fehlgeschlagen", Fr: "Échouée"},
	"interrupted":     {En: "Interrupted", Es: "Interrumpida", Zh: "已中断", De: "Unterbrochen", Fr: "Interrompue"},
	"rate_limited":    {En: "Usage limit", Es: "Límite de uso", Zh: "已达使用上限", De: "Nutzungslimit", Fr: "Limite d'usage"},
}

// statusVariants is the badge colour of each status, as the Rust TaskStatusDto::variant names it.
var statusVariants = map[string]string{
	"backlog":         "muted",
	"starting":        "info",
	"running":         "info",
	"idle":            "warn",
	"finishing":       "info",
	"awaiting_review": "warn",
	"merged":          "ok",
	"discarded":       "muted",
	"conflict":        "error",
	"failed":          "error",
	"interrupted":     "error",
	"rate_limited":    "warn",
}

// statusText is the label of a status in the language of the request. A status this version does not know is
// shown as it is, so a newer database never hides a task.
func statusText(ctx context.Context, status string) string {
	label, ok := statusLabels[status]
	if !ok {
		return status
	}
	return label.T(ctx)
}

// statusVariant is the badge colour of a status; an unknown one is muted.
func statusVariant(status string) string {
	if variant, ok := statusVariants[status]; ok {
		return variant
	}
	return "muted"
}
