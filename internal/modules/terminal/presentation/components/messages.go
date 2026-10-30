package components

import "iworkflow/internal/core/i18n"

// User-facing texts of the terminal components. Each one copies the Rust `Msg` constant it replaces
// (src/modules/terminal/presentation/components/**).
var (
	msgNoTerminals = i18n.Msg{En: "No open terminals.", Es: "No hay terminales abiertas.", Zh: "没有打开的终端。", De: "Keine Terminals geöffnet.", Fr: "Aucun terminal ouvert."}
	msgWorkspace   = i18n.Msg{En: "Workspace", Es: "Workspace", Zh: "工作区", De: "Workspace", Fr: "Espace de travail"}
	msgSession     = i18n.Msg{En: "Session", Es: "Sesión", Zh: "会话", De: "Sitzung", Fr: "Session"}
	msgStarted     = i18n.Msg{En: "Started", Es: "Inicio", Zh: "开始时间", De: "Start", Fr: "Début"}
	msgStatus      = i18n.Msg{En: "Status", Es: "Estado", Zh: "状态", De: "Status", Fr: "Statut"}
	msgOpen        = i18n.Msg{En: "Open", Es: "Abrir", Zh: "打开", De: "Öffnen", Fr: "Ouvrir"}
	msgClose       = i18n.Msg{En: "Close", Es: "Cerrar", Zh: "关闭", De: "Schließen", Fr: "Fermer"}
	msgSpecialKeys = i18n.Msg{En: "Special keys", Es: "Teclas especiales", Zh: "特殊按键", De: "Sondertasten", Fr: "Touches spéciales"}
)
