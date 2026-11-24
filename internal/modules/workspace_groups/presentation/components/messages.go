// Package components renders the group management card.
package components

import "iworkflow/internal/core/i18n"

var (
	msgTitle        = i18n.Msg{En: "Groups", Es: "Grupos", Zh: "分组", De: "Gruppen", Fr: "Groupes"}
	msgNewName      = i18n.Msg{En: "New group name", Es: "Nombre del nuevo grupo", Zh: "新分组名称", De: "Name der neuen Gruppe", Fr: "Nom du nouveau groupe"}
	msgCreate       = i18n.Msg{En: "Create group", Es: "Crear grupo", Zh: "创建分组", De: "Gruppe erstellen", Fr: "Créer le groupe"}
	msgEmpty        = i18n.Msg{En: "No groups yet. Create the first one above.", Es: "Aún no hay grupos. Crea el primero arriba.", Zh: "还没有分组。请先在上方创建。", De: "Noch keine Gruppen. Erstelle oben die erste.", Fr: "Aucun groupe pour l'instant. Créez le premier ci-dessus."}
	msgGroupName    = i18n.Msg{En: "Group name", Es: "Nombre del grupo", Zh: "分组名称", De: "Gruppenname", Fr: "Nom du groupe"}
	msgColor        = i18n.Msg{En: "Color", Es: "Color", Zh: "颜色", De: "Farbe", Fr: "Couleur"}
	msgNoColor      = i18n.Msg{En: "No color", Es: "Sin color", Zh: "无颜色", De: "Keine Farbe", Fr: "Sans couleur"}
	msgAssignments  = i18n.Msg{En: "Assignments", Es: "Asignaciones", Zh: "分配", De: "Zuweisungen", Fr: "Affectations"}
	msgDelete       = i18n.Msg{En: "Delete group", Es: "Eliminar grupo", Zh: "删除分组", De: "Gruppe löschen", Fr: "Supprimer le groupe"}
	msgConfirmDel   = i18n.Msg{En: "Delete this group? Its workspaces are kept and become ungrouped.", Es: "¿Eliminar este grupo? Sus workspaces se conservan y quedan sin grupo.", Zh: "删除此分组？其工作区会保留并变为未分组。", De: "Diese Gruppe löschen? Die Workspaces bleiben erhalten und sind danach ohne Gruppe.", Fr: "Supprimer ce groupe ? Ses workspaces sont conservés et n'ont plus de groupe."}
	msgInGroup      = i18n.Msg{En: "Assigned workspaces", Es: "Workspaces asignados", Zh: "已分配的工作区", De: "Zugewiesene Workspaces", Fr: "Workspaces affectés"}
	msgNoMembers    = i18n.Msg{En: "No workspaces in this group.", Es: "Este grupo no tiene workspaces.", Zh: "此分组没有工作区。", De: "Diese Gruppe enthält keine Workspaces.", Fr: "Ce groupe ne contient aucun workspace."}
	msgUnassign     = i18n.Msg{En: "Unassign", Es: "Desasignar", Zh: "取消分配", De: "Entfernen", Fr: "Retirer"}
	msgAvailable    = i18n.Msg{En: "Available workspaces", Es: "Workspaces disponibles", Zh: "可分配的工作区", De: "Verfügbare Workspaces", Fr: "Workspaces disponibles"}
	msgNoAvailable  = i18n.Msg{En: "Every workspace is already in this group.", Es: "Todos los workspaces ya están en este grupo.", Zh: "所有工作区都已在此分组中。", De: "Alle Workspaces sind bereits in dieser Gruppe.", Fr: "Tous les workspaces sont déjà dans ce groupe."}
	msgAssign       = i18n.Msg{En: "Assign", Es: "Asignar", Zh: "分配", De: "Zuweisen", Fr: "Affecter"}
	msgCurrentGroup = i18n.Msg{En: "now in {group}", Es: "ahora en {group}", Zh: "目前在 {group}", De: "jetzt in {group}", Fr: "actuellement dans {group}"}
)
