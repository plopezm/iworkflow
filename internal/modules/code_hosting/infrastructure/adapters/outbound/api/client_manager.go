// Package api implements the code hosting client port over the REST APIs of GitLab and GitHub.
package api

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

const (
	requestTimeout = 30 * time.Second
	connectTimeout = 10 * time.Second
)

// ClientManager owns the HTTP clients shared by every provider client, so connections and TLS
// sessions are reused instead of opened per request. There are two clients: one with strict TLS
// verification and one that accepts untrusted certificates, because that is a property of the client.
type ClientManager struct {
	strict   *http.Client
	insecure *http.Client
}

// NewClientManager builds both clients.
func NewClientManager() *ClientManager {
	return &ClientManager{strict: newHTTPClient(false), insecure: newHTTPClient(true)}
}

// Client returns the shared client for the TLS mode of an instance.
func (m *ClientManager) Client(insecureTLS bool) *http.Client {
	if insecureTLS {
		return m.insecure
	}
	return m.strict
}

// newHTTPClient applies the timeouts and the no-redirect policy. Proxies come from the environment,
// as they do for the rest of the application.
func newHTTPClient(insecureTLS bool) *http.Client {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: connectTimeout}).DialContext,
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: connectTimeout,
		// Only the instance the user marked as insecure gets this client, never the default one.
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: insecureTLS},
	}
	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
		// The token travels in a header: following a redirect could hand it to another host, so the
		// redirect response itself is returned and reported.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
