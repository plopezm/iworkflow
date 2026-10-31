package models

// NetworkInterface is an address of this machine that other devices could use to reach the server.
type NetworkInterface struct {
	// Name is the interface name, e.g. "eth0" or "tailscale0".
	Name string
	// Address is the IPv4 address in dotted form.
	Address string
}

// ServerStatus describes the server that is running right now.
type ServerStatus struct {
	// Port is the port the running server listens on.
	Port uint16
	// TLSPort is the port of the HTTPS listener, 0 when there is none.
	TLSPort uint16
	// Exposed reports whether the running server accepts connections from other machines.
	Exposed bool
	// LockedByEnv reports whether the address is pinned by the environment, so saved choices have no
	// effect.
	LockedByEnv bool
	// TokenRequired reports whether the running server demands the access token.
	TokenRequired bool
}

// NetworkAccessStatus is everything the network access card shows: the saved choices, the running
// server and, only while it is in play, the access token.
type NetworkAccessStatus struct {
	// Enabled is the saved choice: listen on every interface after the next start.
	Enabled bool
	// Active reports whether the running server is reachable from other machines.
	Active bool
	// LockedByEnv is set when the environment pins the address.
	LockedByEnv bool
	// Port is the port the running server listens on.
	Port uint16
	// TLSPort is the port of the HTTPS listener of the running server, 0 when there is none.
	TLSPort uint16
	// ConfiguredPort is the saved port, or the running one when none was saved.
	ConfiguredPort uint16
	// Targets lists the interfaces, preferred one first. It is never filtered by access state here.
	Targets []NetworkInterface
	// RequireToken is the saved choice: network clients must present the token.
	RequireToken bool
	// TokenActive reports whether the running server demands the token right now.
	TokenActive bool
	// Token is set only while the token is in play (reachable and required), otherwise nil.
	Token *AccessToken
}
