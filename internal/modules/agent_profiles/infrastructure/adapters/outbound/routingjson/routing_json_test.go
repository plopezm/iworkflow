package routingjson

import (
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
)

func TestParseEntryLegacyStringIsModelOnly(t *testing.T) {
	entry, err := parseEntry("p/m")

	require.NoError(t, err)
	require.Equal(t, "p/m", *entry.Model)
	require.Nil(t, entry.Thinking)
}

func TestParseEntryObjectWithOnlyThinkingIsAccepted(t *testing.T) {
	entry, err := parseEntry(map[string]any{"thinking": "low"})

	require.NoError(t, err)
	require.Nil(t, entry.Model)
	require.Equal(t, "low", *entry.Thinking)
}

func TestParseEntryRejectsBadTypesAndValues(t *testing.T) {
	for _, value := range []any{3.0, map[string]any{"model": 3.0}, map[string]any{"thinking": "ultra"}, "  "} {
		_, err := parseEntry(value)
		require.Error(t, err, value)
	}
}

func TestParseRoutingErrorNamesTheAgent(t *testing.T) {
	_, err := ParseRouting(map[string]any{"sdd-apply": map[string]any{"thinking": "ultra"}})

	require.ErrorIs(t, err, agenterrors.InvalidRouting(""))
	require.ErrorContains(t, err, "agente `sdd-apply`")
}

func TestRoutingToObjectSerializesOnlyPresentFields(t *testing.T) {
	model := "a/b"

	object := RoutingToObject(map[string]models.RoutingEntry{"x": {Model: &model}, "y": {}})

	require.Equal(t, map[string]any{"x": map[string]any{"model": "a/b"}, "y": map[string]any{}}, object)
}

func TestDecodeRejectsTrailingContent(t *testing.T) {
	_, err := Decode([]byte(`{} {}`))

	require.Error(t, err)
}
