package app

import "iworkflow/internal/core/i18n"

// Texts of the shell and of the placeholders the composition root owns. The module texts live in
// their own presentation packages.
var (
	msgBrand              = i18n.Msg{En: "iworkflow", Es: "iworkflow", Zh: "iworkflow", De: "iworkflow", Fr: "iworkflow"}
	msgMenu               = i18n.Msg{En: "Menu", Es: "Menú", Zh: "菜单", De: "Menü", Fr: "Menu"}
	msgNavigation         = i18n.Msg{En: "Navigation", Es: "Navegación", Zh: "导航", De: "Navigation", Fr: "Navigation"}
	msgEmptyGroup         = i18n.Msg{En: "This group has no tabs. Open one:", Es: "Este grupo no tiene pestañas. Abre una:", Zh: "此分组没有标签页。请打开一个：", De: "Diese Gruppe hat keine Tabs. Öffne einen:", Fr: "Ce groupe n'a aucun onglet. Ouvrez-en un :"}
	msgNotMigrated        = i18n.Msg{En: "This screen has not been migrated to the Go server yet.", Es: "Esta pantalla todavía no se ha migrado al servidor Go.", Zh: "此页面尚未迁移到 Go 服务器。", De: "Dieser Bereich wurde noch nicht auf den Go-Server migriert.", Fr: "Cet écran n'a pas encore été migré vers le serveur Go."}
	msgResources          = i18n.Msg{En: "Resources", Es: "Recursos", Zh: "资源", De: "Ressourcen", Fr: "Ressources"}
	msgSettingsLang       = i18n.Msg{En: "Language", Es: "Idioma", Zh: "语言", De: "Sprache", Fr: "Langue"}
	msgSettingsTerminal   = i18n.Msg{En: "Terminal", Es: "Terminal", Zh: "终端", De: "Terminal", Fr: "Terminal"}
	msgSettingsAppearance = i18n.Msg{En: "Appearance", Es: "Apariencia", Zh: "外观", De: "Darstellung", Fr: "Apparence"}
	msgSettingsNet        = i18n.Msg{En: "Network", Es: "Red", Zh: "网络", De: "Netzwerk", Fr: "Réseau"}
	msgTitleTasks         = i18n.Msg{En: "Task", Es: "Tarea", Zh: "任务", De: "Aufgabe", Fr: "Tâches"}

	msgPaletteTitle       = i18n.Msg{En: "Command palette", Es: "Paleta de comandos", Zh: "命令面板", De: "Befehlspalette", Fr: "Palette de commandes"}
	msgPalettePlaceholder = i18n.Msg{En: "Go to…", Es: "Ir a…", Zh: "跳转到…", De: "Gehe zu …", Fr: "Aller à…"}
	msgPaletteEmpty       = i18n.Msg{En: "No results", Es: "Sin resultados", Zh: "没有结果", De: "Keine Ergebnisse", Fr: "Aucun résultat"}
	msgPaletteHint        = i18n.Msg{En: "↑↓ move · Enter open · Esc close", Es: "↑↓ mover · Intro abrir · Esc cerrar", Zh: "↑↓ 移动 · Enter 打开 · Esc 关闭", De: "↑↓ bewegen · Enter öffnen · Esc schließen", Fr: "↑↓ déplacer · Entrée ouvrir · Échap fermer"}
)
