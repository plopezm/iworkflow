package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/models"
)

func TestRelatedProjectLabels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		refs []models.WorkspaceRef
		want []string
	}{
		{"no projects", nil, []string{}},
		{"plain name", []models.WorkspaceRef{{ID: "a", Name: "api"}}, []string{"api"}},
		{"whitespace and colon become one dash", []models.WorkspaceRef{{ID: "a", Name: "  my  big:app "}}, []string{"my-big-app"}},
		{"empty name falls back", []models.WorkspaceRef{{ID: "a", Name: " : "}}, []string{"project"}},
		{
			"same name is told apart by id",
			[]models.WorkspaceRef{
				{ID: "11111111-2222-3333-4444-555555555555", Name: "web"},
				{ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Name: "web"},
				{ID: "c", Name: "api"},
			},
			[]string{"web-11111111", "web-aaaaaaaa", "api"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, models.RelatedProjectLabels(tc.refs))
		})
	}
}
