package dto_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

func TestCreateTaskFromFormReadsEveryFieldOfTheBoardForm(t *testing.T) {
	values := url.Values{
		"workspace_id": {" w1 "},
		"title":        {"  Fix login "},
		"prompt":       {"Do it\n"},
		"priority":     {"high"},
		"start_now":    {"on"},
	}

	input := dto.CreateTaskFromForm(values)

	assert.Equal(t, "w1", input.WorkspaceID)
	assert.Equal(t, "Fix login", input.Title)
	assert.Equal(t, "Do it\n", input.Prompt, "the prompt is sent as typed")
	assert.Equal(t, "high", input.Priority)
	assert.True(t, input.StartNow)
	assert.Nil(t, input.BaseBranch, "a form without a base branch keeps the workspace base")
	assert.Nil(t, input.MemoryLimitMB)
}

func TestCreateTaskFromFormUncheckedStartIsFalseAndBlankPriorityKeepsTheDefault(t *testing.T) {
	input := dto.CreateTaskFromForm(url.Values{"prompt": {"p"}})

	assert.False(t, input.StartNow)
	assert.Empty(t, input.Priority)
}
