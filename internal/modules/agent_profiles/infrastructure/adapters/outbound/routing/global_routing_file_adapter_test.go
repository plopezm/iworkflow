package routing

import (
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
)

func TestParseModelsAcceptsMixedLegacyAndObjectEntries(t *testing.T) {
	routing, err := parseModels([]byte(`{"a":"p/m","b":{"thinking":"low"}}`))

	require.NoError(t, err)
	require.Equal(t, "p/m", *routing["a"].Model)
	require.Equal(t, "low", *routing["b"].Thinking)
}

func TestParseModelsRejectsNonObjectRoot(t *testing.T) {
	_, err := parseModels([]byte("[]"))

	require.ErrorIs(t, err, agenterrors.Store(""))
}

func TestSerializeModelsEndsWithNewlineAndRejectsInvalidEntry(t *testing.T) {
	routing := map[string]models.RoutingEntry{"a": {}}

	content, err := serializeModels(routing)
	require.NoError(t, err)
	require.Equal(t, "{\n  \"a\": {}\n}\n", content)

	thinking := "ultra"
	routing["b"] = models.RoutingEntry{Thinking: &thinking}
	_, err = serializeModels(routing)
	require.ErrorIs(t, err, agenterrors.InvalidRouting(""))
}
