//go:build unix

package listen

import (
	"context"
	"net"
	"net/netip"
)

// localNetworkInterfaces enumerates every IPv4 address of this machine. The context is unused
// because the OS enumeration is local and non-blocking.
func localNetworkInterfaces(_ context.Context) ([]NetworkInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, wrapInterfaceError(err)
	}
	var candidates []candidateInterface
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, wrapInterfaceError(err)
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipNet.IP.To4())
			if !ok {
				continue
			}
			candidates = append(candidates, candidateInterface{
				Name: iface.Name,
				IP:   ip,
				Up:   iface.Flags&net.FlagUp != 0,
			})
		}
	}
	return reachableInterfaces(candidates), nil
}
