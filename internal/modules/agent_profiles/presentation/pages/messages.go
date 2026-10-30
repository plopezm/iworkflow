// Package pages holds the full screens of the agent_profiles tabs. A page is the content of a dock tab.
package pages

import "iworkflow/internal/core/i18n"

// Texts of the two screens. Each one copies the Rust Msg constant of the same name in
// src/app.rs (TabTitle, ModelsTitle) and src/modules/agent_profiles/presentation/pages/*.rs.
var (
	// TabTitle is the title of the profiles tab and of its page heading.
	TabTitle = i18n.Msg{En: "Profiles", Es: "Perfiles", Zh: "配置文件", De: "Profile", Fr: "Profils"}
	// ModelsTitle is the title of the models tab.
	ModelsTitle = i18n.Msg{En: "Models", Es: "Modelos", Zh: "模型", De: "Modelle", Fr: "Modèles"}
	// ModelsHelp explains the global routing.
	ModelsHelp = i18n.Msg{
		En: "Global routing of models and effort per agent; profiles store snapshots of this routing; it does not apply to the current pi session until the next subagent launch.",
		Es: "Routing global de modelos y esfuerzo por agente; los perfiles guardan snapshots de este routing; no se aplica a la sesión de pi actual hasta el siguiente lanzamiento de subagente.",
		Zh: "按智能体划分的模型和推理强度全局路由；配置文件会保存此路由的快照；在下一次启动子智能体之前，不会应用到当前的 pi 会话。",
		De: "Globales Routing von Modellen und Aufwand pro Agent; Profile speichern Snapshots dieses Routings; es gilt für die aktuelle pi-Sitzung erst ab dem nächsten Start eines Subagents.",
		Fr: "Routage global des modèles et de l'effort par agent ; les profils enregistrent des instantanés de ce routage ; il ne s'applique pas à la session pi en cours avant le prochain lancement d'un sous-agent.",
	}
	// CreateRole opens the role dialog from the models screen.
	CreateRole = i18n.Msg{En: "Create role", Es: "Crear rol", Zh: "创建角色", De: "Rolle erstellen", Fr: "Créer un rôle"}
	// GlobalRouting is the title of the routing card.
	GlobalRouting = i18n.Msg{En: "Global routing", Es: "Routing global", Zh: "全局路由", De: "Globales Routing", Fr: "Routage global"}
)
