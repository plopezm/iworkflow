package dto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// Rust: MergeOutcomeDto::from(MergeOutcome). A clean integration carries no files; a conflict carries the files.
func TestMergeOutcomeDTOMapsAMergeAndAConflict(t *testing.T) {
	t.Parallel()

	merged := dto.MergeOutcomeFromDomain(models.MergedOutcome())
	assert.False(t, merged.Conflict)
	assert.Empty(t, merged.Files)

	conflict := dto.MergeOutcomeFromDomain(models.ConflictOutcome([]string{"a.go", "b.go"}))
	assert.True(t, conflict.Conflict)
	assert.Equal(t, []string{"a.go", "b.go"}, conflict.Files)
}
