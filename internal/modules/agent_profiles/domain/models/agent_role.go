//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"strings"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

// RoleName is a validated agent role name. It uses the same slug format as profile names. The zero
// value is not valid; build values with ParseRoleName.
type RoleName struct {
	value string
}

// ParseRoleName validates raw and returns the role name.
func ParseRoleName(raw string) (RoleName, error) {
	if !isValidSlug(raw) {
		return RoleName{}, agenterrors.InvalidRoleName(raw)
	}
	return RoleName{value: raw}, nil
}

// String returns the name as used for the `<name>.md` file.
func (n RoleName) String() string {
	return n.value
}

// AgentRole is an existing agent role, that is, a `<role>.md` file in the agents folder.
type AgentRole struct {
	Name        RoleName
	Description string
	Model       *string
	Thinking    *string
}

// NewAgentRole holds the data needed to create an agent role.
type NewAgentRole struct {
	Name        RoleName
	Description string
	Prompt      string
	Model       *string
	Thinking    *string
}

// Validate checks that the model, when present, is a non-blank single line and that the thinking
// level, when present, is known.
func (r NewAgentRole) Validate() error {
	if r.Model != nil {
		if strings.TrimSpace(*r.Model) == "" {
			return agenterrors.InvalidRouting("modelo vacío")
		}
		if strings.ContainsAny(*r.Model, "\n\r") {
			return agenterrors.InvalidRouting("el modelo no puede contener saltos de línea")
		}
	}
	if r.Thinking != nil {
		return ValidateThinkingLevel(*r.Thinking)
	}
	return nil
}
