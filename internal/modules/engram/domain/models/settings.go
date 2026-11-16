package models

import (
	"net"
	"net/url"
	"strings"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
)

// DefaultServerURL is the address Engram listens on with its own defaults.
const DefaultServerURL = "http://127.0.0.1:7437"

// URLSource tells where the effective server address comes from.
type URLSource string

const (
	// SourceSaved means the user saved the address in Configuración.
	SourceSaved URLSource = "saved"
	// SourceEnvironment means the address comes from IWF_ENGRAM_URL.
	SourceEnvironment URLSource = "environment"
	// SourceDefault means Engram's default address applies.
	SourceDefault URLSource = "default"
)

// ServerSettings is the effective Engram server address and where it comes from.
type ServerSettings struct {
	URL    string
	Source URLSource
}

// ParseServerURL validates an Engram server address. Surrounding spaces are ignored and trailing slashes are
// dropped. A blank value, a URL that is not http or https, one without a host, or one that carries credentials is
// invalid. The error never echoes the value.
func ParseServerURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if trimmed == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return "", engramerrors.ErrInvalidServerURL
	}
	return strings.TrimRight(trimmed, "/"), nil
}

// Port returns the explicit port of the address, or an empty string when it has none.
func (s ServerSettings) Port() string {
	parsed, err := url.Parse(s.URL)
	if err != nil {
		return ""
	}
	return parsed.Port()
}

// IsLocal reports whether the address points at this machine, the only place iworkflow may start Engram.
func (s ServerSettings) IsLocal() bool {
	parsed, err := url.Parse(s.URL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// AutoStart reports whether Engram may be started for this address: it is local, has an explicit port and was not
// set through IWF_ENGRAM_URL, which the operator manages.
func (s ServerSettings) AutoStart() bool {
	return s.Source != SourceEnvironment && s.IsLocal() && s.Port() != ""
}

// StartOutcome is what EnsureRunning did.
type StartOutcome string

const (
	// OutcomeRunning means Engram already answered, so nothing was started.
	OutcomeRunning StartOutcome = "running"
	// OutcomeStarted means Engram was off and now answers.
	OutcomeStarted StartOutcome = "started"
	// OutcomeSkipped means the address is not one iworkflow may start Engram for.
	OutcomeSkipped StartOutcome = "skipped"
)
