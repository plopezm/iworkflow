package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
)

// Each thumbnail opens the enlarged view: it carries the lightbox hooks and the panel has a hidden dialog to show it in.
// The plain link stays as the fallback when the script does not run.
func TestTaskAttachmentsListThumbnailsOpenTheLightbox(t *testing.T) {
	t.Parallel()

	items := []dto.TaskAttachmentDTO{{ID: "att-1", Name: "shot.png", URL: "/api/tasks/t1/attachments/att-1"}}

	body := renderIn(t, i18n.En, components.TaskAttachmentsList(items, ""))

	assert.Contains(t, body, "data-image-lightbox")
	assert.Contains(t, body, `data-src="/api/tasks/t1/attachments/att-1"`)
	assert.Contains(t, body, `data-alt="shot.png"`)
	assert.Contains(t, body, `href="/api/tasks/t1/attachments/att-1"`, "the link is the fallback without script")
	assert.Contains(t, body, "data-lightbox-img")
	assert.Contains(t, body, "data-lightbox")
	assert.Contains(t, body, "display: none", "the dialog starts closed")
}

// A task without images takes no space, so no dialog is rendered either.
func TestTaskAttachmentsListWithoutImagesRendersNothing(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.TaskAttachmentsList(nil, ""))

	assert.NotContains(t, body, "data-lightbox")
}
