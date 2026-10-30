package models

import agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"

// maxProfileNameLength is the longest accepted profile name, in bytes.
const maxProfileNameLength = 64

// ProfileName is a validated agent profile name: 1-64 characters from `[A-Za-z0-9._-]`, starting
// with a letter or digit (the format of `gentle-pi.agent_model_profile_pin`).
type ProfileName struct {
	value string
}

// ParseProfileName validates a profile name.
func ParseProfileName(raw string) (ProfileName, error) {
	if raw == "" || len(raw) > maxProfileNameLength || !isASCIIAlphanumeric(raw[0]) {
		return ProfileName{}, agenterrors.InvalidProfileName(raw) //nolint:wrapcheck // domain constructor returns a domain error
	}
	for index := range len(raw) {
		if !isProfileNameByte(raw[index]) {
			return ProfileName{}, agenterrors.InvalidProfileName(raw) //nolint:wrapcheck // domain constructor returns a domain error
		}
	}
	return ProfileName{value: raw}, nil
}

// String returns the profile name.
func (p ProfileName) String() string {
	return p.value
}

func isProfileNameByte(value byte) bool {
	return isASCIIAlphanumeric(value) || value == '.' || value == '_' || value == '-'
}

func isASCIIAlphanumeric(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || (value >= '0' && value <= '9')
}
