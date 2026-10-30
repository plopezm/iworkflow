package dto_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/inbound/dto"
)

func TestAgentsFileMapsToDTO(t *testing.T) {
	id, err := models.ParseAgentsFileID("6f1c2a3e-0000-4000-8000-000000000001")
	require.NoError(t, err)
	file := models.AgentsFile{
		ID:          id,
		Description: "Reglas",
		Path:        "/data/a.md",
		CreatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	out := dto.FromAgentsFile(file)

	require.Equal(t, id.String(), out.ID)
	require.Equal(t, "Reglas", out.Description)
	require.Equal(t, "/data/a.md", out.Path)
	require.Equal(t, "2026-01-02T03:04:05+00:00", out.CreatedAt)
}

func TestCreatedAtKeepsFractionsAndUsesTheUTCOffsetLikeRust(t *testing.T) {
	tests := []struct {
		name string
		at   time.Time
		want string
	}{
		{"whole seconds", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), "2026-01-02T03:04:05+00:00"},
		{"milliseconds", time.Date(2026, 1, 2, 3, 4, 5, 120_000_000, time.UTC), "2026-01-02T03:04:05.12+00:00"},
		{"non-UTC input is converted", time.Date(2026, 1, 2, 4, 4, 5, 0, time.FixedZone("CET", 3600)), "2026-01-02T03:04:05+00:00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := dto.FromAgentsFile(models.AgentsFile{CreatedAt: tc.at})

			require.Equal(t, tc.want, out.CreatedAt)
		})
	}
}

func TestProjectMapsToOption(t *testing.T) {
	out := dto.FromProjectRef(models.ProjectRef{ID: "p1", Name: "demo", Path: "/repos/demo"})

	require.Equal(t, dto.ProjectOptionDTO{ID: "p1", Name: "demo"}, out)
}

func TestApplyOutcomeMapsSuccessToApplied(t *testing.T) {
	out, err := dto.ApplyOutcome(models.InstallOutcome{Committed: true}, nil, false)

	require.NoError(t, err)
	require.Equal(t, dto.ApplyAgentsFileOutcomeDTO{Applied: true, Committed: true}, out)
}

func TestApplyOutcomeMapsTargetExistsWithoutOverwriteToAConfirmation(t *testing.T) {
	out, err := dto.ApplyOutcome(models.InstallOutcome{}, agentserrors.TargetExists("/x/AGENTS.md"), false)

	require.NoError(t, err)
	require.Equal(t, dto.ApplyAgentsFileOutcomeDTO{Applied: false}, out)
}

func TestApplyOutcomeKeepsTargetExistsWithOverwrite(t *testing.T) {
	_, err := dto.ApplyOutcome(models.InstallOutcome{}, agentserrors.TargetExists("/x/AGENTS.md"), true)

	require.Equal(t, agentserrors.TargetExists("/x/AGENTS.md"), err)
}

func TestApplyOutcomeLetsOtherErrorsPassThrough(t *testing.T) {
	_, err := dto.ApplyOutcome(models.InstallOutcome{}, agentserrors.ProjectNotFound("p"), false)

	require.Equal(t, agentserrors.ProjectNotFound("p"), err)
	require.False(t, errors.Is(err, agentserrors.TargetExists("")))
}
