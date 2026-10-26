package components

import (
	"iworkflow/internal/core/i18n"
)

// Texts of the inline file editor, copied from the Rust editor (src/modules/tasks/presentation/components/diff_file_editor.rs).
var (
	msgFileContent    = i18n.Msg{En: "File content", Es: "Contenido del fichero", Zh: "文件内容", De: "Dateiinhalt", Fr: "Contenu du fichier"}
	msgSaveFile       = i18n.Msg{En: "Save", Es: "Guardar", Zh: "保存", De: "Speichern", Fr: "Enregistrer"}
	msgUnsavedChanges = i18n.Msg{En: "Unsaved changes", Es: "Cambios sin guardar", Zh: "未保存的更改", De: "Nicht gespeicherte Änderungen", Fr: "Modifications non enregistrées"}
	msgSaveHint       = i18n.Msg{En: "Saving creates a commit on the task branch.", Es: "Guardar crea un commit en la rama de la tarea.", Zh: "保存会在任务分支上创建一个提交。", De: "Beim Speichern wird ein Commit auf dem Branch der Aufgabe erstellt.", Fr: "L’enregistrement crée un commit sur la branche de la tâche."}
	msgUnchanged      = i18n.Msg{En: "No changes: the file already had that content, no commit was created.", Es: "Sin cambios: el fichero ya tenía ese contenido, no se creó ningún commit.", Zh: "没有更改：文件已是该内容，未创建任何提交。", De: "Keine Änderungen: Die Datei hatte bereits diesen Inhalt, es wurde kein Commit erstellt.", Fr: "Aucune modification : le fichier avait déjà ce contenu, aucun commit n’a été créé."}
)

// textareaText is the content of a textarea. The HTML parser drops one line break right after the opening tag, so a
// content that starts with one gets an extra break, and the editor shows the same text it loaded.
func textareaText(content string) string {
	if len(content) > 0 && content[0] == '\n' {
		return "\n" + content
	}
	return content
}
