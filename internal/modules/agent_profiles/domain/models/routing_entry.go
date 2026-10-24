//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"strings"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

// RoutingEntry is the model and thinking level assigned to one agent inside a profile. A nil
// field means "not set".
type RoutingEntry struct {
	Model    *string
	Thinking *string
}

// Validate checks that the model, when present, is not blank and that the thinking level, when
// present, is known.
func (e RoutingEntry) Validate() error {
	if e.Model != nil && strings.TrimSpace(*e.Model) == "" {
		return agenterrors.InvalidRouting("modelo vacío")
	}
	if e.Thinking != nil {
		return ValidateThinkingLevel(*e.Thinking)
	}
	return nil
}

// CloneRouting copies a routing map, pointers included, so the copy shares no state with the
// original. A nil map becomes an empty one.
func CloneRouting(routing map[string]RoutingEntry) map[string]RoutingEntry {
	cloned := make(map[string]RoutingEntry, len(routing))
	for agent, entry := range routing {
		cloned[agent] = RoutingEntry{Model: clonePointer(entry.Model), Thinking: clonePointer(entry.Thinking)}
	}
	return cloned
}

func clonePointer(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
