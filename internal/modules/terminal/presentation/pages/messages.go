package pages

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// User-facing texts of the terminal pages. Each one copies the Rust `Msg` constant it replaces
// (src/modules/terminal/presentation/pages/**) or the app title it shares (src/app.rs).
var (
	msgTerminals        = i18n.Msg{En: "Terminals", Es: "Terminales", Zh: "终端", De: "Terminals", Fr: "Terminaux"}
	msgNewHomeTerminal  = i18n.Msg{En: "New terminal in $HOME", Es: "Nueva terminal en $HOME", Zh: "在 $HOME 中新建终端", De: "Neues Terminal in $HOME", Fr: "Nouveau terminal dans $HOME"}
	msgRefresh          = i18n.Msg{En: "Refresh", Es: "Actualizar", Zh: "刷新", De: "Aktualisieren", Fr: "Actualiser"}
	msgSessions         = i18n.Msg{En: "Sessions", Es: "Sesiones", Zh: "会话", De: "Sitzungen", Fr: "Sessions"}
	msgAllTerminals     = i18n.Msg{En: "All terminals", Es: "Todas las terminales", Zh: "所有终端", De: "Alle Terminals", Fr: "Tous les terminaux"}
	msgCloseSession     = i18n.Msg{En: "Close session", Es: "Cerrar sesión", Zh: "关闭会话", De: "Sitzung schließen", Fr: "Fermer la session"}
	msgOpeningTerminal  = i18n.Msg{En: "Opening terminal", Es: "Abriendo terminal", Zh: "正在打开终端", De: "Terminal wird geöffnet", Fr: "Ouverture du terminal"}
	msgOpeningLoading   = i18n.Msg{En: "Opening terminal…", Es: "Abriendo terminal…", Zh: "正在打开终端…", De: "Terminal wird geöffnet…", Fr: "Ouverture du terminal…"}
	msgBackToWorkspaces = i18n.Msg{En: "Back to workspaces", Es: "Volver a workspaces", Zh: "返回工作区", De: "Zurück zu den Workspaces", Fr: "Retour aux espaces de travail"}
	msgBackToBoard      = i18n.Msg{En: "Back to board", Es: "Volver al tablero", Zh: "返回看板", De: "Zurück zum Board", Fr: "Retour au tableau"}
)

// TerminalsTitle is the title of the sessions list tab.
func TerminalsTitle(ctx context.Context) string {
	return msgTerminals.T(ctx)
}
