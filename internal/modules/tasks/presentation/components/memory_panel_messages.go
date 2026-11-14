package components

import "iworkflow/internal/core/i18n"

// Texts of the Memory panel of the task detail page. Memory content is never shown, only the type, title and date.
var (
	msgMemoryPanelTitle = i18n.Msg{En: "Memory", Es: "Memoria", Zh: "记忆", De: "Gedächtnis", Fr: "Mémoire"}
	msgMemoryPanelHint  = i18n.Msg{
		En: "Recent memories of this project (Engram)",
		Es: "Memorias recientes de este proyecto (Engram)",
		Zh: "此项目的近期记忆（Engram）",
		De: "Letzte Erinnerungen dieses Projekts (Engram)",
		Fr: "Souvenirs récents de ce projet (Engram)",
	}
)
