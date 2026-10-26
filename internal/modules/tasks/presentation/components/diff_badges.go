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

// statusBadgeClass is the CSS class of the badge of a status.
func statusBadgeClass(status state.FileStatus) string {
	switch status {
	case state.FileStatusAdded:
		return "diff-status diff-status-added"
	case state.FileStatusDeleted:
		return "diff-status diff-status-deleted"
	case state.FileStatusRenamed:
		return "diff-status diff-status-renamed"
	default:
		return "diff-status diff-status-modified"
	}
}

// statusTitle is the name of a status in the language of the request, shown as the tooltip of its badge.
func statusTitle(ctx context.Context, status state.FileStatus) string {
	switch status {
	case state.FileStatusAdded:
		return msgStatusAdded.T(ctx)
	case state.FileStatusDeleted:
		return msgStatusDeleted.T(ctx)
	case state.FileStatusRenamed:
		return msgStatusRenamed.T(ctx)
	default:
		return msgStatusModified.T(ctx)
	}
}
