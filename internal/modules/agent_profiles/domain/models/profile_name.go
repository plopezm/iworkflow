// Package models holds the agent_profiles entities and value objects. It is pure Go: no I/O.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

// maxSlugLength is the longest accepted profile or role name, in bytes.
const maxSlugLength = 64

// ProfileName is a validated profile name: 1-64 characters from [A-Za-z0-9._-], starting with a
// letter or digit, and not one of the JavaScript property names that the Rust version reserved.
// The zero value is not a valid name; build values with ParseProfileName.
type ProfileName struct {
	value string
}

// ParseProfileName validates raw and returns the name.
func ParseProfileName(raw string) (ProfileName, error) {
	if !isValidSlug(raw) {
		return ProfileName{}, agenterrors.InvalidProfileName(raw)
	}
	return ProfileName{value: raw}, nil
}

// String returns the name as stored in profiles.json.
func (n ProfileName) String() string {
	return n.value
}

// isValidSlug checks the format shared by profile and role names.
func isValidSlug(raw string) bool {
	if raw == "" || len(raw) > maxSlugLength || isReservedName(raw) {
		return false
	}
	if !isASCIIAlnum(raw[0]) {
		return false
	}
	for i := range len(raw) {
		c := raw[i]
		if !isASCIIAlnum(c) && c != '.' && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// isReservedName reports the names that collide with JavaScript object properties.
func isReservedName(raw string) bool {
	switch raw {
	case "__proto__", "constructor", "prototype":
		return true
	default:
		return false
	}
}

func isASCIIAlnum(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
