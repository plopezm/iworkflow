package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
)

func stringPointer(value string) *string {
	return &value
}

func uint32Pointer(value uint32) *uint32 {
	return &value
}

func TestSettingsRoundTripThroughDTO(t *testing.T) {
	profile, err := models.ParseProfileName("Claude")
	require.NoError(t, err)
	thinking := models.ThinkingHigh
	settings := models.AgentSettings{
		Model:    &models.ModelRef{Provider: "anthropic", Model: "claude-5"},
		Profile:  &profile,
		Thinking: &thinking,
	}

	back, err := dto.FromAgentSettings(settings).ToAgentSettings()

	require.NoError(t, err)
	require.Equal(t, settings, back)
}

func TestEmptyDTOMapsToDefaultSettings(t *testing.T) {
	settings, err := dto.AgentSettingsDTO{}.ToAgentSettings()

	require.NoError(t, err)
	require.Equal(t, models.AgentSettings{}, settings)
}

func TestPartialModelFieldsMapToNoModel(t *testing.T) {
	cases := map[string]dto.AgentSettingsDTO{
		"missing id":       {ModelProvider: stringPointer("anthropic")},
		"missing provider": {ModelID: stringPointer("claude-5")},
		"empty provider":   {ModelProvider: stringPointer(""), ModelID: stringPointer("claude-5")},
		"empty id":         {ModelProvider: stringPointer("anthropic"), ModelID: stringPointer("")},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			settings, err := input.ToAgentSettings()

			require.NoError(t, err)
			require.Nil(t, settings.Model)
		})
	}
}

func TestEmptyIdentifiersMapToNotSet(t *testing.T) {
	settings, err := dto.AgentSettingsDTO{
		Profile:  stringPointer(""),
		Thinking: stringPointer(""),
		Runtime:  stringPointer(""),
	}.ToAgentSettings()

	require.NoError(t, err)
	require.Nil(t, settings.Profile)
	require.Nil(t, settings.Thinking)
	require.Nil(t, settings.Runtime)
}

func TestInvalidProfileNameIsRejected(t *testing.T) {
	_, err := dto.AgentSettingsDTO{Profile: stringPointer("invalid profile")}.ToAgentSettings()

	require.ErrorIs(t, err, agenterrors.ErrInvalidProfileName)
}

func TestInvalidThinkingLevelIsRejected(t *testing.T) {
	_, err := dto.AgentSettingsDTO{Thinking: stringPointer("ultra")}.ToAgentSettings()

	require.EqualError(t, err, "nivel de razonamiento inválido: ultra")
}

func TestInvalidRuntimeIsRejected(t *testing.T) {
	_, err := dto.AgentSettingsDTO{Runtime: stringPointer("codex")}.ToAgentSettings()

	require.EqualError(t, err, "runtime de agente inválido: codex")
}

func TestLimitsRoundTripThroughDTO(t *testing.T) {
	settings := models.AgentSettings{
		MemoryLimitMB:   uint32Pointer(2048),
		CPULimitPercent: uint32Pointer(150),
		LimitsDisabled:  true,
	}

	back, err := dto.FromAgentSettings(settings).ToAgentSettings()

	require.NoError(t, err)
	require.Equal(t, settings, back)
}

func TestZeroLimitsAreRejectedWithTheirMessage(t *testing.T) {
	cases := map[string]struct {
		input   dto.AgentSettingsDTO
		message string
	}{
		"memory": {
			input:   dto.AgentSettingsDTO{MemoryLimitMB: uint32Pointer(0)},
			message: "límite de recursos inválido: el límite de memoria debe ser mayor que 0",
		},
		"cpu": {
			input:   dto.AgentSettingsDTO{CPULimitPercent: uint32Pointer(0)},
			message: "límite de recursos inválido: el límite de CPU debe ser mayor que 0",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := testCase.input.ToAgentSettings()

			require.ErrorIs(t, err, agenterrors.ErrInvalidLimit)
			require.EqualError(t, err, testCase.message)
		})
	}
}

func TestValidationOrderIsProfileThinkingRuntimeMemoryCPU(t *testing.T) {
	// Every field is invalid: the first failing check in the contract order decides the message.
	_, err := dto.AgentSettingsDTO{
		Profile:         stringPointer("bad profile"),
		Thinking:        stringPointer("ultra"),
		Runtime:         stringPointer("codex"),
		MemoryLimitMB:   uint32Pointer(0),
		CPULimitPercent: uint32Pointer(0),
	}.ToAgentSettings()

	require.ErrorIs(t, err, agenterrors.ErrInvalidProfileName)
}

func TestRuntimeRoundTripsThroughDTO(t *testing.T) {
	runtime := models.RuntimePi
	settings := models.AgentSettings{Runtime: &runtime}

	back, err := dto.FromAgentSettings(settings).ToAgentSettings()

	require.NoError(t, err)
	require.Equal(t, settings, back)
}

func TestSettingsJSONUsesTheWireNamesAndNulls(t *testing.T) {
	var input dto.AgentSettingsDTO
	require.NoError(t, json.Unmarshal([]byte(`{"runtime":null,"model_provider":"anthropic"}`), &input))

	encoded, err := json.Marshal(dto.FromAgentSettings(models.AgentSettings{}))

	require.NoError(t, err)
	require.Nil(t, input.Runtime)
	require.False(t, input.LimitsDisabled)
	require.JSONEq(t, `{"runtime":null,"model_provider":null,"model_id":null,"profile":null,"thinking":null,`+
		`"memory_limit_mb":null,"cpu_limit_percent":null,"limits_disabled":false}`, string(encoded))
}

func TestProfileCatalogNamesAreNeverNull(t *testing.T) {
	encoded, err := json.Marshal(dto.FromProfileCatalog(models.ProfileCatalog{}))

	require.NoError(t, err)
	require.JSONEq(t, `{"active":null,"names":[]}`, string(encoded))
}

func TestThinkingLevelIDsFollowTheRuntime(t *testing.T) {
	cases := map[string][]string{
		"claude":       {"low", "medium", "high", "xhigh", "max"},
		"cursor":       {},
		"pi":           {"off", "minimal", "low", "medium", "high", "xhigh", "max"},
		"gentle-shell": {"off", "minimal", "low", "medium", "high", "xhigh", "max"},
		"":             {"off", "minimal", "low", "medium", "high", "xhigh", "max"},
	}
	for raw, expected := range cases {
		t.Run(raw, func(t *testing.T) {
			ids, err := dto.ThinkingLevelIDs(&raw)

			require.NoError(t, err)
			require.Equal(t, expected, ids)
		})
	}
}

func TestThinkingLevelIDsWithoutRuntimeUseTheDefault(t *testing.T) {
	ids, err := dto.ThinkingLevelIDs(nil)

	require.NoError(t, err)
	require.Len(t, ids, 7)
}

func TestThinkingLevelIDsRejectUnknownRuntimeWithTheDomainMessage(t *testing.T) {
	_, err := dto.ThinkingLevelIDs(stringPointer("codex"))

	require.EqualError(t, err, "runtime de agente inválido: codex")
}

func TestRuntimeOptionsListEveryRuntimeInSelectorOrder(t *testing.T) {
	require.Equal(t, []string{"gentle-shell", "pi", "claude", "cursor"}, dto.RuntimeOptions())
}

func TestModelOptionMapsProviderAndModel(t *testing.T) {
	option := dto.FromModelRef(models.ModelRef{Provider: "openai", Model: "gpt-5"})

	require.Equal(t, dto.ModelOptionDTO{Provider: "openai", Model: "gpt-5"}, option)
}
