package di

import (
	"errors"
	"strings"

	"iworkflow/internal/modules/engram/domain/models"
)

// ErrNotConfigured means IWF_ENGRAM_URL is unset or blank: the feature is off and nothing is reported.
var ErrNotConfigured = errors.New("engram: base url not configured")

// ErrInvalidBaseURL means IWF_ENGRAM_URL is set but is not an absolute http or https URL with a host.
var ErrInvalidBaseURL = errors.New("engram: base url must be an absolute http or https URL with a host")

// ResolveBaseURL validates the configured Engram API root. Surrounding spaces are ignored and trailing slashes are
// dropped. A URL that carries credentials (user or user:password before the host) is invalid. The error never echoes
// the value, which may carry credentials.
func ResolveBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrNotConfigured
	}
	baseURL, err := models.ParseServerURL(trimmed)
	if err != nil {
		return "", ErrInvalidBaseURL
	}
	return baseURL, nil
}
