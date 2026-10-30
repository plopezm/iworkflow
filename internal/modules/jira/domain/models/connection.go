//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
)

// Connection is the single Jira Server/Data Center connection: base URL plus personal access token.
//
// The token is never printed: String and GoString mask it, so logging or formatting a connection
// cannot leak it.
type Connection struct {
	baseURL            string
	token              string
	acceptInvalidCerts bool
	lastSyncAt         *time.Time
	lastSyncError      *string
}

// NewConnection validates and normalises user input: the URL is trimmed and loses its trailing slashes.
func NewConnection(baseURL string, token string, acceptInvalidCerts bool) (Connection, error) {
	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Connection{}, err
	}
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return Connection{}, jiraerrors.EmptyToken()
	}
	return Connection{baseURL: normalized, token: trimmedToken, acceptInvalidCerts: acceptInvalidCerts}, nil
}

// RestoreConnection rebuilds a connection stored by a previous version. The stored values were
// validated when they were saved, so nothing is checked again.
func RestoreConnection(baseURL string, token string, acceptInvalidCerts bool, lastSyncAt *time.Time, lastSyncError *string) Connection {
	return Connection{
		baseURL:            baseURL,
		token:              token,
		acceptInvalidCerts: acceptInvalidCerts,
		lastSyncAt:         copyTime(lastSyncAt),
		lastSyncError:      copyString(lastSyncError),
	}
}

// BaseURL is the normalised server URL, without a trailing slash.
func (c Connection) BaseURL() string { return c.baseURL }

// Token is the personal access token. Only the outbound client should read it.
func (c Connection) Token() string { return c.token }

// AcceptInvalidCerts reports whether self-signed or internal-CA certificates are accepted.
func (c Connection) AcceptInvalidCerts() bool { return c.acceptInvalidCerts }

// LastSyncAt is the instant of the last sync attempt, or nil when there was none.
func (c Connection) LastSyncAt() *time.Time { return copyTime(c.lastSyncAt) }

// LastSyncError is the outcome of the last sync: nil on success.
func (c Connection) LastSyncError() *string { return copyString(c.lastSyncError) }

// WithSyncRecord returns a copy that carries the given sync bookkeeping.
func (c Connection) WithSyncRecord(at *time.Time, syncError *string) Connection {
	c.lastSyncAt = copyTime(at)
	c.lastSyncError = copyString(syncError)
	return c
}

// APIURL is the REST API v2 endpoint for path (e.g. `myself`, `issue/ABC-1`).
func (c Connection) APIURL(path string) string {
	return c.baseURL + "/rest/api/2/" + strings.TrimLeft(path, "/")
}

// BrowseURL is the browser URL of an issue.
func (c Connection) BrowseURL(key string) string {
	return c.baseURL + "/browse/" + key
}

// String masks the token.
func (c Connection) String() string {
	return fmt.Sprintf("Connection{baseURL:%s token:[redacted] acceptInvalidCerts:%t}", c.baseURL, c.acceptInvalidCerts)
}

// GoString masks the token for %#v too.
func (c Connection) GoString() string {
	return c.String()
}

func normalizeBaseURL(value string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	host, found := strings.CutPrefix(trimmed, "https://")
	if !found {
		host, found = strings.CutPrefix(trimmed, "http://")
	}
	if !found {
		return "", jiraerrors.InvalidURL()
	}
	if host == "" || strings.HasPrefix(host, "/") || strings.IndexFunc(host, unicode.IsSpace) >= 0 {
		return "", jiraerrors.InvalidURL()
	}
	return trimmed, nil
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
