package inbound

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/inbound/dto"
)

func strptr(value string) *string { return &value }

func uint32ptr(value uint32) *uint32 { return &value }

// full is a settings payload with every field set, so each test shows which fields a change keeps.
func full() dto.AgentSettingsDTO {
	return dto.AgentSettingsDTO{
		Runtime:         strptr("pi"),
		ModelProvider:   strptr("anthropic"),
		ModelID:         strptr("claude-5"),
		Profile:         strptr("Claude"),
		Thinking:        strptr("high"),
		MemoryLimitMB:   uint32ptr(2048),
		CPULimitPercent: uint32ptr(150),
		LimitsDisabled:  true,
	}
}

func TestChangeRuntimeClearsTheModelAndKeepsTheRest(t *testing.T) {
	next := changeRuntime(full(), "claude")

	require.Equal(t, strptr("claude"), next.Runtime)
	require.Nil(t, next.ModelProvider)
	require.Nil(t, next.ModelID)
	require.Equal(t, strptr("Claude"), next.Profile)
	require.Equal(t, strptr("high"), next.Thinking)
	require.Equal(t, uint32ptr(2048), next.MemoryLimitMB)
	require.True(t, next.LimitsDisabled)
}

func TestEmptyRuntimeReturnsToTheDefault(t *testing.T) {
	require.Nil(t, changeRuntime(full(), "").Runtime)
}

func TestChangeModelDecodesTheOptionValue(t *testing.T) {
	next := changeModel(full(), dto.EncodeModelValue("openai", "gpt-5"))

	require.Equal(t, strptr("openai"), next.ModelProvider)
	require.Equal(t, strptr("gpt-5"), next.ModelID)
	require.Equal(t, strptr("pi"), next.Runtime)
}

func TestChangeModelWithAnInvalidOrEmptyValueClearsIt(t *testing.T) {
	for _, value := range []string{"", "anthropic-claude-5"} {
		next := changeModel(full(), value)

		require.Nil(t, next.ModelProvider, value)
		require.Nil(t, next.ModelID, value)
	}
}

func TestChangeProfileAndThinkingKeepTheModel(t *testing.T) {
	profile := changeProfile(full(), "Gentleman")
	thinking := changeThinking(full(), "")

	require.Equal(t, strptr("Gentleman"), profile.Profile)
	require.Equal(t, strptr("anthropic"), profile.ModelProvider)
	require.Nil(t, thinking.Thinking)
	require.Equal(t, strptr("claude-5"), thinking.ModelID)
}

func TestLimitChangesTreatEmptyZeroAndInvalidAsNoLimit(t *testing.T) {
	cases := map[string]*uint32{
		"":    nil,
		"0":   nil,
		"-1":  nil,
		"abc": nil,
		" 7 ": uint32ptr(7),
	}
	for value, expected := range cases {
		require.Equal(t, expected, changeMemoryLimit(full(), value).MemoryLimitMB, "memory %q", value)
		require.Equal(t, expected, changeCPULimit(full(), value).CPULimitPercent, "cpu %q", value)
	}
}

func TestLimitChangesKeepTheOtherCap(t *testing.T) {
	next := changeMemoryLimit(full(), "512")

	require.Equal(t, uint32ptr(512), next.MemoryLimitMB)
	require.Equal(t, uint32ptr(150), next.CPULimitPercent)
}

func TestLimitsSwitchInvertsIntoTheDisabledFlagAndKeepsValues(t *testing.T) {
	off := changeLimitsEnabled(full(), false)
	on := changeLimitsEnabled(full(), true)

	require.True(t, off.LimitsDisabled)
	require.False(t, on.LimitsDisabled)
	require.Equal(t, uint32ptr(2048), off.MemoryLimitMB)
}

func TestChangesDoNotMutateTheCurrentSettings(t *testing.T) {
	current := full()

	_ = changeRuntime(current, "claude")

	require.Equal(t, strptr("anthropic"), current.ModelProvider)
}
