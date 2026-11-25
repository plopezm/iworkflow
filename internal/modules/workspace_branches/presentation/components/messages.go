package components

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// User-facing texts of the branch screen.
var (
	msgBranchesTitle         = i18n.Msg{En: "Branches", Es: "Ramas", Zh: "分支", De: "Branches", Fr: "Branches"}
	msgSelectAll             = i18n.Msg{En: "Select all", Es: "Seleccionar todas", Zh: "全选", De: "Alle auswählen", Fr: "Tout sélectionner"}
	msgSearchBranches        = i18n.Msg{En: "Filter branches…", Es: "Filtrar ramas…", Zh: "筛选分支…", De: "Branches filtern…", Fr: "Filtrer les branches…"}
	msgOpenBoard             = i18n.Msg{En: "Open board", Es: "Abrir tablero", Zh: "打开看板", De: "Board öffnen", Fr: "Ouvrir le tableau"}
	msgLocalBadge            = i18n.Msg{En: "local", Es: "local", Zh: "本地", De: "lokal", Fr: "locale"}
	msgRemoteBadge           = i18n.Msg{En: "remote", Es: "remota", Zh: "远程", De: "remote", Fr: "distante"}
	msgDelete                = i18n.Msg{En: "Delete", Es: "Eliminar", Zh: "删除", De: "Löschen", Fr: "Supprimer"}
	msgDeleteSelected        = i18n.Msg{En: "Delete selected", Es: "Eliminar seleccionadas", Zh: "删除所选", De: "Ausgewählte löschen", Fr: "Supprimer la sélection"}
	msgNoBranches            = i18n.Msg{En: "No branches match.", Es: "No hay ramas que coincidan.", Zh: "没有匹配的分支。", De: "Keine Branches gefunden.", Fr: "Aucune branche ne correspond."}
	msgSelectOne             = i18n.Msg{En: "Select at least one branch.", Es: "Selecciona al menos una rama.", Zh: "请至少选择一个分支。", De: "Wähle mindestens einen Branch aus.", Fr: "Sélectionnez au moins une branche."}
	msgConfirmDeleteLocal    = i18n.Msg{En: "Delete the local branch \"{name}\"? It is removed from this machine.", Es: "¿Eliminar la rama local \"{name}\"? Se borra de este equipo.", Zh: "删除本地分支“{name}”？", De: "Lokalen Branch „{name}“ löschen? Er wird von diesem Rechner entfernt.", Fr: "Supprimer la branche locale « {name} » ? Elle est retirée de cette machine."}
	msgConfirmDeleteRemote   = i18n.Msg{En: "Delete \"{name}\"? It is removed from the remote too, not only from this machine.", Es: "¿Eliminar \"{name}\"? Se borra también del remoto, no solo de este equipo.", Zh: "删除“{name}”？它也会从远程删除。", De: "„{name}“ löschen? Er wird auch vom Remote entfernt, nicht nur von diesem Rechner.", Fr: "Supprimer « {name} » ? Elle est aussi retirée du dépôt distant, pas seulement de cette machine."}
	msgConfirmDeleteSelected = i18n.Msg{En: "Delete the selected branches? Remote ones are removed from the remote too.", Es: "¿Eliminar las ramas seleccionadas? Las remotas se borran también del remoto.", Zh: "删除所选分支？远程分支也会从远程删除。", De: "Ausgewählte Branches löschen? Remote-Branches werden auch vom Remote entfernt.", Fr: "Supprimer les branches sélectionnées ? Les branches distantes sont aussi retirées du dépôt distant."}

	msgReasonCheckedOut = i18n.Msg{En: "Current branch", Es: "Rama actual", Zh: "当前分支", De: "Aktueller Branch", Fr: "Branche actuelle"}
	msgReasonBase       = i18n.Msg{En: "Workspace base branch", Es: "Rama base del workspace", Zh: "工作区基础分支", De: "Basis-Branch des Workspace", Fr: "Branche de base de l'espace de travail"}
	msgReasonWorktree   = i18n.Msg{En: "Checked out in a worktree", Es: "Desplegada en un worktree", Zh: "已在工作树中检出", De: "In einem Worktree ausgecheckt", Fr: "Extraite dans un worktree"}
	msgReasonTask       = i18n.Msg{En: "Used by a live task", Es: "La usa una tarea activa", Zh: "被进行中的任务使用", De: "Von einer laufenden Aufgabe benutzt", Fr: "Utilisée par une tâche en cours"}
	msgReasonRemoteHead = i18n.Msg{En: "Remote default branch", Es: "Rama por defecto del remoto", Zh: "远程默认分支", De: "Standard-Branch des Remotes", Fr: "Branche par défaut du dépôt distant"}

	msgResultDeleted  = i18n.Msg{En: "Deleted: {name}", Es: "Eliminada: {name}", Zh: "已删除：{name}", De: "Gelöscht: {name}", Fr: "Supprimée : {name}"}
	msgResultRefused  = i18n.Msg{En: "{name} was not deleted: {reason}", Es: "No se eliminó {name}: {reason}", Zh: "未删除 {name}：{reason}", De: "{name} wurde nicht gelöscht: {reason}", Fr: "{name} n'a pas été supprimée : {reason}"}
	msgResultInvalid  = i18n.Msg{En: "Invalid branch name: {name}", Es: "Nombre de rama no válido: {name}", Zh: "分支名无效：{name}", De: "Ungültiger Branch-Name: {name}", Fr: "Nom de branche invalide : {name}"}
	msgResultNotFound = i18n.Msg{En: "{name} no longer exists", Es: "{name} ya no existe", Zh: "{name} 已不存在", De: "{name} existiert nicht mehr", Fr: "{name} n'existe plus"}
	msgResultFailed   = i18n.Msg{En: "Could not delete {name}: {error}", Es: "No se pudo eliminar {name}: {error}", Zh: "无法删除 {name}：{error}", De: "{name} konnte nicht gelöscht werden: {error}", Fr: "Impossible de supprimer {name} : {error}"}
)

// reasonText is the message of a protection reason code.
func reasonText(ctx context.Context, reason string) string {
	switch reason {
	case "checked_out":
		return msgReasonCheckedOut.T(ctx)
	case "base_branch":
		return msgReasonBase.T(ctx)
	case "worktree":
		return msgReasonWorktree.T(ctx)
	case "task_in_use":
		return msgReasonTask.T(ctx)
	case "remote_head":
		return msgReasonRemoteHead.T(ctx)
	default:
		return ""
	}
}
