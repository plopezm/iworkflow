package components

import (
	"context"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Names of the statuses, copied from the Rust badge (src/modules/tasks/presentation/components/diff_badges.rs).
var (
	msgStatusAdded    = i18n.Msg{En: "Added", Es: "Añadido", Zh: "已添加", De: "Hinzugefügt", Fr: "Ajouté"}
	msgStatusModified = i18n.Msg{En: "Modified", Es: "Modificado", Zh: "已修改", De: "Geändert", Fr: "Modifié"}
	msgStatusDeleted  = i18n.Msg{En: "Deleted", Es: "Eliminado", Zh: "已删除", De: "Gelöscht", Fr: "Supprimé"}
	msgStatusRenamed  = i18n.Msg{En: "Renamed", Es: "Renombrado", Zh: "已重命名", De: "Umbenannt", Fr: "Renommé"}
)

// statusStyle is how a status shows in its badge: the CSS class of the badge and the name in its tooltip.
type statusStyle struct {
	class string
	name  i18n.Msg
}

// styleOf is the badge of a status. Rust matches the four statuses one by one; the modified style also covers any
// other value, so a file is never shown without a badge.
func styleOf(status state.FileStatus) statusStyle {
	switch status {
	case state.FileStatusAdded:
		return statusStyle{class: "diff-status diff-status-added", name: msgStatusAdded}
	case state.FileStatusDeleted:
		return statusStyle{class: "diff-status diff-status-deleted", name: msgStatusDeleted}
	case state.FileStatusRenamed:
		return statusStyle{class: "diff-status diff-status-renamed", name: msgStatusRenamed}
	default:
		return statusStyle{class: "diff-status diff-status-modified", name: msgStatusModified}
	}
}

// statusTitle is the name of a status in the language of the request, shown as the tooltip of its badge.
func statusTitle(ctx context.Context, status state.FileStatus) string {
	return styleOf(status).name.T(ctx)
}
