package pages

import "iworkflow/internal/core/i18n"

// The texts below copy the Rust Msg constants of the project_tree pages, in the same five languages.

var (
	msgProject        = i18n.Msg{En: "Project", Es: "Proyecto", Zh: "项目", De: "Projekt", Fr: "Projet"}
	msgProjectActions = i18n.Msg{En: "Project actions", Es: "Acciones del proyecto", Zh: "项目操作", De: "Projektaktionen", Fr: "Actions du projet"}
	msgOpenTerminal   = i18n.Msg{En: "Open terminal", Es: "Abrir terminal", Zh: "打开终端", De: "Terminal öffnen", Fr: "Ouvrir un terminal"}
	msgEditWorkspace  = i18n.Msg{En: "Edit workspace", Es: "Editar workspace", Zh: "编辑工作区", De: "Workspace bearbeiten", Fr: "Modifier l'espace de travail"}
	msgSearchContent  = i18n.Msg{En: "Search in files", Es: "Buscar en ficheros", Zh: "在文件中搜索", De: "In Dateien suchen", Fr: "Rechercher dans les fichiers"}
	msgMatchCase      = i18n.Msg{En: "Match case", Es: "Distinguir mayúsculas", Zh: "区分大小写", De: "Groß-/Kleinschreibung", Fr: "Respecter la casse"}
	msgRegex          = i18n.Msg{En: "Regular expression", Es: "Expresión regular", Zh: "正则表达式", De: "Regulärer Ausdruck", Fr: "Expression régulière"}
	msgWholeWord      = i18n.Msg{En: "Whole word", Es: "Palabra completa", Zh: "全字匹配", De: "Ganzes Wort", Fr: "Mot entier"}
	msgReload         = i18n.Msg{En: "Reload", Es: "Recargar", Zh: "重新加载", De: "Neu laden", Fr: "Recharger"}
	msgFindInFile     = i18n.Msg{En: "Find in file", Es: "Buscar en el fichero", Zh: "在文件中查找", De: "In Datei suchen", Fr: "Rechercher dans le fichier"}
	msgPrevMatch      = i18n.Msg{En: "Previous match", Es: "Coincidencia anterior", Zh: "上一个匹配", De: "Vorheriger Treffer", Fr: "Occurrence précédente"}
	msgNextMatch      = i18n.Msg{En: "Next match", Es: "Coincidencia siguiente", Zh: "下一个匹配", De: "Nächster Treffer", Fr: "Occurrence suivante"}
	msgCloseSearch    = i18n.Msg{En: "Close search", Es: "Cerrar búsqueda", Zh: "关闭搜索", De: "Suche schließen", Fr: "Fermer la recherche"}
)

// TabTitle is the title of the project tree tab (the Rust TITLE_PROJECT constant).
var TabTitle = msgProject
