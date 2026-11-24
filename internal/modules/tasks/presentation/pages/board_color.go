package pages

import shared "iworkflow/internal/core/components"

// boardColorClass marks the board of a colored workspace: a faint wash and a thin line at the top of the page,
// both drawn from the workspace color. A board without a color gets nothing.
func boardColorClass(color string) string {
	if colorClass := shared.WorkspaceColorClass(color); colorClass != "" {
		return "ws-board " + colorClass
	}
	return ""
}
