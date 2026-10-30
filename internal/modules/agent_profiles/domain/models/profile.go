//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"maps"
	"slices"
	"strings"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

// OrchestratorKey is the routing key of the orchestrator agent. It is always allowed.
const OrchestratorKey = "orchestrator"

// Profile is a model profile: the routing of every agent (role -> model and thinking level).
type Profile struct {
	Name    ProfileName
	Routing map[string]RoutingEntry
}

// NewProfile builds a profile. A nil routing becomes an empty one, so profiles compare equal
// whether they were built or loaded.
func NewProfile(name ProfileName, routing map[string]RoutingEntry) Profile {
	if routing == nil {
		routing = map[string]RoutingEntry{}
	}
	return Profile{Name: name, Routing: routing}
}

// ValidateRouting checks the agent keys and entries. Keys are visited in sorted order, so the
// first error reported is always the same one.
func ValidateRouting(routing map[string]RoutingEntry) error {
	for _, agent := range slices.Sorted(maps.Keys(routing)) {
		if strings.TrimSpace(agent) == "" {
			return agenterrors.InvalidRouting("clave de agente vacía")
		}
		if err := routing[agent].Validate(); err != nil {
			return err
		}
	}
	return nil
}
