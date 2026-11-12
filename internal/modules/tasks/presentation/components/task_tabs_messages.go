package components

import (
	"context"

	"github.com/a-h/templ"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Labels of the detail tabs of a task, copied from the Rust detail page (TITLE_FILES and TITLE_CHANGES in src/app.rs).
var (
	msgTabFiles    = i18n.Msg{En: "Files", Es: "Archivos", Zh: "文件", De: "Dateien", Fr: "Fichiers"}
	msgTabChanges  = i18n.Msg{En: "Changes", Es: "Cambios", Zh: "更改", De: "Änderungen", Fr: "Modifications"}
	msgTabCodeHost = i18n.Msg{En: "PR / MR", Es: "PR / MR", Zh: "PR / MR", De: "PR / MR", Fr: "PR / MR"}
)

// msgTabJira names the Jira tab. The product name is the same in every language.
const msgTabJira = "Jira"

// taskTabItems are the tabs of the strip. Each one asks for the region with itself active, and the answer swaps the
// region in place. The Jira tab is listed only when the strip shows it.
func taskTabItems(ctx context.Context, tabs state.TaskTabs) []coreui.TabItem {
	labels := []struct {
		id    string
		label string
	}{
		{state.TaskTabActions, msgActions.T(ctx)},
		{state.TaskTabFiles, msgTabFiles.T(ctx)},
		{state.TaskTabChanges, msgTabChanges.T(ctx)},
		{state.TaskTabCodeHost, msgTabCodeHost.T(ctx)},
	}
	if tabs.JiraVisible {
		labels = append(labels, struct {
			id    string
			label string
		}{state.TaskTabJira, msgTabJira})
	}
	items := make([]coreui.TabItem, 0, len(labels))
	for _, tab := range labels {
		items = append(items, coreui.TabItem{
			ID:    tab.id,
			Label: tab.label,
			Attrs: templ.Attributes{
				"hx-get":    tabs.Path(tab.id),
				"hx-target": "#" + tabs.RegionID(),
				"hx-swap":   "outerHTML",
			},
		})
	}
	return items
}
