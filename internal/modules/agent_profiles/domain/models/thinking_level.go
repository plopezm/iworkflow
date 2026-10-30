//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"slices"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

// ThinkingLevels returns the reasoning levels pi accepts (`--thinking <level>`).
func ThinkingLevels() []string {
	return []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}
}

// ValidateThinkingLevel accepts only the levels returned by ThinkingLevels.
func ValidateThinkingLevel(raw string) error {
	if slices.Contains(ThinkingLevels(), raw) {
		return nil
	}
	return agenterrors.InvalidRouting("nivel de razonamiento inválido: " + raw)
}
