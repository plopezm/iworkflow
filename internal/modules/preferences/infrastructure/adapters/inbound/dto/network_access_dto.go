// Package dto holds the data shapes that travel between the browser and the preferences module.
package dto

// AccessTargetDTO is one way to reach the server: an interface of this machine with its link and
// QR code.
type AccessTargetDTO struct {
	// Interface is the interface name, e.g. "eth0" or "tailscale0".
	Interface string `json:"interface"`
	Address   string `json:"address"`
	// AccessURL is "http://<address>:<port>", with "/?token=..." appended when a token is demanded.
	AccessURL string `json:"access_url"`
	// QRSVG is the inline SVG of the QR code for AccessURL.
	QRSVG *string `json:"qr_svg"`
	// SecureURL is the same link over HTTPS; empty when the server has no HTTPS listener.
	SecureURL string `json:"secure_url"`
	// SecureQRSVG is the inline SVG of the QR code for SecureURL.
	SecureQRSVG *string `json:"secure_qr_svg"`
}

// NetworkAccessDTO is what the settings card needs to show and toggle network access.
type NetworkAccessDTO struct {
	// Enabled is the saved choice: listen on every interface the next time the server starts.
	Enabled bool `json:"enabled"`
	// Active reports whether the running server is reachable from other machines right now.
	Active bool `json:"active"`
	// LockedByEnv is set when --addr or IWORKFLOW_ADDR pins the address, so the saved choice has no effect.
	LockedByEnv bool `json:"locked_by_env"`
	// Port is the port the running server listens on.
	Port uint16 `json:"port"`
	// ConfiguredPort is the saved choice: port to listen on the next time the server starts.
	ConfiguredPort uint16 `json:"configured_port"`
	// Targets are the interfaces other devices can use, preferred one first. Empty when no network
	// was detected or access is off. Always serialized as an array, never null.
	Targets []AccessTargetDTO `json:"targets"`
	// RequireToken is the saved choice: demand the access token from network clients.
	RequireToken bool `json:"require_token"`
	// TokenActive reports whether the running server demands the access token right now.
	TokenActive bool `json:"token_active"`
	// Token is the secret itself, only while it is in play (reachable and required).
	Token *string `json:"token"`
}

// NewNetworkAccessDTO starts from the saved state: the configured port defaults to the running
// one, the token is required and no secret or address is included.
func NewNetworkAccessDTO(enabled bool, active bool, lockedByEnv bool, port uint16) NetworkAccessDTO {
	return NetworkAccessDTO{
		Enabled:        enabled,
		Active:         active,
		LockedByEnv:    lockedByEnv,
		Port:           port,
		ConfiguredPort: port,
		Targets:        []AccessTargetDTO{},
		RequireToken:   true,
		TokenActive:    false,
		Token:          nil,
	}
}

// WithTokenState sets the saved and the running token requirement.
func (d NetworkAccessDTO) WithTokenState(requireToken bool, tokenActive bool) NetworkAccessDTO {
	d.RequireToken = requireToken
	d.TokenActive = tokenActive
	return d
}

// WithTargets keeps the addresses only while access is on or active, so a private server shows
// none.
func (d NetworkAccessDTO) WithTargets(targets []AccessTargetDTO) NetworkAccessDTO {
	if targets == nil {
		targets = []AccessTargetDTO{}
	}
	if d.Enabled || d.Active {
		d.Targets = targets
	}
	return d
}

// WithConfiguredPort sets the port that applies on the next start.
func (d NetworkAccessDTO) WithConfiguredPort(configuredPort uint16) NetworkAccessDTO {
	d.ConfiguredPort = configuredPort
	return d
}

// WithToken attaches the secret (nil when it is not in play).
func (d NetworkAccessDTO) WithToken(token *string) NetworkAccessDTO {
	d.Token = token
	return d
}
