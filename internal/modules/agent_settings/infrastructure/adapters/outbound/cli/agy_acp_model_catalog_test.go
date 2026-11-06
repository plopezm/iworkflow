package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

func TestParseAgyModelsKeepsModelID(t *testing.T) {
	result := json.RawMessage(`{"sessionId": "s", "models": {"currentModelId": "gemini-3.8-flash-high",
		"availableModels": [
			{"modelId": "gemini-3.8-flash-high", "name": "Gemini 3.8 Flash (High)"},
			{"modelId": "claude-sonnet-4-6", "name": "Claude Sonnet 4.6 (Thinking)"}
		]}}`)

	require.Equal(t, []models.ModelRef{
		{Provider: "agy", Model: "gemini-3.8-flash-high"},
		{Provider: "agy", Model: "claude-sonnet-4-6"},
	}, parseAgyModels(result))
}

func TestAgyResultWithoutModelsIsAnEmptyCatalog(t *testing.T) {
	require.Empty(t, parseAgyModels(json.RawMessage(`{"sessionId": "s"}`)))
	require.NotNil(t, parseAgyModels(nil))
	require.Empty(t, parseAgyModels(nil))
}
