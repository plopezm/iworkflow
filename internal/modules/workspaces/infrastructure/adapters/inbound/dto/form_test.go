package dto

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterFormReadsEveryField(t *testing.T) {
	values := url.Values{
		"name":           {"Demo"},
		"path":           {"  /repos/demo  "},
		"base_branch":    {"dev"},
		"agents_file_id": {"abc"},
		"initialize":     {"true"},
	}

	dto := RegisterWorkspaceFromForm(values)

	assert.Equal(t, "Demo", dto.Name)
	assert.Equal(t, "  /repos/demo  ", dto.Path, "the form keeps the raw path; the domain trims it")
	require.NotNil(t, dto.BaseBranch)
	assert.Equal(t, "dev", *dto.BaseBranch)
	require.NotNil(t, dto.AgentsFileID)
	assert.Equal(t, "abc", *dto.AgentsFileID)
	assert.True(t, dto.Initialize)
}

func TestRegisterFormWithoutFieldsMeansNotGiven(t *testing.T) {
	cases := map[string]url.Values{
		"absent":        {},
		"empty strings": {"base_branch": {""}, "agents_file_id": {""}, "initialize": {""}},
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			input := RegisterWorkspaceFromForm(values).ToNewWorkspace()

			assert.Empty(t, input.BaseBranch)
			assert.Empty(t, input.AgentsFileID)
			assert.False(t, input.Initialize)
		})
	}
}

func TestRegisterFormInitializeOnlyAcceptsTrue(t *testing.T) {
	assert.False(t, RegisterWorkspaceFromForm(url.Values{"initialize": {"yes"}}).Initialize)
	assert.False(t, RegisterWorkspaceFromForm(url.Values{"initialize": {"TRUE"}}).Initialize)
	assert.True(t, RegisterWorkspaceFromForm(url.Values{"initialize": {"true"}}).Initialize)
}

func TestUpdateFormReadsNameBranchAndAgentsFileWithoutPath(t *testing.T) {
	dto := UpdateWorkspaceFromForm(url.Values{
		"name":           {"Nuevo"},
		"base_branch":    {" develop "},
		"agents_file_id": {""},
		"path":           {"/somewhere/else"},
	})

	update := dto.ToWorkspaceUpdate()

	assert.Equal(t, "Nuevo", update.Name)
	assert.Equal(t, " develop ", update.BaseBranch)
	assert.Empty(t, update.AgentsFileID)
}
