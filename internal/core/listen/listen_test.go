package listen

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustAddrPort(t *testing.T, value string) netip.AddrPort {
	t.Helper()
	addr, err := netip.ParseAddrPort(value)
	require.NoError(t, err)
	return addr
}

func mustPort(port uint16) *uint16 {
	return &port
}

func TestResolveListenAddrStaysOnConfiguredHostByDefault(t *testing.T) {
	loopback := mustAddrPort(t, "127.0.0.1:3000")

	got := ResolveListenAddr(loopback, false, false, nil)

	assert.Equal(t, loopback, got)
}

func TestResolveListenAddrOptingInBindsEveryIPv4InterfaceKeepingThePort(t *testing.T) {
	got := ResolveListenAddr(mustAddrPort(t, "127.0.0.1:3000"), false, true, nil)

	assert.Equal(t, mustAddrPort(t, "0.0.0.0:3000"), got)
}

func TestResolveListenAddrSavedPortReplacesTheConfiguredOneInBothModes(t *testing.T) {
	loopback := mustAddrPort(t, "127.0.0.1:3000")

	assert.Equal(t, mustAddrPort(t, "127.0.0.1:9100"), ResolveListenAddr(loopback, false, false, mustPort(9100)))
	assert.Equal(t, mustAddrPort(t, "0.0.0.0:9100"), ResolveListenAddr(loopback, false, true, mustPort(9100)))
}

func TestResolveListenAddrEnvironmentAddressWinsOverThePreference(t *testing.T) {
	pinned := mustAddrPort(t, "127.0.0.1:9100")

	got := ResolveListenAddr(pinned, true, true, mustPort(9500))

	assert.Equal(t, pinned, got)
}

func TestInfoExposureFollowsTheBoundIP(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		exposed bool
	}{
		{"unspecified address is exposed", "0.0.0.0:3000", true},
		{"loopback address is local", "127.0.0.1:3000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := Info{Addr: mustAddrPort(t, tt.addr), TokenRequired: true}

			assert.Equal(t, tt.exposed, info.IsExposed())
		})
	}
}

func candidate(name string, ip string, up bool) candidateInterface {
	return candidateInterface{Name: name, IP: netip.MustParseAddr(ip), Up: up}
}

func TestReachableInterfacesKeepsEthernetAndTailscaleAddresses(t *testing.T) {
	found := reachableInterfaces([]candidateInterface{
		candidate("eth0", "192.168.1.5", true),
		candidate("tailscale0", "100.64.0.7", true),
	})

	names := make([]string, 0, len(found))
	for _, iface := range found {
		names = append(names, iface.Name)
	}
	assert.Equal(t, []string{"eth0", "tailscale0"}, names)
}

func TestReachableInterfacesDropsLoopbackLinkLocalDownAndDuplicateAddresses(t *testing.T) {
	found := reachableInterfaces([]candidateInterface{
		candidate("lo", "127.0.0.1", true),
		candidate("wlan0", "169.254.3.3", true),
		candidate("eth1", "10.0.0.2", false),
		candidate("eth0", "192.168.1.5", true),
		candidate("eth0:1", "192.168.1.5", true),
	})

	require.Len(t, found, 1)
	assert.Equal(t, "eth0", found[0].Name)
}

func TestReachableInterfacesDropsUnspecifiedAddress(t *testing.T) {
	found := reachableInterfaces([]candidateInterface{
		candidate("any", "0.0.0.0", true),
	})

	assert.Empty(t, found)
}
