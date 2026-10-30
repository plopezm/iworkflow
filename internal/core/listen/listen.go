// Package listen decides the address the server binds to: loopback by default, every interface
// when the user opts in.
package listen

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
)

// PrimarySiteAddrVar is the preferred environment variable for the bind address.
const PrimarySiteAddrVar string = "IWORKFLOW_ADDR"

// LegacySiteAddrVar was kept from Leptos for backwards compatibility.
const LegacySiteAddrVar string = "LEPTOS_SITE_ADDR"

// SiteAddrVar points to PrimarySiteAddrVar.
const SiteAddrVar string = PrimarySiteAddrVar

// LookupSiteAddr checks IWORKFLOW_ADDR first, falling back to LEPTOS_SITE_ADDR.
func LookupSiteAddr() (string, bool) {
	if val, ok := os.LookupEnv(PrimarySiteAddrVar); ok && val != "" {
		return val, true
	}
	if val, ok := os.LookupEnv(LegacySiteAddrVar); ok && val != "" {
		return val, true
	}
	return "", false
}

// routeProbe is only used to ask the OS which interface it would route through. Nothing is sent.
var routeProbe netip.AddrPort = netip.AddrPortFrom(netip.AddrFrom4([4]byte{192, 0, 2, 1}), 9)

// Info describes where the running server listens and what decided it.
type Info struct {
	Addr netip.AddrPort
	// LockedByEnv means LEPTOS_SITE_ADDR was set, so the saved preference is ignored.
	LockedByEnv bool
	// TokenRequired means the running server demands the access token (only meaningful while exposed).
	TokenRequired bool
}

// IsExposed reports whether the server accepts connections from other machines.
func (i Info) IsExposed() bool {
	return !i.Addr.Addr().IsLoopback()
}

// ResolveListenAddr chooses the bind address. An explicit environment address always wins.
// Otherwise a saved port (if any) replaces the configured one, opting in to network access listens
// on every IPv4 interface, and the default stays on the configured host.
func ResolveListenAddr(configured netip.AddrPort, envLocked bool, networkAccess bool, savedPort *uint16) netip.AddrPort {
	if envLocked {
		return configured
	}
	port := configured.Port()
	if savedPort != nil {
		port = *savedPort
	}
	ip := configured.Addr()
	if networkAccess {
		ip = netip.IPv4Unspecified()
	}
	return netip.AddrPortFrom(ip, port)
}

// NetworkInterface is a network interface of this machine that other devices could reach the server through.
type NetworkInterface struct {
	Name string
	IP   netip.Addr
}

// candidateInterface is one address seen on this machine, before filtering.
type candidateInterface struct {
	Name string
	IP   netip.Addr
	Up   bool
}

// LocalNetworkIP returns the address of this machine on the local network, if it has one.
func LocalNetworkIP(ctx context.Context) (netip.Addr, bool) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "udp4", routeProbe.String())
	if err != nil {
		return netip.Addr{}, false
	}
	defer func() { _ = conn.Close() }()
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, false
	}
	ip, ok := netip.AddrFromSlice(local.IP.To4())
	if !ok || ip.IsLoopback() || ip.IsUnspecified() {
		return netip.Addr{}, false
	}
	return ip, true
}

// LocalNetworkInterfaces returns the IPv4 interfaces of this machine (ethernet, Wi-Fi, Tailscale...),
// in the order the OS lists them.
func LocalNetworkInterfaces(ctx context.Context) ([]NetworkInterface, error) {
	return localNetworkInterfaces(ctx)
}

// reachableInterfaces keeps the addresses a phone could use: up, not loopback, unspecified or
// link-local, and listed once.
func reachableInterfaces(candidates []candidateInterface) []NetworkInterface {
	interfaces := make([]NetworkInterface, 0, len(candidates))
	for _, candidate := range candidates {
		usable := candidate.Up &&
			!candidate.IP.IsLoopback() &&
			!candidate.IP.IsUnspecified() &&
			!candidate.IP.IsLinkLocalUnicast()
		seen := false
		for _, known := range interfaces {
			if known.IP == candidate.IP {
				seen = true
				break
			}
		}
		if usable && !seen {
			interfaces = append(interfaces, NetworkInterface{Name: candidate.Name, IP: candidate.IP})
		}
	}
	return interfaces
}

// wrapInterfaceError adds context to an OS enumeration failure.
func wrapInterfaceError(err error) error {
	return fmt.Errorf("list network interfaces: %w", err)
}
