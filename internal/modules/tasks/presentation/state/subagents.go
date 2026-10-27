package state

import (
	"net/url"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// SubagentsView is the panel of the subagents of a task. Selected is the subagent whose detail is open, and Detail is
// that subagent. The selection survives only while the subagent is in the list. Scope is the page instance the panel
// answers to.
type SubagentsView struct {
	Scope     string
	TaskID    string
	Subagents []dto.SubagentDTO
	Selected  string
	Detail    *dto.SubagentDTO
}

// NewSubagentsView builds the panel of a task. A selection that is no longer in the list is dropped.
func NewSubagentsView(scope string, taskID string, subagents []dto.SubagentDTO, selected string) SubagentsView {
	view := SubagentsView{Scope: scope, TaskID: taskID, Subagents: subagents}
	for index := range subagents {
		if subagents[index].ID == selected {
			view.Selected = selected
			view.Detail = &subagents[index]
			break
		}
	}
	return view
}

// SubagentsPath is the address of the subagents panel of a task, with the page scope and the selected subagent when
// they are set.
func SubagentsPath(taskID string, scope string, selected string) string {
	query := url.Values{}
	if scope != "" {
		query.Set("scope", scope)
	}
	if selected != "" {
		query.Set("selected", selected)
	}
	path := TaskPath(taskID) + "/subagents"
	if encoded := query.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}
