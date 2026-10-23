package agent_settings_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/persistence"
)

func TestMissingRowReturnsDefaultSettings(t *testing.T) {
	adapter := persistence.NewAgentSettingsRepositoryAdapter(dbtest.New(t))

	settings, err := adapter.Get(context.Background())

	require.NoError(t, err)
	require.Equal(t, models.AgentSettings{}, settings)
}

func TestSaveAndGetRoundTripFullSettings(t *testing.T) {
	adapter := persistence.NewAgentSettingsRepositoryAdapter(dbtest.New(t))
	thinking := models.ThinkingHigh
	settings := models.AgentSettings{
		Model:    &models.ModelRef{Provider: "anthropic", Model: "claude-5"},
		Profile:  profileOf(t, "Claude"),
		Thinking: &thinking,
	}

	require.NoError(t, adapter.Save(context.Background(), settings))
	loaded, err := adapter.Get(context.Background())

	require.NoError(t, err)
	require.Equal(t, settings, loaded)
}

func TestSaveAndGetRoundTripThinkingOnly(t *testing.T) {
	adapter := persistence.NewAgentSettingsRepositoryAdapter(dbtest.New(t))
	thinking := models.ThinkingXHigh
	settings := models.AgentSettings{Thinking: &thinking}

	require.NoError(t, adapter.Save(context.Background(), settings))
	loaded, err := adapter.Get(context.Background())

	require.NoError(t, err)
	require.Equal(t, settings, loaded)
}

func TestSaveUpsertsTheSingleRow(t *testing.T) {
	database := dbtest.New(t)
	adapter := persistence.NewAgentSettingsRepositoryAdapter(database)
	low := models.ThinkingLow
	first := models.AgentSettings{
		Model:    &models.ModelRef{Provider: "anthropic", Model: "claude-5"},
		Thinking: &low,
	}
	second := models.AgentSettings{Profile: profileOf(t, "Gentleman")}

	require.NoError(t, adapter.Save(context.Background(), first))
	require.NoError(t, adapter.Save(context.Background(), second))
	loaded, err := adapter.Get(context.Background())

	require.NoError(t, err)
	require.Equal(t, second, loaded)
	require.Equal(t, 1, countRows(t, database))
}

func TestSaveAndGetRoundTripRuntime(t *testing.T) {
	adapter := persistence.NewAgentSettingsRepositoryAdapter(dbtest.New(t))
	pi := models.RuntimePi
	settings := models.AgentSettings{Runtime: &pi}

	require.NoError(t, adapter.Save(context.Background(), settings))
	loaded, err := adapter.Get(context.Background())

	require.NoError(t, err)
	require.Equal(t, settings, loaded)
}

func TestSaveAndGetRoundTripResourceLimits(t *testing.T) {
	adapter := persistence.NewAgentSettingsRepositoryAdapter(dbtest.New(t))
	memory := uint32(4096)
	cpu := uint32(200)
	settings := models.AgentSettings{MemoryLimitMB: &memory, CPULimitPercent: &cpu, LimitsDisabled: true}

	require.NoError(t, adapter.Save(context.Background(), settings))
	loaded, err := adapter.Get(context.Background())

	require.NoError(t, err)
	require.Equal(t, settings, loaded)
}

func TestLimitsAboveTheSigned32BitRangeAreStoredAsNull(t *testing.T) {
	database := dbtest.New(t)
	adapter := persistence.NewAgentSettingsRepositoryAdapter(database)
	tooBig := uint32(1 << 31)

	require.NoError(t, adapter.Save(context.Background(), models.AgentSettings{MemoryLimitMB: &tooBig}))
	loaded, err := adapter.Get(context.Background())

	require.NoError(t, err)
	require.Nil(t, loaded.MemoryLimitMB)
}

func TestStoredProfileThatNoLongerValidatesFailsTheRead(t *testing.T) {
	database := dbtest.New(t)
	adapter := persistence.NewAgentSettingsRepositoryAdapter(database)
	require.NoError(t, adapter.Save(context.Background(), models.AgentSettings{}))
	_, err := database.ExecContext(context.Background(),
		`UPDATE agent_settings SET profile = 'not valid!' WHERE id = 1`)
	require.NoError(t, err)

	_, err = adapter.Get(context.Background())

	require.ErrorIs(t, err, agenterrors.ErrInvalidProfileName)
}

func TestStoredRuntimeThatNoLongerValidatesFailsTheRead(t *testing.T) {
	database := dbtest.New(t)
	adapter := persistence.NewAgentSettingsRepositoryAdapter(database)
	require.NoError(t, adapter.Save(context.Background(), models.AgentSettings{}))
	_, err := database.ExecContext(context.Background(), `UPDATE agent_settings SET runtime = 'codex' WHERE id = 1`)
	require.NoError(t, err)

	_, err = adapter.Get(context.Background())

	require.ErrorIs(t, err, agenterrors.ErrInvalidRuntime)
}

func TestClosedDatabaseIsARepositoryError(t *testing.T) {
	database := dbtest.New(t)
	require.NoError(t, database.Close())
	adapter := persistence.NewAgentSettingsRepositoryAdapter(database)

	_, getErr := adapter.Get(context.Background())
	saveErr := adapter.Save(context.Background(), models.AgentSettings{})

	require.ErrorIs(t, getErr, agenterrors.ErrRepository)
	require.ErrorIs(t, saveErr, agenterrors.ErrRepository)
}

func countRows(t *testing.T, database *sql.DB) int {
	t.Helper()
	var count int
	require.NoError(t, database.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM agent_settings`).Scan(&count))
	return count
}
