package dto

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
)

func storedWorkspace(t *testing.T, createdAt time.Time) models.Workspace {
	t.Helper()
	id, err := models.NewWorkspaceID()
	require.NoError(t, err)
	return models.Workspace{ID: id, Name: "Demo", Path: "/repos/demo", BaseBranch: "main", CreatedAt: createdAt}
}

func TestWorkspaceMapsToDTOWithChronoStyleTimestamp(t *testing.T) {
	workspace := storedWorkspace(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	dto := FromWorkspace(workspace)

	assert.Equal(t, workspace.ID.String(), dto.ID)
	assert.Equal(t, "/repos/demo", dto.Path)
	assert.Equal(t, "2026-01-02T03:04:05+00:00", dto.CreatedAt)
}

func TestWorkspaceTimestampConvertsNonUTCToUTC(t *testing.T) {
	zone := time.FixedZone("UTC+2", 2*60*60)
	workspace := storedWorkspace(t, time.Date(2026, 1, 2, 5, 4, 5, 0, zone))

	dto := FromWorkspace(workspace)

	assert.Equal(t, "2026-01-02T03:04:05+00:00", dto.CreatedAt)
}

func TestRegisterDTOTrimsPath(t *testing.T) {
	agents := "abc"
	dto := RegisterWorkspaceDTO{Name: "Demo", Path: "  /repos/demo  ", Initialize: true, AgentsFileID: &agents}

	input := dto.ToNewWorkspace()

	assert.Equal(t, "/repos/demo", input.Path)
	assert.True(t, input.Initialize)
	assert.Equal(t, "abc", input.AgentsFileID)
}

func TestRegisterDTOWithoutOptionalFieldsDecodesToAbsent(t *testing.T) {
	var dto RegisterWorkspaceDTO
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Demo","path":"/repos/demo","base_branch":null}`), &dto))

	input := dto.ToNewWorkspace()

	assert.Empty(t, input.AgentsFileID)
	assert.Empty(t, input.BaseBranch)
	assert.False(t, input.Initialize)
}

func TestUpdateDTOMapsToDomainUpdate(t *testing.T) {
	branch := "dev"
	agents := "abc"
	dto := UpdateWorkspaceDTO{Name: "Demo", BaseBranch: &branch, AgentsFileID: &agents}

	update := dto.ToWorkspaceUpdate()

	assert.Equal(t, models.WorkspaceUpdate{Name: "Demo", BaseBranch: "dev", AgentsFileID: "abc"}, update)
}

func TestUpdateDTOBlankOptionalFieldsBecomeAbsentAfterValidation(t *testing.T) {
	blank := "  "
	empty := ""
	dto := UpdateWorkspaceDTO{Name: " Demo ", BaseBranch: &blank, AgentsFileID: &empty}

	validated, err := dto.ToWorkspaceUpdate().Validated()

	require.NoError(t, err)
	assert.Equal(t, "Demo", validated.Name)
	assert.Empty(t, validated.BaseBranch)
	assert.Empty(t, validated.AgentsFileID)
}

func TestUpdateDTOWithoutOptionalFieldsDecodesToAbsent(t *testing.T) {
	var dto UpdateWorkspaceDTO
	require.NoError(t, json.Unmarshal([]byte(`{"name":"Demo"}`), &dto))

	assert.Nil(t, dto.BaseBranch)
	assert.Nil(t, dto.AgentsFileID)
}

func TestRegisterOutcomeMapsSuccessToRegistered(t *testing.T) {
	workspace := storedWorkspace(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	outcome, err := FromRegisterOutcome(workspace, nil)

	require.NoError(t, err)
	require.NotNil(t, outcome.Registered)
	assert.Nil(t, outcome.NeedsInitialization)
}

func TestRegisterOutcomeMapsInitializationRequiredToNeedsInitialization(t *testing.T) {
	outcome, err := FromRegisterOutcome(models.Workspace{}, werrors.InitializationRequired("/repos/demo", werrors.ReasonNoCommits))

	require.NoError(t, err)
	assert.Equal(t, &NeedsInitializationDTO{Path: "/repos/demo", Reason: InitializationReasonNoCommits}, outcome.NeedsInitialization)
}

func TestRegisterOutcomeLetsOtherErrorsPassThrough(t *testing.T) {
	_, err := FromRegisterOutcome(models.Workspace{}, werrors.InUse())

	assert.True(t, errors.Is(err, werrors.InUse()))
}

func TestRegisterOutcomeSerializesLikeRustExternallyTaggedEnum(t *testing.T) {
	outcome, err := FromRegisterOutcome(models.Workspace{}, werrors.InitializationRequired("/repos/demo", werrors.ReasonMissingFolder))
	require.NoError(t, err)

	encoded, err := json.Marshal(outcome)

	require.NoError(t, err)
	assert.JSONEq(t, `{"NeedsInitialization":{"path":"/repos/demo","reason":"MissingFolder"}}`, string(encoded))
}

func TestAgentsFileOptionMapsToDTO(t *testing.T) {
	dto := FromAgentsFileOption(models.AgentsFileOption{ID: "a", Description: "Rust"})

	assert.Equal(t, AgentsFileOptionDTO{ID: "a", Description: "Rust"}, dto)
}

func TestDirectoryListingMapsToDTO(t *testing.T) {
	listing := models.DirectoryListing{
		Path:    "/home/pi",
		Parent:  "/home",
		Entries: []models.DirectoryEntry{{Name: "demo", Path: "/home/pi/demo", IsGitRepo: true}},
	}

	dto := FromDirectoryListing(listing)

	assert.Equal(t, "/home/pi", dto.Path)
	require.NotNil(t, dto.Parent)
	assert.Equal(t, "/home", *dto.Parent)
	require.Len(t, dto.Entries, 1)
	assert.True(t, dto.Entries[0].IsGitRepo)
}

func TestDirectoryListingWithoutParentMapsToNull(t *testing.T) {
	dto := FromDirectoryListing(models.DirectoryListing{Path: "/"})

	assert.Nil(t, dto.Parent)
	encoded, err := json.Marshal(dto)
	require.NoError(t, err)
	assert.JSONEq(t, `{"path":"/","parent":null,"entries":[]}`, string(encoded))
}
