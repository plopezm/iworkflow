package dto

import "iworkflow/internal/modules/tasks/domain/models"

// SubagentDTO is a subagent of a task as the subagents panel shows it.
type SubagentDTO struct {
	ID        string
	Agent     string
	Label     *string
	Prompt    *string
	Status    string
	LastStep  *string
	Turns     uint32
	ToolCalls uint32
	Error     *string
	Thread    []SubagentItemDTO
}

// SubagentItemDTO is one item of the thread of a subagent. Kind is text, thinking, note, tool or unknown.
type SubagentItemDTO struct {
	Kind    string
	Text    string
	Name    string
	Args    *string
	Running bool
	IsError bool
	Output  *string
}

// SubagentFromDomain maps a subagent to its view.
func SubagentFromDomain(subagent models.SubagentTask) SubagentDTO {
	thread := make([]SubagentItemDTO, 0, len(subagent.Thread))
	for _, item := range subagent.Thread {
		thread = append(thread, subagentItemFromDomain(item))
	}
	return SubagentDTO{
		ID:        subagent.ID,
		Agent:     subagent.Agent,
		Label:     subagent.Label,
		Prompt:    subagent.Prompt,
		Status:    string(subagent.Status),
		LastStep:  subagent.LastStep,
		Turns:     subagent.Turns,
		ToolCalls: subagent.ToolCalls,
		Error:     subagent.Error,
		Thread:    thread,
	}
}

// subagentItemFromDomain maps one item of the thread; the kinds of the domain are closed, so anything else is unknown.
func subagentItemFromDomain(item models.SubagentThreadItem) SubagentItemDTO {
	switch typed := item.(type) {
	case models.SubagentText:
		return SubagentItemDTO{Kind: "text", Text: typed.Text}
	case models.SubagentThinking:
		return SubagentItemDTO{Kind: "thinking", Text: typed.Text}
	case models.SubagentNote:
		return SubagentItemDTO{Kind: "note", Text: typed.Text}
	case models.SubagentTool:
		return SubagentItemDTO{Kind: "tool", Name: typed.Name, Args: typed.Args, Running: typed.Running, IsError: typed.IsError, Output: typed.Output}
	default:
		return SubagentItemDTO{Kind: "unknown"}
	}
}
