package app

import "iworkflow/internal/core/i18n"

// Texts of the shell and of the placeholders the composition root owns. The module texts live in
// their own presentation packages.
var (
	msgBrand          = i18n.Msg{En: "iworkflow", Es: "iworkflow", Zh: "iworkflow", De: "iworkflow", Fr: "iworkflow"}
	msgMenu           = i18n.Msg{En: "Menu", Es: "Menú", Zh: "菜单", De: "Menü", Fr: "Menu"}
	msgNavigation     = i18n.Msg{En: "Navigation", Es: "Navegación", Zh: "导航", De: "Navigation", Fr: "Navigation"}
	msgEmptyGroup     = i18n.Msg{En: "This group has no tabs. Open one:", Es: "Este grupo no tiene pestañas. Abre una:", Zh: "此分组没有标签页。请打开一个：", De: "Diese Gruppe hat keine Tabs. Öffne einen:", Fr: "Ce groupe n'a aucun onglet. Ouvrez-en un :"}
	msgNotMigrated    = i18n.Msg{En: "This screen has not been migrated to the Go server yet.", Es: "Esta pantalla todavía no se ha migrado al servidor Go.", Zh: "此页面尚未迁移到 Go 服务器。", De: "Dieser Bereich wurde noch nicht auf den Go-Server migriert.", Fr: "Cet écran n'a pas encore été migré vers le serveur Go."}
	msgResources      = i18n.Msg{En: "Resources", Es: "Recursos", Zh: "资源", De: "Ressourcen", Fr: "Ressources"}
	msgSettingsLang   = i18n.Msg{En: "Language", Es: "Idioma", Zh: "语言", De: "Sprache", Fr: "Langue"}
	msgSettingsNet    = i18n.Msg{En: "Network", Es: "Red", Zh: "网络", De: "Netzwerk", Fr: "Réseau"}
	msgTitleBoard     = i18n.Msg{En: "Board", Es: "Tablero", Zh: "看板", De: "Board", Fr: "Board"}
	msgTitleTerminals = i18n.Msg{En: "Terminals", Es: "Terminales", Zh: "终端", De: "Terminals", Fr: "Terminaux"}
	msgTitleProfiles  = i18n.Msg{En: "Profiles", Es: "Perfiles", Zh: "配置文件", De: "Profile", Fr: "Profils"}
	msgTitleModels    = i18n.Msg{En: "Models", Es: "Modelos", Zh: "模型", De: "Modelle", Fr: "Modèles"}
	msgTitleTasks     = i18n.Msg{En: "Task", Es: "Tarea", Zh: "任务", De: "Aufgabe", Fr: "Tâche"}
	msgTitleJira      = i18n.Msg{En: "Jira", Es: "Jira", Zh: "Jira", De: "Jira", Fr: "Jira"}
)
