package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/domain/models"
)

func TestExpandTemplate(t *testing.T) {
	t.Parallel()
	run := models.RunContext{
		TaskID: "t1", TaskTitle: "Add login", TaskStatus: "in_progress", WorkspacePath: "/ws", WorktreePath: "/wt",
		Branch: "feat/login", Phase: models.PhasePost, Event: models.EventTaskFinish, Result: models.TaskResultOK,
	}
	tests := []struct {
		name     string
		text     string
		want     string
		wantFail bool
	}{
		{name: "plain text is unchanged", text: "check the code", want: "check the code"},
		{name: "fields are expanded", text: "{{.Branch}} in {{.WorktreePath}} on {{.Phase}}/{{.Event}}: {{.Result}}", want: "feat/login in /wt on post/task_finish: ok"},
		{name: "shell metacharacters stay literal", text: "$(rm -rf /) {{.TaskTitle}}; echo", want: "$(rm -rf /) Add login; echo"},
		{name: "unknown field is an error", text: "{{.Nope}}", wantFail: true},
		{name: "broken syntax is an error", text: "{{.Branch", wantFail: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := models.ExpandTemplate(tc.text, run)
			if tc.wantFail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
