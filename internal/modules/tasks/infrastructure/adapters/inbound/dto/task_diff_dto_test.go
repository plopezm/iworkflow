package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// Rust: task_diff_dto::from_domain copies every field of the diff.
func TestTaskDiffFromDomainCopiesEveryField(t *testing.T) {
	t.Parallel()
	diff := models.TaskDiff{
		Stat:      " main.go | 2 +-\n 1 file changed",
		Patch:     "diff --git a/main.go b/main.go\n",
		Truncated: true,
		Commits:   []string{"abc1234 fix the thing", "def5678 add the test"},
	}

	got := dto.TaskDiffFromDomain(diff)

	assert.Equal(t, dto.TaskDiffDTO{
		Stat:      diff.Stat,
		Patch:     diff.Patch,
		Truncated: true,
		Commits:   diff.Commits,
	}, got)
}

// The JSON names are the ones the Rust DTO serialized, so the client keeps reading the same fields.
func TestTaskDiffDTOKeepsTheRustJSONNames(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(dto.TaskDiffDTO{Stat: "s", Patch: "p", Truncated: false, Commits: []string{"c"}})
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))

	assert.Equal(t, map[string]any{"stat": "s", "patch": "p", "truncated": false, "commits": []any{"c"}}, fields)
}
