// Package components renders the fragments of the group screens.
package components

import "iworkflow/internal/core/i18n"

var (
	msgEmpty      = i18n.Msg{En: "No groups yet. Create the first one above.", Es: "Aún no hay grupos. Crea el primero arriba.", Zh: "还没有分组。请先在上方创建。", De: "Noch keine Gruppen. Erstelle oben die erste.", Fr: "Aucun groupe pour l'instant. Créez le premier ci-dessus."}
	msgWorkspaces = i18n.Msg{En: "workspaces", Es: "workspaces", Zh: "个工作区", De: "Workspaces", Fr: "workspaces"}
	msgOpenGroup  = i18n.Msg{En: "Open", Es: "Abrir", Zh: "打开", De: "Öffnen", Fr: "Ouvrir"}
	msgNoMembers  = i18n.Msg{En: "This group has no workspaces. Assign them from each workspace's Edit screen.", Es: "Este grupo no tiene workspaces. Asígnalos desde la pantalla Editar de cada workspace.", Zh: "此分组没有工作区。请在各工作区的“编辑”页面中分配。", De: "Diese Gruppe enthält keine Workspaces. Weise sie auf dem Bearbeiten-Bildschirm des jeweiligen Workspaces zu.", Fr: "Ce groupe ne contient aucun workspace. Affectez-les depuis l'écran Modifier de chaque workspace."}
	msgGone       = i18n.Msg{En: "This group no longer exists.", Es: "Este grupo ya no existe.", Zh: "此分组已不存在。", De: "Diese Gruppe existiert nicht mehr.", Fr: "Ce groupe n'existe plus."}
	msgUnassign   = i18n.Msg{En: "Remove from group", Es: "Quitar del grupo", Zh: "移出分组", De: "Aus Gruppe entfernen", Fr: "Retirer du groupe"}
)
