package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/terminal/domain/models"
)

func sampleSession(status models.TerminalStatus) models.TerminalSession {
	return models.TerminalSession{
		ID:            models.TerminalSessionIDFromUUID(uuid.UUID{15: 1}),
		WorkspaceID:   uuid.UUID{15: 2},
		WorkspaceName: "demo",
		CreatedAt:     time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		Status:        status,
	}
}

func TestFromTerminalSessionMapsEveryField(t *testing.T) {
	dto := FromTerminalSession(sampleSession(models.TerminalStatusRunning))

	require.Equal(t, "00000000-0000-0000-0000-000000000001", dto.ID)
	require.Equal(t, "00000000-0000-0000-0000-000000000002", dto.WorkspaceID)
	require.Equal(t, "demo", dto.WorkspaceName)
	require.Equal(t, "2026-01-02T03:04:05+00:00", dto.CreatedAt)
	require.Equal(t, TerminalStatusRunningDTO, dto.Status)
}

func TestFromTerminalSessionMapsTheExitedStatus(t *testing.T) {
	dto := FromTerminalSession(sampleSession(models.TerminalStatusExited))

	require.Equal(t, TerminalStatusExitedDTO, dto.Status)
}

func TestFormatChronoRFC3339KeepsFractionsInStepsOfThree(t *testing.T) {
	base := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	require.Equal(t, "2026-01-02T03:04:05+00:00", formatChronoRFC3339(base))
	require.Equal(t, "2026-01-02T03:04:05.123+00:00", formatChronoRFC3339(base.Add(123*time.Millisecond)))
	require.Equal(t, "2026-01-02T03:04:05.000123+00:00", formatChronoRFC3339(base.Add(123*time.Microsecond)))
	require.Equal(t, "2026-01-02T03:04:05.000000123+00:00", formatChronoRFC3339(base.Add(123*time.Nanosecond)))
}

func TestTerminalSessionDTORoundTripsThroughJSON(t *testing.T) {
	dto := FromTerminalSession(sampleSession(models.TerminalStatusExited))

	payload, err := json.Marshal(dto)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"00000000-0000-0000-0000-000000000001","workspace_id":"00000000-0000-0000-0000-000000000002","workspace_name":"demo","created_at":"2026-01-02T03:04:05+00:00","status":"Exited"}`, string(payload))

	var back TerminalSessionDTO
	require.NoError(t, json.Unmarshal(payload, &back))
	require.Equal(t, dto, back)
}
