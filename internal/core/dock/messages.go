package dock

import "iworkflow/internal/core/i18n"

// User-facing texts of the dock. Those that existed in the Rust version copy its Msg constants.
// The three split and menu labels were hard-coded in Spanish there; they are translated here.
var (
	msgCloseOthers = i18n.Msg{En: "Close others", Es: "Cerrar las demás", Zh: "关闭其他", De: "Andere schließen", Fr: "Fermer les autres"}
	msgCloseSplit  = i18n.Msg{En: "Close split", Es: "Cerrar división", Zh: "关闭分屏", De: "Teilung schließen", Fr: "Fermer la division"}
	msgCloseTab    = i18n.Msg{En: "Close tab", Es: "Cerrar pestaña", Zh: "关闭标签页", De: "Tab schließen", Fr: "Fermer l'onglet"}
	msgMoveDown    = i18n.Msg{En: "Move tab down", Es: "Mover pestaña abajo", Zh: "将标签页移到下方", De: "Tab nach unten verschieben", Fr: "Déplacer l'onglet vers le bas"}
	msgMoveRight   = i18n.Msg{En: "Move tab to the right", Es: "Mover pestaña a la derecha", Zh: "将标签页移到右侧", De: "Tab nach rechts verschieben", Fr: "Déplacer l'onglet vers la droite"}
	msgNewTab      = i18n.Msg{En: "New tab", Es: "Nueva pestaña", Zh: "新建标签页", De: "Neuer Tab", Fr: "Nouvel onglet"}

	msgGroupOptions = i18n.Msg{En: "Group options", Es: "Opciones del grupo", Zh: "分组选项", De: "Gruppenoptionen", Fr: "Options du groupe"}
	msgSplitRight   = i18n.Msg{En: "Split right", Es: "Dividir a la derecha", Zh: "向右分屏", De: "Rechts teilen", Fr: "Diviser à droite"}
	msgSplitDown    = i18n.Msg{En: "Split down", Es: "Dividir hacia abajo", Zh: "向下分屏", De: "Nach unten teilen", Fr: "Diviser vers le bas"}
)
