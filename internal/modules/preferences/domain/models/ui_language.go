package models

import (
	"strings"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
)

// UILanguage is the interface language the user picked. Only supported codes can be built, so a
// value always holds one of them. The list mirrors the locales of internal/core/i18n.
type UILanguage struct {
	code string
}

// ParseUILanguage accepts a supported language code, ignoring surrounding spaces.
func ParseUILanguage(raw string) (UILanguage, error) {
	code := strings.TrimSpace(raw)
	if !isSupportedLanguage(code) {
		return UILanguage{}, &domainerrors.UnsupportedLanguageError{Code: code}
	}
	return UILanguage{code: code}, nil
}

// Code returns the ISO 639-1 code of the language.
func (l UILanguage) Code() string {
	return l.code
}

func isSupportedLanguage(code string) bool {
	switch code {
	case "en", "es", "zh", "de", "fr":
		return true
	default:
		return false
	}
}
