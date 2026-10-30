package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Rust: resolve_selection. The selected subagent survives only while it is in the list.
func TestSubagentsViewKeepsTheSelectionOnlyWhileTheSubagentExists(t *testing.T) {
	t.Parallel()
	list := []dto.SubagentDTO{{ID: "a"}, {ID: "b"}}

	assert.Equal(t, "b", state.NewSubagentsView("", "t1", list, "b").Selected)
	assert.Empty(t, state.NewSubagentsView("", "t1", list, "gone").Selected, "a vanished subagent clears the selection")
	assert.Empty(t, state.NewSubagentsView("", "t1", list, "").Selected)
}

// The path carries the selection and the scope only when they are set, so the list address stays plain.
func TestSubagentsPathCarriesTheSelectionAndTheScope(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/tasks/t1/subagents", state.SubagentsPath("t1", "", ""))
	assert.Equal(t, "/tasks/t1/subagents?scope=abc123", state.SubagentsPath("t1", "abc123", ""))
	assert.Equal(t, "/tasks/t1/subagents?scope=abc123&selected=s1", state.SubagentsPath("t1", "abc123", "s1"))
}
