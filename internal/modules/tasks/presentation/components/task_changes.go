package components

import (
	"strconv"
	"strings"

	"context"

	"github.com/a-h/templ"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/urlpath"
	"iworkflow/internal/modules/tasks/presentation/state"
)

var (
	msgChangesUncommitted = i18n.Msg{En: "Uncommitted", Es: "Sin commitear", Zh: "未提交", De: "Nicht committet", Fr: "Non commitées"}
	msgChangesCommitted   = i18n.Msg{En: "Committed", Es: "Commiteados", Zh: "已提交", De: "Committet", Fr: "Commitées"}
	msgCommittedEmpty     = i18n.Msg{En: "No commits compared to the base branch.", Es: "Sin commits respecto a la rama base.", Zh: "与基础分支相比没有提交。", De: "Keine Commits gegenüber dem Basis-Branch.", Fr: "Aucun commit par rapport à la branche de base."}
	msgCommittedCommits   = i18n.Msg{En: "Commits ({n})", Es: "Commits ({n})", Zh: "提交（{n}）", De: "Commits ({n})", Fr: "Commits ({n})"}
	msgCommittedFiles     = i18n.Msg{En: "Files ({n})", Es: "Ficheros ({n})", Zh: "文件（{n}）", De: "Dateien ({n})", Fr: "Fichiers ({n})"}
)

// changesSubItems are the sub-tabs of the Changes tab. Each one asks for the region with itself active, and the
// answer swaps the region in place.
func changesSubItems(ctx context.Context, changes state.TaskChanges) []coreui.TabItem {
	labels := []struct {
		id    string
		label string
	}{
		{state.ChangesSubUncommitted, msgChangesUncommitted.T(ctx)},
		{state.ChangesSubCommitted, msgChangesCommitted.T(ctx)},
	}
	items := make([]coreui.TabItem, 0, len(labels))
	for _, sub := range labels {
		items = append(items, coreui.TabItem{
			ID:    sub.id,
			Label: sub.label,
			Attrs: templ.Attributes{
				"hx-get":    changes.Path(sub.id),
				"hx-target": "#" + changes.RegionID(),
				"hx-swap":   "outerHTML",
			},
		})
	}
	return items
}

func committedFilesTitle(ctx context.Context, count int) string {
	return msgCommittedFiles.Fmt(ctx, map[string]string{"n": countText(count)})
}

func committedCommitsTitle(ctx context.Context, count int) string {
	return msgCommittedCommits.Fmt(ctx, map[string]string{"n": countText(count)})
}

func committedFileHref(taskID string, path string) string {
	return "/tasks/" + taskID + "/files/" + urlpath.EncodePath(path)
}

func committedCounts(file state.FileDiff) string {
	if file.Binary {
		return "bin"
	}
	return "+" + strconv.Itoa(file.Additions) + " −" + strconv.Itoa(file.Deletions)
}

func committedFileName(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[index+1:]
	}
	return path
}

func committedParentDir(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[:index]
	}
	return ""
}
