package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	shared "iworkflow/internal/core/components"
	"iworkflow/internal/modules/workspaces/domain/models"
)

// The domain validates the palette keys and the shared component draws them: both lists must agree.
func TestDomainAndPresentationShareTheSamePalette(t *testing.T) {
	assert.Equal(t, models.Colors(), shared.WorkspaceColorKeys())
}
