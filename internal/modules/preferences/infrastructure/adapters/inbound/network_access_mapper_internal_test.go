package inbound

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/domain/models"
)

func TestAccessTargetsLinkOverHTTPSWhenTLSIsOn(t *testing.T) {
	status := models.NetworkAccessStatus{Port: 9000, TLSPort: 9000, Targets: []models.NetworkInterface{{Name: "eth0", Address: "192.168.1.5"}}}

	targets := toAccessTargets(status)

	require.Len(t, targets, 1)
	require.Equal(t, "https://192.168.1.5:9000", targets[0].AccessURL)
	require.Empty(t, targets[0].SecureURL, "there is no separate secure link: the only link is HTTPS")
}

func TestAccessTargetsLinkOverHTTPOnlyWhenTLSIsOff(t *testing.T) {
	status := models.NetworkAccessStatus{Port: 3000, Targets: []models.NetworkInterface{{Name: "eth0", Address: "192.168.1.5"}}}

	targets := toAccessTargets(status)

	require.Equal(t, "http://192.168.1.5:3000", targets[0].AccessURL)
}
