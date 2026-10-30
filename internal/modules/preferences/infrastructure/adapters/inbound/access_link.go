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
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if token == nil {
		return fmt.Sprintf("http://%s:%d", host, port)
	}
	return fmt.Sprintf("http://%s:%d/?token=%s", host, port, token.Value())
}
