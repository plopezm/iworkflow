// Package models holds the entities and value objects of the code_hosting module.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"strings"

	"iworkflow/internal/modules/code_hosting/domain/errors"
)

// HostingProvider is the platform that hosts a workspace repository. Its value is the stable
// identifier used in persistence and transport.
type HostingProvider string

const (
	// GitLab is the GitLab REST API v4 (self-hosted or gitlab.com).
	GitLab HostingProvider = "gitlab"
	// GitHub is the GitHub REST API v3 (github.com or Enterprise).
	GitHub HostingProvider = "github"
)

// AllHostingProviders lists every provider in declaration order.
func AllHostingProviders() []HostingProvider {
	return []HostingProvider{GitLab, GitHub}
}

// Label is the name shown to the user.
func (p HostingProvider) Label() string {
	switch p {
	case GitLab:
		return "GitLab"
	case GitHub:
		return "GitHub"
	}
	return string(p)
}

// ParseHostingProvider accepts the identifier ignoring case and surrounding whitespace.
func ParseHostingProvider(raw string) (HostingProvider, error) {
	wanted := asciiLower(strings.TrimSpace(raw))
	for _, provider := range AllHostingProviders() {
		if string(provider) == wanted {
			return provider, nil
		}
	}
	return "", errors.UnknownProvider(raw)
}

// asciiLower lowercases only ASCII letters, like Rust's to_ascii_lowercase.
func asciiLower(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, value)
}
