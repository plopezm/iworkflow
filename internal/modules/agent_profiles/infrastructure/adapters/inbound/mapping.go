package inbound

import (
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

// routingToView converts a domain routing to its text form. An unset field becomes "".
func routingToView(routing map[string]models.RoutingEntry) map[string]state.RoutingEntryView {
	views := make(map[string]state.RoutingEntryView, len(routing))
	for agent, entry := range routing {
		views[agent] = state.RoutingEntryView{Model: textOf(entry.Model), Thinking: textOf(entry.Thinking)}
	}
	return views
}

// routingFromView converts the text form back to a domain routing. A blank field becomes unset.
func routingFromView(views map[string]state.RoutingEntryView) map[string]models.RoutingEntry {
	routing := make(map[string]models.RoutingEntry, len(views))
	for agent, view := range views {
		routing[agent] = models.RoutingEntry{Model: optionalText(view.Model), Thinking: optionalText(view.Thinking)}
	}
	return routing
}

func textOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func modelOptions(options []outbound.ModelOption) []state.Option {
	converted := make([]state.Option, 0, len(options))
	for _, option := range options {
		converted = append(converted, state.Option{Value: option.ID, Label: option.Label})
	}
	return converted
}

func roleNames(roles []models.AgentRole) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name.String())
	}
	return names
}

func roleItems(roles []models.AgentRole) []state.RoleItem {
	items := make([]state.RoleItem, 0, len(roles))
	for _, role := range roles {
		items = append(items, state.RoleItem{Name: role.Name.String(), Description: role.Description})
	}
	return items
}
