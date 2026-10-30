package components

import "iworkflow/internal/modules/project_tree/presentation/state"

// GitChangesPanelProps configures the changes panel. The panel is reusable: any screen with a scope (a
// workspace, or a task worktree) renders it from a view the module builds.
//
//   - View.Scope: the workspace or task worktree whose changes are listed and acted on.
//   - View.Groups: the changes split into staged, unstaged and untracked.
//   - View.LoadError: replaces the list with a message when git could not be read.
//   - View.ActionError: the message of the last stage, unstage or commit that failed.
type GitChangesPanelProps struct {
	View state.GitView
}
