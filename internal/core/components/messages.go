package components

import "iworkflow/internal/core/i18n"

// User-facing texts of the shared components. Each one copies the Rust `Msg` constant of the
// same component (src/core/components/*.rs) so every language keeps the original wording.
var (
	msgLoading = i18n.Msg{En: "Loading", Es: "Cargando", Zh: "加载中", De: "Wird geladen", Fr: "Chargement"}

	msgCancel = i18n.Msg{En: "Cancel", Es: "Cancelar", Zh: "取消", De: "Abbrechen", Fr: "Annuler"}
	msgClose  = i18n.Msg{En: "Close", Es: "Cerrar", Zh: "关闭", De: "Schließen", Fr: "Fermer"}

	msgInProgress = i18n.Msg{En: "In progress", Es: "En curso", Zh: "进行中", De: "Läuft", Fr: "En cours"}

	msgCloseNotification = i18n.Msg{
		En: "Close notification",
		Es: "Cerrar notificación",
		Zh: "关闭通知",
		De: "Benachrichtigung schließen",
		Fr: "Fermer la notification",
	}

	msgFileContent = i18n.Msg{En: "File content", Es: "Contenido del fichero", Zh: "文件内容", De: "Dateiinhalt", Fr: "Contenu du fichier"}

	msgThemeLight = i18n.Msg{En: "☀ Light", Es: "☀ Claro", Zh: "☀ 浅色", De: "☀ Hell", Fr: "☀ Clair"}
	msgThemeDark  = i18n.Msg{En: "☾ Dark", Es: "☾ Oscuro", Zh: "☾ 深色", De: "☾ Dunkel", Fr: "☾ Sombre"}
	msgThemeAuto  = i18n.Msg{En: "Auto", Es: "Auto", Zh: "自动", De: "Auto", Fr: "Auto"}
	msgTheme      = i18n.Msg{En: "Theme", Es: "Tema", Zh: "主题", De: "Design", Fr: "Thème"}

	msgLanguageTitle = i18n.Msg{En: "Language", Es: "Idioma", Zh: "语言", De: "Sprache", Fr: "Langue"}
	msgLanguageHelp  = i18n.Msg{
		En: "Language of the interface. The page reloads to apply the change.",
		Es: "Idioma de la interfaz. La página se recarga para aplicar el cambio.",
		Zh: "界面语言。更改后页面将重新加载。",
		De: "Sprache der Oberfläche. Die Seite wird neu geladen, um die Änderung zu übernehmen.",
		Fr: "Langue de l'interface. La page se recharge pour appliquer le changement.",
	}
	msgPromptTitle = i18n.Msg{
		En: "Choose your language",
		Es: "Elige tu idioma",
		Zh: "选择你的语言",
		De: "Wähle deine Sprache",
		Fr: "Choisissez votre langue",
	}
	msgPromptHelp = i18n.Msg{
		En: "You can change it later in Settings.",
		Es: "Puedes cambiarlo más tarde en Configuración.",
		Zh: "之后可以在“设置”中更改。",
		De: "Du kannst dies später in den Einstellungen ändern.",
		Fr: "Vous pourrez le modifier plus tard dans les Paramètres.",
	}

	msgClosePanel    = i18n.Msg{En: "Close panel", Es: "Cerrar panel", Zh: "关闭面板", De: "Bereich schließen", Fr: "Fermer le panneau"}
	msgCollapsePanel = i18n.Msg{En: "Collapse panel", Es: "Colapsar panel", Zh: "折叠面板", De: "Bereich einklappen", Fr: "Réduire le panneau"}
	msgExpandPanel   = i18n.Msg{En: "Expand panel", Es: "Expandir panel", Zh: "展开面板", De: "Bereich ausklappen", Fr: "Développer le panneau"}
	msgHideSidebar   = i18n.Msg{En: "Hide sidebar", Es: "Ocultar barra lateral", Zh: "隐藏侧边栏", De: "Seitenleiste ausblenden", Fr: "Masquer la barre latérale"}
	msgShowSidebar   = i18n.Msg{En: "Show sidebar", Es: "Mostrar barra lateral", Zh: "显示侧边栏", De: "Seitenleiste einblenden", Fr: "Afficher la barre latérale"}
	msgSidebar       = i18n.Msg{En: "Sidebar", Es: "Barra lateral", Zh: "侧边栏", De: "Seitenleiste", Fr: "Barre latérale"}
	msgMoreActions   = i18n.Msg{En: "More actions", Es: "Más acciones", Zh: "更多操作", De: "Weitere Aktionen", Fr: "Plus d'actions"}
	msgNeedsAttn     = i18n.Msg{En: "Needs attention", Es: "Necesita atención", Zh: "需要关注", De: "Erfordert Aufmerksamkeit", Fr: "Nécessite votre attention"}
	msgOpenMenu      = i18n.Msg{En: "Open menu", Es: "Abrir menú", Zh: "打开菜单", De: "Menü öffnen", Fr: "Ouvrir le menu"}
)
