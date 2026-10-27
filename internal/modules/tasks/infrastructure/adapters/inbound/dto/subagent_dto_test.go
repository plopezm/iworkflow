package dto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// Rust: SubagentTaskDto::from(SubagentTask). Every thread item keeps its kind and its content.
func TestSubagentDTOMapsStatusCountersAndEveryThreadItem(t *testing.T) {
	t.Parallel()
	label := "tests"
	prompt := "revisa los tests"
	step := "leyendo auth.rs"
	args := `{"path":"auth.rs"}`
	output := "fn login()"
	subagent := models.SubagentTask{
		ID:        "s1",
		Agent:     "claude",
		Label:     &label,
		Prompt:    &prompt,
		Status:    models.SubagentRunning,
		LastStep:  &step,
		Turns:     3,
		ToolCalls: 2,
		Thread: []models.SubagentThreadItem{
			models.SubagentText{Text: "hola"},
			models.SubagentThinking{Text: "pienso"},
			models.SubagentNote{Text: "nota"},
			models.SubagentTool{Name: "read", Args: &args, Running: true, Output: &output},
		},
	}

	got := dto.SubagentFromDomain(subagent)

	assert.Equal(t, "s1", got.ID)
	assert.Equal(t, "claude", got.Agent)
	require.NotNil(t, got.Label)
	assert.Equal(t, "tests", *got.Label)
	require.NotNil(t, got.Prompt)
	assert.Equal(t, "revisa los tests", *got.Prompt)
	assert.Equal(t, "running", got.Status)
	require.NotNil(t, got.LastStep)
	assert.Equal(t, "leyendo auth.rs", *got.LastStep)
	assert.Equal(t, uint32(3), got.Turns)
	assert.Equal(t, uint32(2), got.ToolCalls)

	require.Len(t, got.Thread, 4)
	assert.Equal(t, dto.SubagentItemDTO{Kind: "text", Text: "hola"}, got.Thread[0])
	assert.Equal(t, dto.SubagentItemDTO{Kind: "thinking", Text: "pienso"}, got.Thread[1])
	assert.Equal(t, dto.SubagentItemDTO{Kind: "note", Text: "nota"}, got.Thread[2])
	require.Equal(t, "tool", got.Thread[3].Kind)
	assert.Equal(t, "read", got.Thread[3].Name)
	require.NotNil(t, got.Thread[3].Args)
	assert.Equal(t, args, *got.Thread[3].Args)
	assert.True(t, got.Thread[3].Running)
	assert.False(t, got.Thread[3].IsError)
	require.NotNil(t, got.Thread[3].Output)
	assert.Equal(t, output, *got.Thread[3].Output)
}
