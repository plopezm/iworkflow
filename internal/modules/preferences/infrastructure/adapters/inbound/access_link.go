package inbound

import (
	"fmt"
	"strings"

	"iworkflow/internal/modules/preferences/domain/models"
)

// BuildAccessURL returns "http://<host>:<port>/?token=<token>", or the plain address when token is
// nil. IPv6 hosts are bracketed. The access guard redirects away from the token URL right after
// storing the token in a cookie.
func BuildAccessURL(host string, port uint16, token *models.AccessToken) string {
	return BuildSchemeURL("http", host, port, token)
}

// BuildSchemeURL is BuildAccessURL for the given scheme ("http" or "https").
func BuildSchemeURL(scheme string, host string, port uint16, token *models.AccessToken) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if token == nil {
		return fmt.Sprintf("%s://%s:%d", scheme, host, port)
	}
	return fmt.Sprintf("%s://%s:%d/?token=%s", scheme, host, port, token.Value())
}
