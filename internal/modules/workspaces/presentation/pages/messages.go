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
