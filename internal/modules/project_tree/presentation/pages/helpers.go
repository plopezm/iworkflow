package pages

import (
	"context"

	"iworkflow/internal/modules/project_tree/presentation/state"
)

// rootTitle is the heading of the root tab: the workspace name, or "Project" when the root could not be
// listed.
func rootTitle(ctx context.Context, v state.TreePage) string {
	if v.View.Error != "" || v.View.WorkspaceName == "" {
		return msgProject.T(ctx)
	}
	return v.View.WorkspaceName
}
