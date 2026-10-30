package models

import agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"

// ThinkingLevel is the reasoning effort of the agent (pi `--thinking <level>`, claude `--effort`).
// pi clamps the value to the capabilities of the active model, so no per-model validation is
// needed on this side.
type ThinkingLevel string

// Thinking levels. The string value is the pi identifier (lowercase).
const (
	ThinkingOff     ThinkingLevel = "off"
	ThinkingMinimal ThinkingLevel = "minimal"
	ThinkingLow     ThinkingLevel = "low"
	ThinkingMedium  ThinkingLevel = "medium"
	ThinkingHigh    ThinkingLevel = "high"
	ThinkingXHigh   ThinkingLevel = "xhigh"
	ThinkingMax     ThinkingLevel = "max"
)

// AllThinkingLevels lists the seven levels in increasing order.
func AllThinkingLevels() []ThinkingLevel {
	return []ThinkingLevel{
		ThinkingOff, ThinkingMinimal, ThinkingLow, ThinkingMedium,
		ThinkingHigh, ThinkingXHigh, ThinkingMax,
	}
}

// SupportedBy returns the levels that runtime accepts: `claude --effort` has no `off` or
// `minimal`; pi and gentle-shell accept all of them; Cursor has no levels (the effort is part of
// each model id). The result is never nil.
func SupportedBy(runtime AgentRuntime) []ThinkingLevel {
	switch runtime {
	case RuntimeClaude:
		return []ThinkingLevel{ThinkingLow, ThinkingMedium, ThinkingHigh, ThinkingXHigh, ThinkingMax}
	case RuntimeCursor:
		return []ThinkingLevel{}
	default:
		return AllThinkingLevels()
	}
}

// IsSupportedBy reports whether runtime accepts the level.
func (l ThinkingLevel) IsSupportedBy(runtime AgentRuntime) bool {
	for _, supported := range SupportedBy(runtime) {
		if supported == l {
			return true
		}
	}
	return false
}

// ParseThinkingLevel validates a thinking level identifier.
func ParseThinkingLevel(raw string) (ThinkingLevel, error) {
	for _, level := range AllThinkingLevels() {
		if string(level) == raw {
			return level, nil
		}
	}
	return "", agenterrors.InvalidThinkingLevel(raw) //nolint:wrapcheck // domain constructor returns a domain error
}
