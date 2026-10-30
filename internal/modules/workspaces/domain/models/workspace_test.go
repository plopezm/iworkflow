package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
)

func TestNewWorkspaceValidatedTrimsNameAndMapsBlankBranchToAbsent(t *testing.T) {
	input := models.NewWorkspace{Name: "  demo ", Path: "/tmp/demo", BaseBranch: "  ", Initialize: true}

	validated, err := input.Validated()

	require.NoError(t, err)
	assert.Equal(t, "demo", validated.Name)
	assert.Empty(t, validated.BaseBranch)
	assert.True(t, validated.Initialize)
}

func TestNewWorkspaceValidatedTrimsAgentsFileIDAndMapsBlankToAbsent(t *testing.T) {
	withID, err := models.NewWorkspace{Name: "demo", Path: "/tmp/demo", AgentsFileID: "  abc "}.Validated()
	require.NoError(t, err)
	blank, err := models.NewWorkspace{Name: "demo", Path: "/tmp/demo", AgentsFileID: "   "}.Validated()
	require.NoError(t, err)

	assert.Equal(t, "abc", withID.AgentsFileID)
	assert.Empty(t, blank.AgentsFileID)
}

func TestNewWorkspaceValidatedRejectsBlankName(t *testing.T) {
	_, err := models.NewWorkspace{Name: "   ", Path: "/tmp/demo"}.Validated()

	require.ErrorIs(t, err, werrors.InvalidName())
}

func TestNewWorkspaceValidatedRejectsRelativePath(t *testing.T) {
	_, err := models.NewWorkspace{Name: "demo", Path: "relative/dir"}.Validated()

	require.ErrorIs(t, err, werrors.PathNotAbsolute(""))
	assert.EqualError(t, err, "la ruta debe ser absoluta: relative/dir")
}

func TestWorkspaceUpdateValidatedTrimsAndMapsBlanksToAbsent(t *testing.T) {
	validated, err := models.WorkspaceUpdate{Name: "  demo ", BaseBranch: "  dev ", AgentsFileID: " abc "}.Validated()
	require.NoError(t, err)
	blank, err := models.WorkspaceUpdate{Name: "demo", BaseBranch: "  ", AgentsFileID: "  "}.Validated()
	require.NoError(t, err)

	assert.Equal(t, models.WorkspaceUpdate{Name: "demo", BaseBranch: "dev", AgentsFileID: "abc"}, validated)
	assert.Equal(t, models.WorkspaceUpdate{Name: "demo"}, blank)
}

func TestWorkspaceUpdateValidatedRejectsBlankAndTooLongName(t *testing.T) {
	_, blankErr := models.WorkspaceUpdate{Name: "   "}.Validated()
	_, longErr := models.WorkspaceUpdate{Name: strings.Repeat("a", 81)}.Validated()
	_, maxErr := models.WorkspaceUpdate{Name: strings.Repeat("a", 80)}.Validated()

	require.ErrorIs(t, blankErr, werrors.InvalidName())
	require.ErrorIs(t, longErr, werrors.InvalidName())
	require.NoError(t, maxErr)
}

func TestParseWorkspaceIDRejectsGarbage(t *testing.T) {
	_, err := models.ParseWorkspaceID("nope")

	require.ErrorIs(t, err, werrors.InvalidID(""))
	assert.EqualError(t, err, "identificador de workspace inválido: nope")
}

func TestParseWorkspaceIDRoundTripsGeneratedID(t *testing.T) {
	generated, err := models.NewWorkspaceID()
	require.NoError(t, err)

	parsed, err := models.ParseWorkspaceID(generated.String())

	require.NoError(t, err)
	assert.Equal(t, generated, parsed)
}
