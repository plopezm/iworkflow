package components

import (
	"context"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// toggleClass is base, plus extra while on is true.
func toggleClass(base string, on bool, extra string) string {
	if on {
		return base + " " + extra
	}
	return base
}

// priorityOptions are the priorities of the create dialog, in the order the state declares them.
func priorityOptions(ctx context.Context) []shared.FormOption {
	options := make([]shared.FormOption, 0, len(state.PriorityOptions))
	for _, priority := range state.PriorityOptions {
		options = append(options, shared.FormOption{Value: priority, Label: priorityLabel(priority).T(ctx)})
	}
	return options
}

// BoardTitle is the title of a board tab: the board name, followed by the workspace name when one is shown.
func BoardTitle(ctx context.Context, workspaceName string) string {
	if workspaceName == "" {
		return msgBoard.T(ctx)
	}
	return msgBoard.T(ctx) + " · " + workspaceName
}
