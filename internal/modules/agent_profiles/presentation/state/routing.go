package state

import (
	"maps"
	"slices"
	"strings"
)

// OrchestratorKey is the agent that is always listed first in the routing editor.
const OrchestratorKey = "orchestrator"

// RoutingRow is one editable line of the routing editor: agent, model and thinking level. An empty
// model or thinking level means "inherit".
type RoutingRow struct {
	Agent    string
	Model    string
	Thinking string
}

// RoutingEntryView is the text form of a routing entry, with "" for an unset field.
type RoutingEntryView struct {
	Model    string
	Thinking string
}

// Option is a choice of a select in the routing editor.
type Option struct {
	Value string
	Label string
}

// RowsFromRouting turns a routing map into rows, with the orchestrator first and the rest sorted by
// agent name.
func RowsFromRouting(routing map[string]RoutingEntryView) []RoutingRow {
	agents := slices.Sorted(maps.Keys(routing))
	if index := slices.Index(agents, OrchestratorKey); index > 0 {
		agents = slices.Insert(slices.Delete(agents, index, index+1), 0, OrchestratorKey)
	}
	rows := make([]RoutingRow, 0, len(agents))
	for _, agent := range agents {
		entry := routing[agent]
		rows = append(rows, RoutingRow{Agent: agent, Model: entry.Model, Thinking: entry.Thinking})
	}
	return rows
}

// RoutingFromRows turns rows into a routing map. Blank agents are skipped, values are trimmed, blank
// values become "", and when an agent appears twice the last row wins.
func RoutingFromRows(rows []RoutingRow) map[string]RoutingEntryView {
	routing := make(map[string]RoutingEntryView, len(rows))
	for _, row := range rows {
		agent := strings.TrimSpace(row.Agent)
		if agent == "" {
			continue
		}
		routing[agent] = RoutingEntryView{
			Model:    strings.TrimSpace(row.Model),
			Thinking: strings.TrimSpace(row.Thinking),
		}
	}
	return routing
}

// ThinkingChoices returns the reasoning levels offered for a row. An unknown current value is kept, so
// rendering never drops it.
func ThinkingChoices(levels []string, current string) []string {
	choices := slices.Clone(levels)
	if current != "" && !slices.Contains(choices, current) {
		choices = append(choices, current)
	}
	return choices
}

// ModelChoices returns the models offered for a row, keeping an unknown current value.
func ModelChoices(options []Option, current string) []Option {
	choices := slices.Clone(options)
	if current != "" && !slices.ContainsFunc(choices, func(option Option) bool { return option.Value == current }) {
		choices = append(choices, Option{Value: current, Label: current})
	}
	return choices
}

// AddableAgents lists the agents that can still be added: the orchestrator and every role, without
// the ones that already have a row and without repeats.
func AddableAgents(roles []string, rows []RoutingRow) []string {
	candidates := append([]string{OrchestratorKey}, roles...)
	result := make([]string, 0, len(candidates))
	for _, agent := range candidates {
		used := slices.ContainsFunc(rows, func(row RoutingRow) bool { return row.Agent == agent })
		if !used && !slices.Contains(result, agent) {
			result = append(result, agent)
		}
	}
	return result
}

// Row actions of the routing editor.
const (
	ActionAddRow    = "add"
	ActionRemoveRow = "remove"
)

// ApplyRowAction adds a row for agent or removes the rows of agent. Adding an empty or existing
// agent changes nothing.
func ApplyRowAction(rows []RoutingRow, action string, agent string) []RoutingRow {
	switch action {
	case ActionAddRow:
		if agent == "" || slices.ContainsFunc(rows, func(row RoutingRow) bool { return row.Agent == agent }) {
			return rows
		}
		return append(rows, RoutingRow{Agent: agent})
	case ActionRemoveRow:
		return slices.DeleteFunc(rows, func(row RoutingRow) bool { return row.Agent == agent })
	default:
		return rows
	}
}
