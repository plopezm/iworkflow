package exchange

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
)

func TestParseEnvelopeAcceptsMinimalEnvelope(t *testing.T) {
	profile, err := parseEnvelope(`{"kind":"gentle-pi.agent_model_profile","version":1,"name":"A","config":{"x":{"model":"p/m"}}}`)

	require.NoError(t, err)
	require.Equal(t, "A", profile.Name.String())
	require.Equal(t, "p/m", *profile.Routing["x"].Model)
}

func TestParseEnvelopeRejectsMissingNameAndWrongVersion(t *testing.T) {
	_, err := parseEnvelope(`{"kind":"gentle-pi.agent_model_profile","version":1,"config":{}}`)
	require.Equal(t, agenterrors.InvalidImport("falta `name`"), err)

	_, err = parseEnvelope(`{"kind":"gentle-pi.agent_model_profile","version":"1","name":"A","config":{}}`)
	require.Equal(t, agenterrors.InvalidImport("version debe ser 1"), err)

	_, err = parseEnvelope(`{"kind":"gentle-pi.agent_model_profile","version":1.0,"name":"A","config":{}}`)
	require.Equal(t, agenterrors.InvalidImport("version debe ser 1"), err)
}

func TestSerializeProfileValidatesRouting(t *testing.T) {
	blank := "  "
	name, err := models.ParseProfileName("A")
	require.NoError(t, err)
	profile := models.NewProfile(name, map[string]models.RoutingEntry{"x": {Model: &blank}})

	_, err = NewProfileExchangeJSONAdapter().SerializeProfile(context.Background(), profile)

	require.ErrorIs(t, err, agenterrors.InvalidRouting(""))
}
