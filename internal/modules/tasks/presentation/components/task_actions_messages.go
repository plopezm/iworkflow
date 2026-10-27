package components

import "iworkflow/internal/core/i18n"

// Texts of the actions tab of the task detail, copied from the Rust task_actions.rs and task_detail_page.rs Msg
// constants. The confirmations are the questions the Rust version asks before a destructive action.
var (
	msgActions         = i18n.Msg{En: "Actions", Es: "Acciones", Zh: "操作", De: "Aktionen", Fr: "Actions"}
	msgSectionAgent    = i18n.Msg{En: "Agent", Es: "Agente", Zh: "智能体", De: "Agent", Fr: "Agent"}
	msgSectionDanger   = i18n.Msg{En: "Danger zone", Es: "Zona de peligro", Zh: "危险区域", De: "Gefahrenzone", Fr: "Zone dangereuse"}
	msgFinishAndReview = i18n.Msg{En: "Finish and review", Es: "Finalizar y revisar", Zh: "完成并审查", De: "Abschließen und überprüfen", Fr: "Terminer et réviser"}
	msgResumeAgent     = i18n.Msg{En: "Resume agent", Es: "Reanudar agente", Zh: "恢复智能体", De: "Agent fortsetzen", Fr: "Reprendre l’agent"}
	msgAbortRun        = i18n.Msg{En: "Abort run", Es: "Abortar ejecución", Zh: "中止运行", De: "Ausführung abbrechen", Fr: "Interrompre l’exécution"}
	msgReset           = i18n.Msg{En: "Reset", Es: "Restablecer", Zh: "重置", De: "Zurücksetzen", Fr: "Réinitialiser"}
	msgDiscard         = i18n.Msg{En: "Discard", Es: "Descartar", Zh: "丢弃", De: "Verwerfen", Fr: "Abandonner"}
	msgConfirmFinish   = i18n.Msg{En: "Finish the task? The agent will be closed and the changes will be consolidated for review.", Es: "¿Finalizar la tarea? Se cerrará el agente y se consolidarán los cambios para revisión.", Zh: "要完成该任务吗？智能体将被关闭，更改将合并以供审查。", De: "Aufgabe abschließen? Der Agent wird beendet und die Änderungen werden zur Überprüfung zusammengeführt.", Fr: "Terminer la tâche ? L’agent sera fermé et les modifications seront consolidées pour révision."}
	msgConfirmReset    = i18n.Msg{En: "Reset the task? The agent will be stopped, the worktree, the branch and the history will be deleted, and it will go back to Backlog.", Es: "¿Restablecer la tarea? Se detendrá el agente, se borrarán el worktree, la rama y el historial, y volverá a Backlog.", Zh: "要重置该任务吗？智能体将被停止，worktree、分支和历史记录将被删除，任务将回到 Backlog。", De: "Aufgabe zurücksetzen? Der Agent wird gestoppt, Worktree, Branch und Verlauf werden gelöscht und die Aufgabe kehrt ins Backlog zurück.", Fr: "Réinitialiser la tâche ? L’agent sera arrêté, le worktree, la branche et l’historique seront supprimés, et la tâche retournera dans le Backlog."}
	msgConfirmDiscard  = i18n.Msg{En: "Discard the task? The worktree and the unmerged branch will be deleted.", Es: "¿Descartar la tarea? Se borrarán el worktree y la rama sin integrar.", Zh: "要丢弃该任务吗？worktree 和未合并的分支将被删除。", De: "Aufgabe verwerfen? Der Worktree und der nicht integrierte Branch werden gelöscht.", Fr: "Abandonner la tâche ? Le worktree et la branche non intégrée seront supprimés."}
	msgSectionReview   = i18n.Msg{En: "Review", Es: "Revisión", Zh: "审查", De: "Überprüfung", Fr: "Révision"}
	msgMerge           = i18n.Msg{En: "Merge", Es: "Merge", Zh: "合并", De: "Mergen", Fr: "Fusionner"}
	msgConfirmMerge    = i18n.Msg{En: "Merge the branch into the base? The worktree will be removed.", Es: "¿Integrar la rama en la base? Se eliminará el worktree.", Zh: "要将分支合并到基础分支吗？worktree 将被删除。", De: "Den Branch in die Basis integrieren? Der Worktree wird gelöscht.", Fr: "Intégrer la branche dans la base ? Le worktree sera supprimé."}
	msgMerged          = i18n.Msg{En: "Merged into the base branch.", Es: "Integrada en la rama base.", Zh: "已合并到基础分支。", De: "In den Basis-Branch integriert.", Fr: "Intégrée dans la branche de base."}
	msgMergeConflicts  = i18n.Msg{En: "Conflicts in {files}. Resume the agent to resolve them or do it by hand in the worktree.", Es: "Conflictos en {files}. Reanuda el agente para resolverlos o hazlo a mano en el worktree.", Zh: "{files} 中存在冲突。请恢复智能体来解决，或在 worktree 中手动解决。", De: "Konflikte in {files}. Setze den Agent fort, um sie zu lösen, oder löse sie manuell im Worktree.", Fr: "Conflits dans {files}. Relancez l'agent pour les résoudre ou faites-le à la main dans le worktree."}
	msgInitialPrompt   = i18n.Msg{En: "Initial prompt", Es: "Prompt inicial", Zh: "初始提示词", De: "Ursprünglicher Prompt", Fr: "Prompt initial"}
	msgStartingBranch  = i18n.Msg{En: "Starting branch", Es: "Rama de partida", Zh: "起始分支", De: "Ausgangs-Branch", Fr: "Branche de départ"}
	msgAttachments     = i18n.Msg{En: "Attachments", Es: "Adjuntos", Zh: "附件", De: "Anhänge", Fr: "Pièces jointes"}
	msgTaskImages      = i18n.Msg{En: "Images attached to the task", Es: "Imágenes adjuntas de la tarea", Zh: "任务的附加图片", De: "Der Aufgabe angehängte Bilder", Fr: "Images jointes à la tâche"}
)
