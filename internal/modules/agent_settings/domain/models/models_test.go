package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

func TestParseAgentRuntimeRoundTripsEveryRuntime(t *testing.T) {
	for _, runtime := range models.AllRuntimes() {
		parsed, err := models.ParseAgentRuntime(string(runtime))

		require.NoError(t, err)
		require.Equal(t, runtime, parsed)
	}
}

func TestParseAgentRuntimeRejectsUnknownRuntimeWithItsMessage(t *testing.T) {
	_, err := models.ParseAgentRuntime("codex")

	require.EqualError(t, err, "runtime de agente inválido: codex")
	require.ErrorIs(t, err, agenterrors.ErrInvalidRuntime)
}

func TestClaudeAndCursorAreSelectableRuntimes(t *testing.T) {
	claude, err := models.ParseAgentRuntime("claude")
	require.NoError(t, err)
	cursor, err := models.ParseAgentRuntime("cursor")
	require.NoError(t, err)

	require.Equal(t, models.RuntimeClaude, claude)
	require.Equal(t, "claude", string(claude))
	require.Equal(t, models.RuntimeCursor, cursor)
	require.Equal(t, "cursor", string(cursor))
	require.Contains(t, models.AllRuntimes(), models.RuntimeCursor)
	require.Contains(t, models.AllRuntimes(), models.RuntimeClaude)
}

func TestRuntimeOrDefaultFallsBackToGentleShell(t *testing.T) {
	pi := models.RuntimePi

	require.Equal(t, models.RuntimeGentleShell, models.RuntimeOrDefault(nil))
	require.Equal(t, models.RuntimePi, models.RuntimeOrDefault(&pi))
}

func TestParseThinkingLevelRoundTripsEveryLevel(t *testing.T) {
	for _, level := range models.AllThinkingLevels() {
		parsed, err := models.ParseThinkingLevel(string(level))

		require.NoError(t, err)
		require.Equal(t, level, parsed)
	}
}

func TestParseThinkingLevelRejectsInvalidLevelWithItsMessage(t *testing.T) {
	_, err := models.ParseThinkingLevel("ultra")

	require.EqualError(t, err, "nivel de razonamiento inválido: ultra")
	require.ErrorIs(t, err, agenterrors.ErrInvalidThinkingLevel)
}

func TestClaudeSupportsOnlyLowToMax(t *testing.T) {
	require.Equal(t, []models.ThinkingLevel{
		models.ThinkingLow, models.ThinkingMedium, models.ThinkingHigh, models.ThinkingXHigh, models.ThinkingMax,
	}, models.SupportedBy(models.RuntimeClaude))
	require.False(t, models.ThinkingOff.IsSupportedBy(models.RuntimeClaude))
	require.False(t, models.ThinkingMinimal.IsSupportedBy(models.RuntimeClaude))
	require.True(t, models.ThinkingXHigh.IsSupportedBy(models.RuntimeClaude))
}

func TestPiAndGentleShellKeepAllSevenLevels(t *testing.T) {
	for _, runtime := range []models.AgentRuntime{models.RuntimePi, models.RuntimeGentleShell} {
		require.Equal(t, models.AllThinkingLevels(), models.SupportedBy(runtime))
		for _, level := range models.AllThinkingLevels() {
			require.True(t, level.IsSupportedBy(runtime))
		}
	}
}

func TestCursorHasNoThinkingLevelsAndTheListIsNeverNil(t *testing.T) {
	levels := models.SupportedBy(models.RuntimeCursor)

	require.NotNil(t, levels)
	require.Empty(t, levels)
	require.False(t, models.ThinkingHigh.IsSupportedBy(models.RuntimeCursor))
}

func TestParseProfileNameAcceptsValidNames(t *testing.T) {
	for _, raw := range []string{"Claude", "a", "gpt-5.1_test", strings.Repeat("a", 64)} {
		name, err := models.ParseProfileName(raw)

		require.NoError(t, err, raw)
		require.Equal(t, raw, name.String())
	}
}

func TestParseProfileNameRejectsInvalidNames(t *testing.T) {
	cases := map[string]string{
		"empty":                "",
		"leading dash":         "-claude",
		"leading dot":          ".claude",
		"space":                "claude profile",
		"slash":                "claude/5",
		"longer than 64":       strings.Repeat("a", 65),
		"non ascii letter":     "ñandú",
		"non ascii first byte": "é",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := models.ParseProfileName(raw)

			require.ErrorIs(t, err, agenterrors.ErrInvalidProfileName)
			require.EqualError(t, err, "nombre de perfil inválido: "+raw)
		})
	}
}
