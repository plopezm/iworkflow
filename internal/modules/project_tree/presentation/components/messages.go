package components

import "iworkflow/internal/core/i18n"

// The texts below copy the Rust Msg constants of the project_tree presentation, in the same five
// languages.

var (
	msgEmptyFolder = i18n.Msg{En: "Empty folder.", Es: "Carpeta vacía.", Zh: "文件夹为空。", De: "Leerer Ordner.", Fr: "Dossier vide."}
	msgReload      = i18n.Msg{En: "Reload", Es: "Recargar", Zh: "重新加载", De: "Neu laden", Fr: "Recharger"}

	msgSave           = i18n.Msg{En: "Save", Es: "Guardar", Zh: "保存", De: "Speichern", Fr: "Enregistrer"}
	msgDiscard        = i18n.Msg{En: "Discard", Es: "Descartar", Zh: "放弃", De: "Verwerfen", Fr: "Abandonner"}
	msgReloadFromDisk = i18n.Msg{En: "Reload from disk", Es: "Recargar desde disco", Zh: "从磁盘重新加载", De: "Von Festplatte neu laden", Fr: "Recharger depuis le disque"}
	msgUnsavedChanges = i18n.Msg{En: "Unsaved changes", Es: "Cambios sin guardar", Zh: "未保存的更改", De: "Nicht gespeicherte Änderungen", Fr: "Modifications non enregistrées"}
	msgFileContent    = i18n.Msg{En: "File content", Es: "Contenido del fichero", Zh: "文件内容", De: "Dateiinhalt", Fr: "Contenu du fichier"}
	msgGitChanges     = i18n.Msg{En: "Changes", Es: "Cambios", Zh: "更改", De: "Änderungen", Fr: "Modifications"}
	msgStaged         = i18n.Msg{En: "Staged", Es: "Con stage", Zh: "已暂存", De: "Bereitgestellt", Fr: "Indexées"}
	msgUntrackedFiles = i18n.Msg{En: "Untracked files", Es: "Archivos sin seguimiento", Zh: "未跟踪的文件", De: "Nicht verfolgte Dateien", Fr: "Fichiers non suivis"}
	msgStageAll       = i18n.Msg{En: "Stage all", Es: "Hacer stage de todo", Zh: "全部暂存", De: "Alles bereitstellen", Fr: "Tout indexer"}
	msgUnstageAll     = i18n.Msg{En: "Unstage all", Es: "Quitar todo del stage", Zh: "全部取消暂存", De: "Alle entfernen", Fr: "Tout désindexer"}
	msgStageFile      = i18n.Msg{En: "Stage", Es: "Hacer stage de", Zh: "暂存", De: "Bereitstellen:", Fr: "Indexer"}
	msgUnstageFile    = i18n.Msg{En: "Unstage", Es: "Quitar del stage", Zh: "取消暂存", De: "Entfernen:", Fr: "Désindexer"}
	msgNoChanges      = i18n.Msg{En: "No changes: the working tree is clean.", Es: "Sin cambios: el árbol de trabajo está limpio.", Zh: "没有更改：工作树是干净的。", De: "Keine Änderungen: Der Arbeitsbaum ist sauber.", Fr: "Aucun changement : l'arbre de travail est propre."}
	msgCommit         = i18n.Msg{En: "Commit", Es: "Commit", Zh: "提交", De: "Commit", Fr: "Commit"}
	msgCommitMessage  = i18n.Msg{En: "Commit message", Es: "Mensaje del commit", Zh: "提交信息", De: "Commit-Nachricht", Fr: "Message de commit"}
)
