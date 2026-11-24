// Package pages renders the tabs of the group screens.
package pages

import (
	"context"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspace_groups/presentation/state"
)

var (
	msgWorkspaces   = i18n.Msg{En: "Workspaces", Es: "Workspaces", Zh: "工作区", De: "Workspaces", Fr: "Espaces de travail"}
	msgGroups       = i18n.Msg{En: "Groups", Es: "Grupos", Zh: "分组", De: "Gruppen", Fr: "Groupes"}
	msgYouAreHere   = i18n.Msg{En: "You are here", Es: "Estás aquí", Zh: "当前位置", De: "Du bist hier", Fr: "Vous êtes ici"}
	msgNewName      = i18n.Msg{En: "New group name", Es: "Nombre del nuevo grupo", Zh: "新分组名称", De: "Name der neuen Gruppe", Fr: "Nom du nouveau groupe"}
	msgCreate       = i18n.Msg{En: "Create group", Es: "Crear grupo", Zh: "创建分组", De: "Gruppe erstellen", Fr: "Créer le groupe"}
	msgGroupName    = i18n.Msg{En: "Group name", Es: "Nombre del grupo", Zh: "分组名称", De: "Gruppenname", Fr: "Nom du groupe"}
	msgColor        = i18n.Msg{En: "Color", Es: "Color", Zh: "颜色", De: "Farbe", Fr: "Couleur"}
	msgNoColor      = i18n.Msg{En: "No color", Es: "Sin color", Zh: "无颜色", De: "Keine Farbe", Fr: "Sans couleur"}
	msgInGroup      = i18n.Msg{En: "Workspaces in this group", Es: "Workspaces de este grupo", Zh: "此分组的工作区", De: "Workspaces dieser Gruppe", Fr: "Workspaces de ce groupe"}
	msgDelete       = i18n.Msg{En: "Delete group", Es: "Eliminar grupo", Zh: "删除分组", De: "Gruppe löschen", Fr: "Supprimer le groupe"}
	msgConfirmDel   = i18n.Msg{En: "Delete this group? Its workspaces are kept and become ungrouped.", Es: "¿Eliminar este grupo? Sus workspaces se conservan y quedan sin grupo.", Zh: "删除此分组？其工作区会保留并变为未分组。", De: "Diese Gruppe löschen? Die Workspaces bleiben erhalten und sind danach ohne Gruppe.", Fr: "Supprimer ce groupe ? Ses workspaces sont conservés et n'ont plus de groupe."}
	msgNotFoundText = i18n.Msg{En: "This group no longer exists.", Es: "Este grupo ya no existe.", Zh: "此分组已不存在。", De: "Diese Gruppe existiert nicht mehr.", Fr: "Ce groupe n'existe plus."}
)

// ListTitle is the title of the groups tab.
func ListTitle(ctx context.Context) string { return msgGroups.T(ctx) }

// NotFound is the body of the group tab when the group does not exist.
func NotFound(ctx context.Context) string { return msgNotFoundText.T(ctx) }

// crumbs is the trail of a group screen: Workspaces > Groups, and the group's name as the current step when given.
func crumbs(ctx context.Context, groupName string) []shared.Crumb {
	trail := []shared.Crumb{{Label: msgWorkspaces.T(ctx), Href: state.WorkspacesPath}}
	if groupName == "" {
		return append(trail, shared.Crumb{Label: msgGroups.T(ctx)})
	}
	return append(trail, shared.Crumb{Label: msgGroups.T(ctx), Href: state.ListPath}, shared.Crumb{Label: groupName})
}
