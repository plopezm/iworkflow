package pages

import (
	"context"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspaces/presentation/state"
)

// User-facing texts of the workspaces pages. Each one copies the Rust `Msg` constant it replaces.
var (
	msgWorkspacesTitle    = i18n.Msg{En: "Workspaces", Es: "Workspaces", Zh: "工作区", De: "Workspaces", Fr: "Espaces de travail"}
	msgRegisterTitle      = i18n.Msg{En: "Register workspace", Es: "Registrar workspace", Zh: "注册工作区", De: "Workspace registrieren", Fr: "Enregistrer un espace de travail"}
	msgRegisteredTitle    = i18n.Msg{En: "Registered", Es: "Registrados", Zh: "已注册", De: "Registriert", Fr: "Enregistrés"}
	msgGroupsLink         = i18n.Msg{En: "Groups", Es: "Grupos", Zh: "分组", De: "Gruppen", Fr: "Groupes"}
	msgGroupLabel         = i18n.Msg{En: "Group", Es: "Grupo", Zh: "分组", De: "Gruppe", Fr: "Groupe"}
	msgLoadingGroupsLabel = i18n.Msg{En: "Loading groups…", Es: "Cargando grupos…", Zh: "正在加载分组…", De: "Gruppen werden geladen …", Fr: "Chargement des groupes…"}
	msgNoGroupLabel       = i18n.Msg{En: "No group", Es: "Sin grupo", Zh: "未分组", De: "Keine Gruppe", Fr: "Aucun groupe"}
	msgNewWorkspace       = i18n.Msg{En: "New workspace", Es: "Nuevo workspace", Zh: "新建工作区", De: "Neuer Workspace", Fr: "Nouvel espace de travail"}
	msgEditTitle          = i18n.Msg{En: "Edit workspace", Es: "Editar workspace", Zh: "编辑工作区", De: "Workspace bearbeiten", Fr: "Modifier l'espace de travail"}
	msgNameLabel          = i18n.Msg{En: "Name", Es: "Nombre", Zh: "名称", De: "Name", Fr: "Nom"}
	msgNamePlaceholder    = i18n.Msg{En: "my-project", Es: "mi-proyecto", Zh: "我的项目", De: "mein-projekt", Fr: "mon-projet"}
	msgPathLabel          = i18n.Msg{En: "Path", Es: "Ruta", Zh: "路径", De: "Pfad", Fr: "Chemin"}
	msgBaseBranchLabel    = i18n.Msg{En: "Base branch", Es: "Rama base", Zh: "基础分支", De: "Basis-Branch", Fr: "Branche de base"}
	msgReplaceAgentsLabel = i18n.Msg{En: "Replace AGENTS.md (optional)", Es: "Sustituir AGENTS.md (opcional)", Zh: "替换 AGENTS.md（可选）", De: "AGENTS.md ersetzen (optional)", Fr: "Remplacer AGENTS.md (facultatif)"}
	msgKeepCurrentLabel   = i18n.Msg{En: "Keep the current one", Es: "Mantener el actual", Zh: "保留当前文件", De: "Aktuelle beibehalten", Fr: "Conserver l'actuel"}
	msgDefaultAgentLabel  = i18n.Msg{En: "Default agent", Es: "Agente por defecto", Zh: "默认智能体", De: "Standard-Agent", Fr: "Agent par défaut"}
	msgSystemAgentLabel   = i18n.Msg{En: "System agent", Es: "Agente del sistema", Zh: "系统智能体", De: "System-Agent", Fr: "Agent du système"}
	msgColorLabel         = i18n.Msg{En: "Color", Es: "Color", Zh: "颜色", De: "Farbe", Fr: "Couleur"}
	msgNoColorLabel       = i18n.Msg{En: "No color", Es: "Sin color", Zh: "无颜色", De: "Keine Farbe", Fr: "Aucune couleur"}
	msgCancelLabel        = i18n.Msg{En: "Cancel", Es: "Cancelar", Zh: "取消", De: "Abbrechen", Fr: "Annuler"}
	msgSaveChangesLabel   = i18n.Msg{En: "Save changes", Es: "Guardar cambios", Zh: "保存更改", De: "Änderungen speichern", Fr: "Enregistrer les modifications"}

	// The reasons of the path rules, one per domain rule (see state.Rule*). {path} and {detail} are filled in.
	msgPathBlocked            = i18n.Msg{En: "The path cannot change while the workspace has unfinished tasks.", Es: "La ruta no puede cambiar mientras el workspace tenga tareas sin terminar.", Zh: "工作区有未完成的任务时，路径不能更改。", De: "Der Pfad kann nicht geändert werden, solange der Workspace unerledigte Aufgaben hat.", Fr: "Le chemin ne peut pas changer tant que l'espace de travail a des tâches inachevées."}
	msgPathNotAbsolute        = i18n.Msg{En: "The path must be absolute: {path}", Es: "La ruta debe ser absoluta: {path}", Zh: "路径必须是绝对路径：{path}", De: "Der Pfad muss absolut sein: {path}", Fr: "Le chemin doit être absolu : {path}"}
	msgPathNotFound           = i18n.Msg{En: "The folder does not exist: {path}", Es: "La carpeta no existe: {path}", Zh: "文件夹不存在：{path}", De: "Der Ordner existiert nicht: {path}", Fr: "Le dossier n'existe pas : {path}"}
	msgNotAGitRepoPath        = i18n.Msg{En: "The folder is not a git repository: {path}", Es: "La carpeta no es un repositorio git: {path}", Zh: "该文件夹不是 git 仓库：{path}", De: "Der Ordner ist kein Git-Repository: {path}", Fr: "Le dossier n'est pas un dépôt git : {path}"}
	msgPathNotRepositoryRoot  = i18n.Msg{En: "The new path is not the root of a git repository: {path}", Es: "La nueva ruta no es la raíz de un repositorio git: {path}", Zh: "新路径不是 git 仓库的根目录：{path}", De: "Der neue Pfad ist nicht das Stammverzeichnis eines Git-Repositorys: {path}", Fr: "Le nouveau chemin n'est pas la racine d'un dépôt git : {path}"}
	msgPathNotSameProject     = i18n.Msg{En: "The new path is not the same project as the workspace: {path}", Es: "La nueva ruta no es el mismo proyecto que el workspace: {path}", Zh: "新路径不是该工作区的同一个项目：{path}", De: "Der neue Pfad ist nicht dasselbe Projekt wie der Workspace: {path}", Fr: "Le nouveau chemin n'est pas le même projet que l'espace de travail : {path}"}
	msgPathAlreadyRegistered  = i18n.Msg{En: "The path is already registered by another workspace: {path}", Es: "La ruta ya está registrada por otro workspace: {path}", Zh: "该路径已被其他工作区注册：{path}", De: "Der Pfad ist bereits von einem anderen Workspace registriert: {path}", Fr: "Le chemin est déjà enregistré par un autre espace de travail : {path}"}
	msgSameProjectUnconfirmed = i18n.Msg{En: "The previous folder no longer exists, so the new folder cannot be compared; confirm it is the same project", Es: "La carpeta anterior ya no existe, así que la nueva carpeta no se puede comparar; confirma que es el mismo proyecto", Zh: "原文件夹已不存在，无法比较新文件夹；请确认这是同一个项目", De: "Der bisherige Ordner existiert nicht mehr, daher kann der neue Ordner nicht verglichen werden; bestätige, dass es dasselbe Projekt ist", Fr: "L'ancien dossier n'existe plus, le nouveau dossier ne peut donc pas être comparé ; confirmez qu'il s'agit du même projet"}
	msgWorktreeRepair         = i18n.Msg{En: "The path was saved, but git could not repair the worktrees: {detail}", Es: "La ruta se guardó, pero git no pudo reparar los worktrees: {detail}", Zh: "路径已保存，但 git 无法修复工作树：{detail}", De: "Der Pfad wurde gespeichert, aber Git konnte die Worktrees nicht reparieren: {detail}", Fr: "Le chemin a été enregistré, mais git n'a pas pu réparer les worktrees : {detail}"}
	msgConfirmSameProject     = i18n.Msg{En: "I confirm it is the same project", Es: "Confirmo que es el mismo proyecto", Zh: "我确认这是同一个项目", De: "Ich bestätige, dass es dasselbe Projekt ist", Fr: "Je confirme qu'il s'agit du même projet"}
)

// editFormOptions converts the AGENTS.md options into the shared select options.
func editFormOptions(options []state.AgentsOption) []shared.FormOption {
	converted := make([]shared.FormOption, 0, len(options))
	for _, option := range options {
		converted = append(converted, shared.FormOption{Value: option.Value, Label: option.Label})
	}
	return converted
}

// ListTitle is the default title of the workspaces tab.
func ListTitle(ctx context.Context) string { return msgWorkspacesTitle.T(ctx) }

// NewTitle is the default title of the registration tab.
func NewTitle(ctx context.Context) string { return msgNewWorkspace.T(ctx) }

// EditTitle is the default title of the edit tab.
func EditTitle(ctx context.Context) string { return msgEditTitle.T(ctx) }

// groupSelectOptions converts the groups into the options of the group selector.
func groupSelectOptions(options []state.GroupOption) []shared.SelectOption {
	converted := make([]shared.SelectOption, 0, len(options))
	for _, option := range options {
		converted = append(converted, shared.SelectOption{Value: option.ID, Label: option.Name})
	}
	return converted
}
