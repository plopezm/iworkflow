// Package pages holds the templ pages of the memories module.
package pages

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// User-facing texts of the memories pages.
var (
	msgMemoriesTitle = i18n.Msg{En: "Memories", Es: "Memorias", Zh: "记忆", De: "Erinnerungen", Fr: "Mémoires"}
	msgReadOnly      = i18n.Msg{En: "Read only", Es: "Solo lectura", Zh: "只读", De: "Nur Lesen", Fr: "Lecture seule"}
	msgBackToBoard   = i18n.Msg{En: "Open board", Es: "Abrir tablero", Zh: "打开看板", De: "Board öffnen", Fr: "Ouvrir le tableau"}
	msgBackToList    = i18n.Msg{En: "All memories", Es: "Todas las memorias", Zh: "全部记忆", De: "Alle Erinnerungen", Fr: "Toutes les mémoires"}
	msgUnavailable   = i18n.Msg{En: "The memory system is not available. Check that Engram is running and IWF_ENGRAM_URL is set.", Es: "El sistema de memoria no está disponible. Comprueba que Engram está en marcha y que IWF_ENGRAM_URL está definido.", Zh: "记忆系统不可用。请确认 Engram 正在运行且已设置 IWF_ENGRAM_URL。", De: "Das Gedächtnissystem ist nicht verfügbar. Prüfe, ob Engram läuft und IWF_ENGRAM_URL gesetzt ist.", Fr: "Le système de mémoire n'est pas disponible. Vérifiez qu'Engram tourne et que IWF_ENGRAM_URL est défini."}
	msgEmpty         = i18n.Msg{En: "This workspace has no memories yet.", Es: "Este workspace aún no tiene memorias.", Zh: "此工作区还没有记忆。", De: "Dieser Workspace hat noch keine Erinnerungen.", Fr: "Cet espace de travail n'a pas encore de mémoires."}
)

var (
	msgBranchGroup  = i18n.Msg{En: "From this branch", Es: "De esta rama", Zh: "来自此分支", De: "Von diesem Branch", Fr: "De cette branche"}
	msgBranchHelp   = i18n.Msg{En: "Written since the task started.", Es: "Escritas desde que empezó la tarea.", Zh: "自任务开始以来写入。", De: "Seit dem Start der Aufgabe geschrieben.", Fr: "Écrites depuis le début de la tâche."}
	msgEarlierGroup = i18n.Msg{En: "Earlier", Es: "Anteriores", Zh: "更早", De: "Früher", Fr: "Antérieures"}
	msgEarlierHelp  = i18n.Msg{En: "Already in the workspace when the task started.", Es: "Ya estaban en el workspace cuando empezó la tarea.", Zh: "任务开始时工作区中已有。", De: "Waren beim Start der Aufgabe schon im Workspace.", Fr: "Déjà dans l'espace de travail au début de la tâche."}
	msgNoBranchYet  = i18n.Msg{En: "No memories written on this branch yet.", Es: "Aún no hay memorias escritas en esta rama.", Zh: "此分支尚无记忆。", De: "Auf diesem Branch wurden noch keine Erinnerungen geschrieben.", Fr: "Aucune mémoire écrite sur cette branche pour l'instant."}
	msgBackToTask   = i18n.Msg{En: "Open task", Es: "Abrir tarea", Zh: "打开任务", De: "Aufgabe öffnen", Fr: "Ouvrir la tâche"}
	msgAllWorkspace = i18n.Msg{En: "All workspace memories", Es: "Todas las del workspace", Zh: "工作区全部记忆", De: "Alle des Workspace", Fr: "Toutes celles de l'espace de travail"}
)

// ListTitle is the default title of the memories tab.
func ListTitle(ctx context.Context) string { return msgMemoriesTitle.T(ctx) }
