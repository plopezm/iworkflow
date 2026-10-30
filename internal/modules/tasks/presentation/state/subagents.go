package state

import (
	"net/url"
	"sort"

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
	subagents = workingFirst(subagents)
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

// workingFirst lists the subagents still working before the finished ones, keeping the given order inside each group,
// so what needs attention is on top. The list it receives is left untouched.
func workingFirst(subagents []dto.SubagentDTO) []dto.SubagentDTO {
	ordered := append([]dto.SubagentDTO(nil), subagents...)
	sort.SliceStable(ordered, func(i int, j int) bool {
		return isWorking(ordered[i].Status) && !isWorking(ordered[j].Status)
	})
	return ordered
}

// isWorking reports whether a subagent status is one of a subagent that has not finished.
func isWorking(status string) bool {
	return status == "running" || status == "queued" || status == "waiting"
}

// SubagentsPath is the address of the subagents panel of a task, with the page scope and the selected subagent when
// they are set.
func SubagentsPath(taskID string, scope string, selected string) string {
	return subagentsPath(taskID, scope, selected, false)
}

// SubagentsBodyPath is the address of the content of the subagents dialog alone: the list or the selected detail. It is
// what the dialog asks for when it refreshes or navigates, so that it never reopens a dialog the user closed.
func SubagentsBodyPath(taskID string, scope string, selected string) string {
	return subagentsPath(taskID, scope, selected, true)
}

func subagentsPath(taskID string, scope string, selected string, body bool) string {
	query := url.Values{}
	if body {
		query.Set("body", "1")
	}
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
