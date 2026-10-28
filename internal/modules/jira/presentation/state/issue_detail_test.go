package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/state"
)

func TestIssueDetailViewHelpers(t *testing.T) {
	nameAna := "Ana"
	nameBob := "Bob"
	taskID := "task-123"

	view := state.IssueDetailView{
		Issue: dto.JiraIssueDTO{
			Key:         "ABC-1",
			Summary:     "Falla el login",
			Description: "El login falla en el navegador.",
			Assignee:    &nameAna,
			Reporter:    &nameBob,
			Labels:      []string{"frontend", "auth"},
			TaskID:      &taskID,
		},
	}

	assert.Equal(t, "El login falla en el navegador.", view.DescriptionText(i18n.Es))
	assert.Equal(t, "Ana", view.AssigneeText())
	assert.Equal(t, "Bob", view.ReporterText())
	assert.Equal(t, "frontend, auth", view.LabelsText())
	assert.Equal(t, "/jira/ABC-1/convert", view.ConvertPath())
	assert.Equal(t, "/tasks/task-123", view.TaskPath())

	emptyView := state.IssueDetailView{
		Issue: dto.JiraIssueDTO{
			Key: "ABC-2",
		},
	}
	assert.Equal(t, "(sin descripción)", emptyView.DescriptionText(i18n.Es))
	assert.Equal(t, "(no description)", emptyView.DescriptionText(i18n.En))
	assert.Equal(t, "—", emptyView.AssigneeText())
	assert.Equal(t, "—", emptyView.ReporterText())
	assert.Equal(t, "—", emptyView.LabelsText())
	assert.Equal(t, "", emptyView.TaskPath())
}
