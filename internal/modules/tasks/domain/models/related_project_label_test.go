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
		{
			// The disambiguated "web-11111111" would equal the plain name of the third project: the id part grows.
			"a disambiguated label never equals another project's label",
			[]models.WorkspaceRef{
				{ID: "11111111-aaaa-bbbb-cccc-dddddddddddd", Name: "web"},
				{ID: "22222222-aaaa-bbbb-cccc-dddddddddddd", Name: "web"},
				{ID: "33333333-aaaa-bbbb-cccc-dddddddddddd", Name: "web-11111111"},
			},
			[]string{"web-11111111a", "web-22222222", "web-11111111"},
		},
		{
			// The same workspace listed twice cannot be told apart by its id: a numeric suffix keeps the labels unique.
			"the same workspace twice gets a numeric suffix",
			[]models.WorkspaceRef{
				{ID: "11111111-aaaa", Name: "web"},
				{ID: "11111111-aaaa", Name: "web"},
			},
			[]string{"web-11111111aaaa", "web-11111111aaaa-2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, models.RelatedProjectLabels(tc.refs))
		})
	}
}

// Whatever the order of the projects, each one keeps its label and the labels are all different.
func TestRelatedProjectLabelsAreUniqueAndDoNotDependOnTheOrder(t *testing.T) {
	t.Parallel()
	refs := []models.WorkspaceRef{
		{ID: "11111111-aaaa-bbbb-cccc-dddddddddddd", Name: "web"},
		{ID: "22222222-aaaa-bbbb-cccc-dddddddddddd", Name: "web"},
		{ID: "33333333-aaaa-bbbb-cccc-dddddddddddd", Name: "web-11111111"},
		{ID: "44444444-aaaa-bbbb-cccc-dddddddddddd", Name: "web"},
	}
	reversed := []models.WorkspaceRef{refs[3], refs[2], refs[1], refs[0]}

	forward := models.RelatedProjectLabels(refs)
	backward := models.RelatedProjectLabels(reversed)

	assert.Equal(t, forward, models.RelatedProjectLabels(refs), "the same input gives the same labels")
	assert.Len(t, uniqueStrings(forward), len(refs), "every project has its own label")
	for index, ref := range refs {
		assert.Equal(t, forward[index], backward[len(refs)-1-index], "the label of %s does not depend on the order", ref.ID)
	}
}

func uniqueStrings(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
