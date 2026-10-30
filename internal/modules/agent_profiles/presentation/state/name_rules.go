package state

import (
	"context"
	"slices"

	"iworkflow/internal/core/i18n"
)

// maxNameLength is the longest profile or role name, in bytes (as the Rust check counted them).
const maxNameLength = 64

// reservedNames are the names the server rejects because they collide with JavaScript properties.
var reservedNames = []string{"__proto__", "constructor", "prototype"}

// SlugHint explains why raw is not a valid profile or role name, in the language of ctx. It returns
// "" when the name is valid. The server stays the authority; the hint only guides the user.
func SlugHint(ctx context.Context, raw string) string {
	hint, found := slugProblem(raw)
	if !found {
		return ""
	}
	return hint.T(ctx)
}

// SlugProblemFound reports whether the name breaks a rule, without choosing a language.
func SlugProblemFound(raw string) bool {
	_, found := slugProblem(raw)
	return found
}

func slugProblem(raw string) (i18n.Msg, bool) {
	switch {
	case raw == "":
		return hintEmptyName, true
	case len(raw) > maxNameLength:
		return hintNameTooLong, true
	case !startsWithASCIIAlnum(raw):
		return hintBadStart, true
	case !onlySlugCharacters(raw):
		return hintBadChars, true
	case slices.Contains(reservedNames, raw):
		return hintReserved, true
	default:
		return i18n.Msg{}, false
	}
}

func startsWithASCIIAlnum(raw string) bool {
	return isASCIIAlnum(raw[0])
}

func onlySlugCharacters(raw string) bool {
	for index := range len(raw) {
		c := raw[index]
		if !isASCIIAlnum(c) && c != '.' && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

func isASCIIAlnum(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
