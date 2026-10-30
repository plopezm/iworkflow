package inbound

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/domain/models"
)

func TestMappingKeepsTargetsOnlyWhileAccessIsOnOrActive(t *testing.T) {
	interfaces := []models.NetworkInterface{{Name: "eth0", Address: "192.168.1.5"}}

	private := toNetworkAccessDTO(models.NetworkAccessStatus{Port: 3000, Targets: interfaces})
	enabled := toNetworkAccessDTO(models.NetworkAccessStatus{Enabled: true, Port: 3000, Targets: interfaces})

	require.Empty(t, private.Targets)
	require.Len(t, enabled.Targets, 1)
	require.Equal(t, "http://192.168.1.5:3000", enabled.Targets[0].AccessURL)
}

func TestMappingEmbedsTheTokenInEveryLinkAndExposesIt(t *testing.T) {
	token, err := models.ParseAccessToken(strings.Repeat("ab", 32))
	require.NoError(t, err)

	built := toNetworkAccessDTO(models.NetworkAccessStatus{
		Enabled:      true,
		Port:         3000,
		Targets:      []models.NetworkInterface{{Name: "eth0", Address: "10.0.0.2"}},
		RequireToken: true,
		Token:        &token,
	})

	require.NotNil(t, built.Token)
	require.Equal(t, token.Value(), *built.Token)
	require.Equal(t, "http://10.0.0.2:3000/?token="+token.Value(), built.Targets[0].AccessURL)
	require.NotNil(t, built.Targets[0].QRSVG)
}

func TestMappingWithoutTokenLeavesNoSecretAndPlainLinks(t *testing.T) {
	built := toNetworkAccessDTO(models.NetworkAccessStatus{
		Enabled: true,
		Port:    3000,
		Targets: []models.NetworkInterface{{Name: "eth0", Address: "10.0.0.2"}},
	})

	require.Nil(t, built.Token)
	require.Equal(t, "http://10.0.0.2:3000", built.Targets[0].AccessURL)
}

func TestMappingCarriesTheSavedAndRunningPorts(t *testing.T) {
	built := toNetworkAccessDTO(models.NetworkAccessStatus{Port: 3000, ConfiguredPort: 9100, RequireToken: false, TokenActive: true})

	require.Equal(t, uint16(3000), built.Port)
	require.Equal(t, uint16(9100), built.ConfiguredPort)
	require.False(t, built.RequireToken)
	require.True(t, built.TokenActive)
}
