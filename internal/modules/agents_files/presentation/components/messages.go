package components

import "iworkflow/internal/core/i18n"

// User-facing texts of the AGENTS.md components. Each one copies the Rust Msg constant of the same
// name in src/modules/agents_files/presentation/components/*.rs.
var (
	msgUploadFile = i18n.Msg{En: "Upload file", Es: "Subir fichero", Zh: "上传文件", De: "Datei hochladen", Fr: "Envoyer un fichier"}

	msgDescription            = i18n.Msg{En: "Description", Es: "Descripción", Zh: "描述", De: "Beschreibung", Fr: "Description"}
	msgDescriptionPlaceholder = i18n.Msg{En: "Rules for Rust projects", Es: "Reglas para proyectos Rust", Zh: "Rust 项目的规则", De: "Regeln für Rust-Projekte", Fr: "Règles pour les projets Rust"}
	msgFileLabel              = i18n.Msg{En: "AGENTS.md file", Es: "Fichero AGENTS.md", Zh: "AGENTS.md 文件", De: "AGENTS.md-Datei", Fr: "Fichier AGENTS.md"}
	msgUpload                 = i18n.Msg{En: "Upload", Es: "Subir", Zh: "上传", De: "Hochladen", Fr: "Envoyer"}

	msgNoFiles       = i18n.Msg{En: "No AGENTS.md files yet.", Es: "Aún no hay ficheros AGENTS.md.", Zh: "还没有 AGENTS.md 文件。", De: "Noch keine AGENTS.md-Dateien.", Fr: "Aucun fichier AGENTS.md pour le moment."}
	msgConfirmDelete = i18n.Msg{En: "Delete \"{description}\"?", Es: "¿Eliminar \"{description}\"?", Zh: "要删除“{description}”吗？", De: "„{description}“ löschen?", Fr: "Supprimer « {description} » ?"}
	msgDelete        = i18n.Msg{En: "Delete", Es: "Eliminar", Zh: "删除", De: "Löschen", Fr: "Supprimer"}
	msgProject       = i18n.Msg{En: "Project", Es: "Proyecto", Zh: "项目", De: "Projekt", Fr: "Projet"}
	msgPickProject   = i18n.Msg{En: "Choose a project", Es: "Elige un proyecto", Zh: "选择一个项目", De: "Projekt auswählen", Fr: "Choisissez un projet"}
	msgAddToProject  = i18n.Msg{En: "Add to project", Es: "Añadir a proyecto", Zh: "添加到项目", De: "Zum Projekt hinzufügen", Fr: "Ajouter au projet"}
	msgNoWorkspaces  = i18n.Msg{En: "No workspaces registered.", Es: "No hay workspaces registrados.", Zh: "没有已注册的工作区。", De: "Keine Workspaces registriert.", Fr: "Aucun espace de travail enregistré."}
	msgOverwrite     = i18n.Msg{En: "Overwrite", Es: "Sobrescribir", Zh: "覆盖", De: "Überschreiben", Fr: "Écraser"}
	msgClose         = i18n.Msg{En: "Close", Es: "Cerrar", Zh: "关闭", De: "Schließen", Fr: "Fermer"}
)
