package dto

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateFormReadsThePathAndTheSameProjectConfirmation(t *testing.T) {
	values := url.Values{
		"name":                 {"Demo"},
		"path":                 {"  /moved/repo  "},
		"confirm_same_project": {"true"},
	}

	dto := UpdateWorkspaceFromForm(values)
	input := dto.ToWorkspaceUpdate()

	assert.Equal(t, "/moved/repo", dto.Path, "the form value is trimmed like the registration path")
	assert.True(t, dto.ConfirmSameProject)
	assert.Equal(t, "/moved/repo", input.Path)
	assert.True(t, input.ConfirmSameProject, "the confirmation reaches the use case")
}

func TestUpdateFormWithoutPathKeepsTheSavedPath(t *testing.T) {
	// A form without the path field, as a disabled (locked) path field sends it, must change nothing about the path.
	values := url.Values{
		"name":        {"Demo"},
		"base_branch": {"dev"},
		"color":       {""},
	}

	dto := UpdateWorkspaceFromForm(values)
	input := dto.ToWorkspaceUpdate()

	assert.Empty(t, dto.Path)
	assert.False(t, dto.ConfirmSameProject, "an absent checkbox is not a confirmation")
	assert.Empty(t, input.Path, "an empty path keeps the saved one")
	assert.False(t, input.ConfirmSameProject)
	assert.Equal(t, "Demo", input.Name)
	assert.Equal(t, "dev", input.BaseBranch)
	assert.NotNil(t, input.Color, "the other fields keep their previous decoding")
}
