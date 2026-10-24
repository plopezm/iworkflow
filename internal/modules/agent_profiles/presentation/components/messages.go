package components

import "iworkflow/internal/core/i18n"

// User-facing texts of the agent_profiles components. Each one copies the Rust Msg constant of the
// same name in src/modules/agent_profiles/presentation/components/*.rs (or pages/state).
var (
	msgProfilesTitle   = i18n.Msg{En: "Profiles", Es: "Perfiles", Zh: "配置文件", De: "Profile", Fr: "Profils"}
	msgNew             = i18n.Msg{En: "New", Es: "Nuevo", Zh: "新建", De: "Neu", Fr: "Nouveau"}
	msgImport          = i18n.Msg{En: "Import", Es: "Importar", Zh: "导入", De: "Importieren", Fr: "Importer"}
	msgNoProfiles      = i18n.Msg{En: "No profiles yet.", Es: "Aún no hay perfiles.", Zh: "还没有配置文件。", De: "Noch keine Profile.", Fr: "Aucun profil pour le moment."}
	msgActive          = i18n.Msg{En: "Active", Es: "Activo", Zh: "当前", De: "Aktiv", Fr: "Actif"}
	msgAgentRoles      = i18n.Msg{En: "Agent roles", Es: "Roles de agente", Zh: "智能体角色", De: "Agent-Rollen", Fr: "Rôles d'agent"}
	msgNewRole         = i18n.Msg{En: "New role", Es: "Nuevo rol", Zh: "新建角色", De: "Neue Rolle", Fr: "Nouveau rôle"}
	msgNoRoles         = i18n.Msg{En: "No agent roles yet.", Es: "Aún no hay roles de agente.", Zh: "还没有智能体角色。", De: "Noch keine Agent-Rollen.", Fr: "Aucun rôle d'agent pour le moment."}
	msgCreateOrImport  = i18n.Msg{En: "Create or import a profile to get started.", Es: "Crea o importa un perfil para empezar.", Zh: "创建或导入配置文件以开始。", De: "Erstelle oder importiere ein Profil, um zu beginnen.", Fr: "Créez ou importez un profil pour commencer."}
	msgDeleteConfirm   = i18n.Msg{En: "Delete profile “{name}”?", Es: "¿Eliminar el perfil «{name}»?", Zh: "删除配置文件“{name}”？", De: "Profil „{name}“ löschen?", Fr: "Supprimer le profil « {name} » ?"}
	msgDuplicate       = i18n.Msg{En: "Duplicate", Es: "Duplicar", Zh: "复制", De: "Duplizieren", Fr: "Dupliquer"}
	msgRename          = i18n.Msg{En: "Rename", Es: "Renombrar", Zh: "重命名", De: "Umbenennen", Fr: "Renommer"}
	msgExport          = i18n.Msg{En: "Export", Es: "Exportar", Zh: "导出", De: "Exportieren", Fr: "Exporter"}
	msgCopyJSON        = i18n.Msg{En: "Copy JSON", Es: "Copiar JSON", Zh: "复制 JSON", De: "JSON kopieren", Fr: "Copier le JSON"}
	msgDelete          = i18n.Msg{En: "Delete", Es: "Eliminar", Zh: "删除", De: "Löschen", Fr: "Supprimer"}
	msgIsActive        = i18n.Msg{En: "This is the active profile: it cannot be deleted.", Es: "Es el perfil activo: no se puede eliminar.", Zh: "这是当前配置文件：无法删除。", De: "Dies ist das aktive Profil: Es kann nicht gelöscht werden.", Fr: "C'est le profil actif : il ne peut pas être supprimé."}
	msgInherit         = i18n.Msg{En: "Inherit / default", Es: "Heredar / por defecto", Zh: "继承 / 默认", De: "Erben / Standard", Fr: "Hériter / par défaut"}
	msgRemove          = i18n.Msg{En: "Remove", Es: "Quitar", Zh: "移除", De: "Entfernen", Fr: "Retirer"}
	msgModel           = i18n.Msg{En: "Model", Es: "Modelo", Zh: "模型", De: "Modell", Fr: "Modèle"}
	msgReasoning       = i18n.Msg{En: "Reasoning", Es: "Razonamiento", Zh: "推理", De: "Reasoning", Fr: "Raisonnement"}
	msgAgentToAdd      = i18n.Msg{En: "Agent to add", Es: "Agente a añadir", Zh: "要添加的智能体", De: "Hinzuzufügender Agent", Fr: "Agent à ajouter"}
	msgAddAgent        = i18n.Msg{En: "Add agent…", Es: "Añadir agente…", Zh: "添加智能体…", De: "Agent hinzufügen…", Fr: "Ajouter un agent…"}
	msgAddRow          = i18n.Msg{En: "Add row", Es: "Añadir fila", Zh: "添加行", De: "Zeile hinzufügen", Fr: "Ajouter une ligne"}
	msgSaveRouting     = i18n.Msg{En: "Save routing", Es: "Guardar enrutado", Zh: "保存路由", De: "Routing speichern", Fr: "Enregistrer le routage"}
	msgCancel          = i18n.Msg{En: "Cancel", Es: "Cancelar", Zh: "取消", De: "Abbrechen", Fr: "Annuler"}
	msgProfileName     = i18n.Msg{En: "Profile name", Es: "Nombre del perfil", Zh: "配置文件名称", De: "Profilname", Fr: "Nom du profil"}
	msgNewRoleTitle    = i18n.Msg{En: "New agent role", Es: "Nuevo rol de agente", Zh: "新建智能体角色", De: "Neue Agent-Rolle", Fr: "Nouveau rôle d'agent"}
	msgRoleName        = i18n.Msg{En: "Name (slug)", Es: "Nombre (slug)", Zh: "名称（slug）", De: "Name (Slug)", Fr: "Nom (slug)"}
	msgRoleDescription = i18n.Msg{En: "Description", Es: "Descripción", Zh: "描述", De: "Beschreibung", Fr: "Description"}
	msgRoleDescHint    = i18n.Msg{En: "What this agent does", Es: "Qué hace este agente", Zh: "此智能体的作用", De: "Was dieser Agent tut", Fr: "Ce que fait cet agent"}
	msgRolePromptHint  = i18n.Msg{En: "Agent instructions", Es: "Instrucciones del agente", Zh: "智能体指令", De: "Anweisungen für den Agent", Fr: "Instructions de l'agent"}
	msgImportTitle     = i18n.Msg{En: "Import profile", Es: "Importar perfil", Zh: "导入配置文件", De: "Profil importieren", Fr: "Importer un profil"}
	msgImportFile      = i18n.Msg{En: ".profile.json file", Es: "Fichero .profile.json", Zh: ".profile.json 文件", De: "Datei .profile.json", Fr: "Fichier .profile.json"}
	msgPasteJSON       = i18n.Msg{En: "Or paste the JSON", Es: "O pega el JSON", Zh: "或粘贴 JSON", De: "Oder JSON einfügen", Fr: "Ou collez le JSON"}
	msgRenameLabel     = i18n.Msg{En: "New name (optional)", Es: "Nuevo nombre (opcional)", Zh: "新名称（可选）", De: "Neuer Name (optional)", Fr: "Nouveau nom (facultatif)"}
	msgRenameHint      = i18n.Msg{En: "Leave empty to keep the original", Es: "Dejar vacío para conservar el original", Zh: "留空则保留原名称", De: "Leer lassen, um das Original zu behalten", Fr: "Laissez vide pour conserver l'original"}
	msgImportAction    = i18n.Msg{En: "Import", Es: "Importar", Zh: "导入", De: "Importieren", Fr: "Importer"}
)
