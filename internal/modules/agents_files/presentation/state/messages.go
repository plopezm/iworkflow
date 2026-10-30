package state

import "iworkflow/internal/core/i18n"

// User-facing texts of the apply feedback. Each one copies the Rust Msg constant of the same name in
// src/modules/agents_files/presentation/state/agents_files_state.rs.
var (
	msgAddedCommitted = i18n.Msg{
		En: "Added and committed",
		Es: "Añadido y commiteado",
		Zh: "已添加并提交",
		De: "Hinzugefügt und committet",
		Fr: "Ajouté et commité",
	}
	msgAddedNotCommitted = i18n.Msg{
		En: "Added (the commit could not be created; left uncommitted)",
		Es: "Añadido (no se pudo crear el commit; queda sin commitear)",
		Zh: "已添加（无法创建提交；保持未提交状态）",
		De: "Hinzugefügt (der Commit konnte nicht erstellt werden; bleibt uncommittet)",
		Fr: "Ajouté (impossible de créer le commit ; reste non commité)",
	}
	msgTargetExists = i18n.Msg{
		En: "AGENTS.md already exists in that project. Overwrite?",
		Es: "Ya existe AGENTS.md en ese proyecto. ¿Sobrescribir?",
		Zh: "该项目中已存在 AGENTS.md。是否覆盖？",
		De: "AGENTS.md existiert in diesem Projekt bereits. Überschreiben?",
		Fr: "AGENTS.md existe déjà dans ce projet. Écraser ?",
	}
)
