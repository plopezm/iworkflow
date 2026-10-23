package state

import (
	"context"
	"strconv"

	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
	presentation "iworkflow/internal/modules/agent_settings/presentation/components"
)

// LimitsData is what the limits card handler read.
//
// Err is the message of a failed read of the stored settings: the card then shows only that error.
// Notice is the message of a failed change: the card keeps the stored values and shows the notice.
type LimitsData struct {
	Settings dto.AgentSettingsDTO
	Err      string
	Notice   string
}

// Limits builds the props of the resource limits card. Empty fields mean "no limit".
func Limits(ctx context.Context, data LimitsData) presentation.ResourceLimitsProps {
	props := presentation.ResourceLimitsProps{
		Title:       msgResourceLimits.T(ctx),
		Help:        msgLimitsHelp.T(ctx),
		Switch:      msgLimitAgentResources.T(ctx),
		NoLimitText: msgNoLimit.T(ctx),
		Notice:      data.Notice,
	}
	if data.Err != "" {
		props.Error = data.Err
		return props
	}
	props.Loaded = true
	props.MemoryMB = limitText(data.Settings.MemoryLimitMB)
	props.CPUPercent = limitText(data.Settings.CPULimitPercent)
	props.Enabled = !data.Settings.LimitsDisabled
	return props
}

// limitText is the form text of a limit: its number, or "" when there is no limit.
func limitText(limit *uint32) string {
	if limit == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*limit), 10)
}
