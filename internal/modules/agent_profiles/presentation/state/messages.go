// Package state holds the view-models of the agent_profiles screens and the pure rules that shape
// them: selection, routing rows and name hints. It has no I/O: handlers build the views from the
// use cases and render them with the components of the presentation layer.
package state

import "iworkflow/internal/core/i18n"

// Toast texts. Each one copies the Rust Msg constant of the same name in
// src/modules/agent_profiles/presentation/state/*.rs.
var (
	ProfileCreated    = i18n.Msg{En: "Profile created", Es: "Perfil creado", Zh: "配置文件已创建", De: "Profil erstellt", Fr: "Profil créé"}
	ProfileDuplicated = i18n.Msg{En: "Profile duplicated", Es: "Perfil duplicado", Zh: "配置文件已复制", De: "Profil dupliziert", Fr: "Profil dupliqué"}
	ProfileRenamed    = i18n.Msg{En: "Profile renamed", Es: "Perfil renombrado", Zh: "配置文件已重命名", De: "Profil umbenannt", Fr: "Profil renommé"}
	ProfileDeleted    = i18n.Msg{En: "Profile deleted", Es: "Perfil eliminado", Zh: "配置文件已删除", De: "Profil gelöscht", Fr: "Profil supprimé"}
	ProfileImported   = i18n.Msg{En: "Profile imported", Es: "Perfil importado", Zh: "配置文件已导入", De: "Profil importiert", Fr: "Profil importé"}
	RoutingSaved      = i18n.Msg{En: "Routing saved", Es: "Enrutado guardado", Zh: "路由已保存", De: "Routing gespeichert", Fr: "Routage enregistré"}
	Exported          = i18n.Msg{En: "Exported {file}", Es: "Exportado {file}", Zh: "已导出 {file}", De: "{file} exportiert", Fr: "{file} exporté"}
	RoleCreated       = i18n.Msg{En: "Role created", Es: "Rol creado", Zh: "角色已创建", De: "Rolle erstellt", Fr: "Rôle créé"}

	GlobalRoutingSavedNone = i18n.Msg{En: "Global routing saved (no agents)", Es: "Routing global guardado (sin agentes)", Zh: "全局路由已保存（无智能体）", De: "Globales Routing gespeichert (keine Agents)", Fr: "Routage global enregistré (aucun agent)"}
	GlobalRoutingSavedOne  = i18n.Msg{En: "Global routing saved (1 agent)", Es: "Routing global guardado (1 agente)", Zh: "全局路由已保存（1 个智能体）", De: "Globales Routing gespeichert (1 Agent)", Fr: "Routage global enregistré (1 agent)"}
	GlobalRoutingSavedMany = i18n.Msg{En: "Global routing saved ({n} agents)", Es: "Routing global guardado ({n} agentes)", Zh: "全局路由已保存（{n} 个智能体）", De: "Globales Routing gespeichert ({n} Agents)", Fr: "Routage global enregistré ({n} agents)"}
)

// Name hint texts (name_rules.rs).
var (
	hintEmptyName   = i18n.Msg{En: "The name cannot be empty", Es: "El nombre no puede estar vacío", Zh: "名称不能为空", De: "Der Name darf nicht leer sein", Fr: "Le nom ne peut pas être vide"}
	hintNameTooLong = i18n.Msg{En: "Maximum 64 characters", Es: "Máximo 64 caracteres", Zh: "最多 64 个字符", De: "Maximal 64 Zeichen", Fr: "64 caractères maximum"}
	hintBadStart    = i18n.Msg{En: "Must start with a letter or a digit", Es: "Debe empezar por una letra o un dígito", Zh: "必须以字母或数字开头", De: "Muss mit einem Buchstaben oder einer Ziffer beginnen", Fr: "Doit commencer par une lettre ou un chiffre"}
	hintBadChars    = i18n.Msg{En: "Only letters, digits, “.”, “_” and “-”", Es: "Solo letras, dígitos, «.», «_» y «-»", Zh: "仅限字母、数字、“.”、“_”和“-”", De: "Nur Buchstaben, Ziffern, „.“, „_“ und „-“", Fr: "Uniquement des lettres, chiffres, « . », « _ » et « - »"}
	hintReserved    = i18n.Msg{En: "Reserved name", Es: "Nombre reservado", Zh: "保留名称", De: "Reservierter Name", Fr: "Nom réservé"}
)

// Labels and titles of the profile dialogs, shared by the handlers that open them and the replies
// that refresh them (dialogs.rs, profiles_page.rs).
var (
	DialogNewTitle       = i18n.Msg{En: "New profile", Es: "Nuevo perfil", Zh: "新建配置文件", De: "Neues Profil", Fr: "Nouveau profil"}
	DialogDuplicateTitle = i18n.Msg{En: "Duplicate profile", Es: "Duplicar perfil", Zh: "复制配置文件", De: "Profil duplizieren", Fr: "Dupliquer le profil"}
	DialogRenameTitle    = i18n.Msg{En: "Rename profile", Es: "Renombrar perfil", Zh: "重命名配置文件", De: "Profil umbenennen", Fr: "Renommer le profil"}
	// CopySuffix is the name a duplicate starts with: the source name plus this suffix.
	CopySuffix = i18n.Msg{En: "{name}-copy", Es: "{name}-copia", Zh: "{name}-副本", De: "{name}-kopie", Fr: "{name}-copie"}
	// AcceptLabel is the submit button of the name dialogs.
	AcceptLabel = i18n.Msg{En: "OK", Es: "Aceptar", Zh: "确定", De: "OK", Fr: "OK"}
	// CreateRoleLabel is the submit button of the role dialog.
	CreateRoleLabel = i18n.Msg{En: "Create role", Es: "Crear rol", Zh: "创建角色", De: "Rolle erstellen", Fr: "Créer le rôle"}
)
