package components

import (
	"context"

	"iworkflow/internal/core/i18n"
)

var (
	msgPruneTitle = i18n.Msg{En: "Completed tasks", Es: "Tareas completadas", Zh: "已完成的任务", De: "Abgeschlossene Aufgaben", Fr: "Tâches terminées"}
	msgPruneLabel = i18n.Msg{En: "Delete merged tasks after one week", Es: "Borrar las tareas integradas pasada una semana", Zh: "一周后删除已合并的任务", De: "Integrierte Aufgaben nach einer Woche löschen", Fr: "Supprimer les tâches intégrées après une semaine"}
	msgPruneHelp  = i18n.Msg{En: "A task merged more than seven days ago is removed from the database, with its events and attachments, to save space. Discarded and unfinished tasks are never removed.", Es: "Una tarea integrada hace más de siete días se elimina de la base de datos, con sus eventos y adjuntos, para ahorrar espacio. Las tareas descartadas y las sin terminar nunca se eliminan.", Zh: "合并超过七天的任务会连同其事件和附件从数据库中删除以节省空间。已丢弃和未完成的任务不会被删除。", De: "Eine vor mehr als sieben Tagen integrierte Aufgabe wird samt Ereignissen und Anhängen aus der Datenbank entfernt, um Platz zu sparen. Verworfene und unfertige Aufgaben werden nie entfernt.", Fr: "Une tâche intégrée il y a plus de sept jours est supprimée de la base de données, avec ses événements et ses pièces jointes, pour gagner de la place. Les tâches abandonnées et inachevées ne sont jamais supprimées."}
)

// NewPruneSettingProps builds the props of the prune card. errText is the message of a failed read or save, "" when none.
func NewPruneSettingProps(ctx context.Context, enabled bool, errText string) PruneSettingProps {
	return PruneSettingProps{
		Title:   msgPruneTitle.T(ctx),
		Label:   msgPruneLabel.T(ctx),
		Help:    msgPruneHelp.T(ctx),
		Enabled: enabled,
		Error:   errText,
	}
}
