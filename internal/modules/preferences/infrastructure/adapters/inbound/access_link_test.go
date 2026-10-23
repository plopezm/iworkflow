package inbound_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound"
)

func TestAccessURLEmbedsTheTokenInTheQuery(t *testing.T) {
	token, err := models.ParseAccessToken(strings.Repeat("ab", 32))
	require.NoError(t, err)

	require.Equal(t,
		"http://192.168.1.5:3000/?token="+token.Value(),
		inbound.BuildAccessURL("192.168.1.5", 3000, &token),
	)
}

func TestAccessURLWithoutTokenIsThePlainAddress(t *testing.T) {
	require.Equal(t, "http://192.168.1.5:3000", inbound.BuildAccessURL("192.168.1.5", 3000, nil))
}

func TestIPv6HostsAreBracketed(t *testing.T) {
	require.Equal(t, "http://[fd00::1]:9000", inbound.BuildAccessURL("fd00::1", 9000, nil))
	require.Equal(t, "http://[fd00::1]:9000", inbound.BuildAccessURL("[fd00::1]", 9000, nil))
}
