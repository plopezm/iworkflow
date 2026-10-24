package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: agent_capabilities.rs::tests::full_enables_everything
func TestFullAgentCapabilitiesEnableEverything(t *testing.T) {
	assert.Equal(t, models.AgentCapabilities{
		Steer:          true,
		FollowUp:       true,
		Dialogs:        true,
		ModelSelection: true,
		ThinkingLevels: true,
		Resume:         true,
		Subagents:      true,
		Modes:          true,
	}, models.FullAgentCapabilities())
}

// Rust: agent_capabilities.rs::tests::minimal_disables_everything_optional
func TestMinimalAgentCapabilitiesDisableEverythingOptional(t *testing.T) {
	assert.Equal(t, models.AgentCapabilities{}, models.MinimalAgentCapabilities())
}
