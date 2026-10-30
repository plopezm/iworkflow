package persistence

import (
	"database/sql"
	"math"

	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/persistence/gen"
)

// settingsFromRow maps a stored row to the domain. Identifiers that no longer validate fail the
// read instead of being silently dropped.
func settingsFromRow(row gen.AgentSetting) (models.AgentSettings, error) {
	var settings models.AgentSettings
	if row.ModelProvider.Valid && row.ModelID.Valid {
		settings.Model = &models.ModelRef{Provider: row.ModelProvider.String, Model: row.ModelID.String}
	}
	if row.Profile.Valid {
		profile, err := models.ParseProfileName(row.Profile.String)
		if err != nil {
			return models.AgentSettings{}, err //nolint:wrapcheck // domain error; passed unchanged
		}
		settings.Profile = &profile
	}
	if row.Thinking.Valid {
		thinking, err := models.ParseThinkingLevel(row.Thinking.String)
		if err != nil {
			return models.AgentSettings{}, err //nolint:wrapcheck // domain error; passed unchanged
		}
		settings.Thinking = &thinking
	}
	if row.Runtime.Valid {
		runtime, err := models.ParseAgentRuntime(row.Runtime.String)
		if err != nil {
			return models.AgentSettings{}, err //nolint:wrapcheck // domain error; passed unchanged
		}
		settings.Runtime = &runtime
	}
	settings.MemoryLimitMB = limitFromColumn(row.MemoryLimitMb)
	settings.CPULimitPercent = limitFromColumn(row.CpuLimitPercent)
	settings.LimitsDisabled = row.LimitsDisabled != 0
	return settings, nil
}

// rowFromSettings maps the domain to the row written by ReplaceAgentSettings.
func rowFromSettings(id int64, settings models.AgentSettings) gen.ReplaceAgentSettingsParams {
	params := gen.ReplaceAgentSettingsParams{ID: id, LimitsDisabled: 0}
	if settings.Model != nil {
		params.ModelProvider = sql.NullString{String: settings.Model.Provider, Valid: true}
		params.ModelID = sql.NullString{String: settings.Model.Model, Valid: true}
	}
	if settings.Profile != nil {
		params.Profile = sql.NullString{String: settings.Profile.String(), Valid: true}
	}
	if settings.Thinking != nil {
		params.Thinking = sql.NullString{String: string(*settings.Thinking), Valid: true}
	}
	if settings.Runtime != nil {
		params.Runtime = sql.NullString{String: string(*settings.Runtime), Valid: true}
	}
	params.MemoryLimitMb = limitToColumn(settings.MemoryLimitMB)
	params.CpuLimitPercent = limitToColumn(settings.CPULimitPercent)
	if settings.LimitsDisabled {
		params.LimitsDisabled = 1
	}
	return params
}

// limitFromColumn reads a limit column. Only values in the signed 32-bit range are limits, as in
// the original INTEGER column; anything else reads as "no limit".
func limitFromColumn(value sql.NullInt64) *uint32 {
	if !value.Valid || value.Int64 < 0 || value.Int64 > math.MaxInt32 {
		return nil
	}
	limit := uint32(value.Int64) //nolint:gosec // range checked above
	return &limit
}

// limitToColumn stores a limit. Values beyond the signed 32-bit range are stored as NULL, as the
// original writer did.
func limitToColumn(limit *uint32) sql.NullInt64 {
	if limit == nil || *limit > math.MaxInt32 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*limit), Valid: true}
}
