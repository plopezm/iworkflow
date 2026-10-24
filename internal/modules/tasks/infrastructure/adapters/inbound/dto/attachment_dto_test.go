package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// Rust: image_attachment_dto::dto_round_trips_through_the_domain
func TestImageDTORoundTripsThroughTheDomain(t *testing.T) {
	t.Parallel()
	original := dto.ImageDTO{Name: "a.png", MimeType: "image/png", Data: "aGk="}

	domain := models.ImageAttachment{Name: original.Name, MimeType: original.MimeType, Data: original.Data}
	back := dto.ImageDTOFromDomain(domain)

	assert.Equal(t, original, back)
}

// Rust: image_attachment_dto::task_attachment_dto_maps_from_the_domain
func TestTaskAttachmentDTOMapsFromTheDomain(t *testing.T) {
	t.Parallel()
	stored := models.StoredAttachment{ID: "att-1", Name: "a.png", MimeType: "image/png", Path: "t/att-1.png", SizeBytes: 42}

	view := dto.TaskAttachmentFromDomain("task-1", stored)

	assert.Equal(t, dto.TaskAttachmentDTO{
		ID:        "att-1",
		Name:      "a.png",
		MimeType:  "image/png",
		SizeBytes: 42,
		URL:       "/api/tasks/task-1/attachments/att-1",
	}, view)
	assert.NotContains(t, view.URL, "t/att-1.png", "the storage path never reaches the client")
}

// Rust: image_attachment_dto::missing_images_in_create_payload_default_to_empty
func TestMissingImagesInCreatePayloadDefaultToEmpty(t *testing.T) {
	t.Parallel()
	var form dto.CreateTaskDTO

	require.NoError(t, json.Unmarshal([]byte(`{"workspace_id":"ws","title":"t","prompt":"p"}`), &form))

	assert.Empty(t, form.Images)
	input, err := form.ToNewTask()
	require.NoError(t, err)
	assert.Empty(t, input.Images)
}
