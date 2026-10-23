package models

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
)

// accessTokenLength is the length of a token: 32 random bytes written as hex (64 characters).
const accessTokenLength = 64

// accessTokenBytes is the number of random bytes behind a token (about 256 bits).
const accessTokenBytes = accessTokenLength / 2

// AccessToken is the secret other devices present to use the app when it is reachable from the
// network. Its String and GoString methods never print the value.
type AccessToken struct {
	value string
}

// GenerateAccessToken draws a fresh token from the operating system's random source.
func GenerateAccessToken() (AccessToken, error) {
	raw := make([]byte, accessTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return AccessToken{}, fmt.Errorf("read random token bytes: %w", err)
	}
	return AccessToken{value: hex.EncodeToString(raw)}, nil
}

// ParseAccessToken accepts only well-formed tokens, so a damaged stored value is treated as missing.
// The result is always lowercase.
func ParseAccessToken(raw string) (AccessToken, error) {
	if len(raw) != accessTokenLength || !isHex(raw) {
		return AccessToken{}, domainerrors.ErrInvalidAccessToken
	}
	return AccessToken{value: strings.ToLower(raw)}, nil
}

// Value returns the secret itself. Only persistence and the access link should call it.
func (t AccessToken) Value() string {
	return t.value
}

// Matches compares in constant time, so the response timing reveals nothing about the token.
func (t AccessToken) Matches(candidate string) bool {
	// The zero value holds no secret and must never accept an empty candidate.
	if t.value == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(t.value), []byte(candidate)) == 1
}

// String hides the secret, so logs and test failures never print it.
func (t AccessToken) String() string {
	return "AccessToken(…)"
}

// GoString hides the secret for %#v as well.
func (t AccessToken) GoString() string {
	return t.String()
}

func isHex(raw string) bool {
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		isDigit := b >= '0' && b <= '9'
		isLetter := (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
		if !isDigit && !isLetter {
			return false
		}
	}
	return true
}
