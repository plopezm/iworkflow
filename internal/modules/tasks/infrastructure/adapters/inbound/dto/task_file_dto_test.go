package dto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// Rust: task_file_dto::maps_file_content
func TestTaskFileDTOMapsFileContent(t *testing.T) {
	t.Parallel()
	file := dto.TaskFileFromDomain(models.TaskFileContent{Path: "src/a.rs", Content: "fn a() {}"})

	assert.Equal(t, dto.TaskFileDTO{Path: "src/a.rs", Content: "fn a() {}"}, file)
}

// Rust: task_file_dto::maps_commit_flag
func TestSaveTaskFileResultMapsCommitFlag(t *testing.T) {
	t.Parallel()
	assert.True(t, dto.SaveTaskFileResult(true).Committed)
	assert.False(t, dto.SaveTaskFileResult(false).Committed)
}
