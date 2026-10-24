package agent_profiles_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/exchange"
)

const gentlePiExportFixture = "../../fixtures/gentle_pi_profile.export.json"

func sampleExchangeProfile(t *testing.T) models.Profile {
	t.Helper()
	name, err := models.ParseProfileName("Claude")
	require.NoError(t, err)
	model := "a/b"
	high := "high"
	other := "a/c"
	return models.NewProfile(name, map[string]models.RoutingEntry{
		"orchestrator": {Model: &model, Thinking: &high},
		"sdd-apply":    {Model: &other},
	})
}

func TestExchangeSerializeThenParseRoundTrips(t *testing.T) {
	adapter := exchange.NewProfileExchangeJSONAdapter()
	raw, err := adapter.SerializeProfile(context.Background(), sampleExchangeProfile(t))
	require.NoError(t, err)

	require.Contains(t, raw, `"kind": "gentle-pi.agent_model_profile"`)
	parsed, err := adapter.ParseProfile(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, sampleExchangeProfile(t), parsed)
}

func TestExchangeRejectsInvalidDocuments(t *testing.T) {
	adapter := exchange.NewProfileExchangeJSONAdapter()
	cases := []string{
		"not json",
		`{"kind":"other","version":1,"name":"A","config":{}}`,
		`{"kind":"gentle-pi.agent_model_profile","version":2,"name":"A","config":{}}`,
		`{"kind":"gentle-pi.agent_model_profile","version":1,"name":"bad name","config":{}}`,
		`{"kind":"gentle-pi.agent_model_profile","version":1,"name":"A","config":{"x":{"thinking":"ultra"}}}`,
		`{"kind":"gentle-pi.agent_model_profile","version":1,"name":"A","config":[]}`,
	}
	for _, raw := range cases {
		_, err := adapter.ParseProfile(context.Background(), raw)
		require.ErrorIs(t, err, agenterrors.InvalidImport(""), raw)
	}
}

func TestExchangeParsesGentlePiExportFixture(t *testing.T) {
	raw, err := os.ReadFile(gentlePiExportFixture)
	require.NoError(t, err)
	adapter := exchange.NewProfileExchangeJSONAdapter()

	profile, err := adapter.ParseProfile(context.Background(), string(raw))

	require.NoError(t, err)
	require.Equal(t, "Claude", profile.Name.String())
	require.Len(t, profile.Routing, 6)
}
