package components

import "iworkflow/internal/core/i18n"

// User-facing texts of the usage bar.
var (
	msgWeek        = i18n.Msg{En: "7d", Es: "7d", Zh: "7d", De: "7d", Fr: "7d"}
	msgStale       = i18n.Msg{En: "stale", Es: "desactualizado", Zh: "过期", De: "veraltet", Fr: "périmé"}
	msgUnavailable = i18n.Msg{En: "{name}: unavailable", Es: "{name}: no disponible", Zh: "{name}：不可用", De: "{name}: nicht verfügbar", Fr: "{name} : indisponible"}
)
