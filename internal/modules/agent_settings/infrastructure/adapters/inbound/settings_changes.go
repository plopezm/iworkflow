package inbound

import (
	"iworkflow/internal/core/components"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
)

// The functions below turn one control change into the settings to save. Each one keeps every
// other field of the current settings, exactly like the Rust client did before sending the update:
// changing the runtime also clears the model (each runtime has its own catalog), and an empty value
// returns the field to its default.

// changeRuntime sets the runtime and clears the model; the other fields are kept.
func changeRuntime(current dto.AgentSettingsDTO, value string) dto.AgentSettingsDTO {
	next := current
	next.Runtime = nonEmptyOrNil(value)
	next.ModelProvider = nil
	next.ModelID = nil
	return next
}

// changeModel sets the model from an encoded option value; an empty or invalid value clears it.
func changeModel(current dto.AgentSettingsDTO, value string) dto.AgentSettingsDTO {
	next := current
	provider, model, ok := dto.DecodeModelValue(value)
	if ok {
		next.ModelProvider = &provider
		next.ModelID = &model
	} else {
		next.ModelProvider = nil
		next.ModelID = nil
	}
	return next
}

// changeProfile sets the profile; an empty value uses the active global profile.
func changeProfile(current dto.AgentSettingsDTO, value string) dto.AgentSettingsDTO {
	next := current
	next.Profile = nonEmptyOrNil(value)
	return next
}

// changeThinking sets the reasoning level; an empty value uses the runtime default.
func changeThinking(current dto.AgentSettingsDTO, value string) dto.AgentSettingsDTO {
	next := current
	next.Thinking = nonEmptyOrNil(value)
	return next
}

// changeMemoryLimit sets the RAM cap in MiB; an empty, zero or invalid value removes it.
func changeMemoryLimit(current dto.AgentSettingsDTO, value string) dto.AgentSettingsDTO {
	next := current
	next.MemoryLimitMB = positiveOrNil(value)
	return next
}

// changeCPULimit sets the CPU cap in percent of a core; an empty, zero or invalid value removes it.
func changeCPULimit(current dto.AgentSettingsDTO, value string) dto.AgentSettingsDTO {
	next := current
	next.CPULimitPercent = positiveOrNil(value)
	return next
}

// changeLimitsEnabled turns the master switch on or off and keeps the limit values.
func changeLimitsEnabled(current dto.AgentSettingsDTO, enabled bool) dto.AgentSettingsDTO {
	next := current
	next.LimitsDisabled = !enabled
	return next
}

func nonEmptyOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func positiveOrNil(value string) *uint32 {
	parsed, ok := components.ParsePositiveNumber(value)
	if !ok {
		return nil
	}
	return &parsed
}
