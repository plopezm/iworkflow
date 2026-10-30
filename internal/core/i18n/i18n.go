// Package i18n holds the supported locales, the per-request locale and translatable messages.
//
// Every user-facing text is a Msg with one string per language, so a missing translation is a
// compile error (all fields are required by the keyed literal check in tests and lint). The chosen
// language lives in the `iw-lang` cookie.
package i18n

import (
	"context"
	"strings"
)

// LocaleCookie is the name of the cookie that stores the chosen language.
const LocaleCookie = "iw-lang"

// CookieMaxAgeSecs is one year, in seconds.
const CookieMaxAgeSecs = 31_536_000

// Locale is a language the interface is available in.
type Locale string

// Supported locales.
const (
	En Locale = "en"
	Es Locale = "es"
	Zh Locale = "zh"
	De Locale = "de"
	Fr Locale = "fr"
)

// All lists every supported language in the order the selector shows them.
var All = [...]Locale{En, Es, Zh, De, Fr}

// Code is the ISO 639-1 code; also the cookie value and the <html lang> attribute.
func (l Locale) Code() string { return string(l) }

// NativeName is the name of the language written in that language.
func (l Locale) NativeName() string {
	switch l {
	case Es:
		return "Español"
	case Zh:
		return "中文"
	case De:
		return "Deutsch"
	case Fr:
		return "Français"
	default:
		return "English"
	}
}

// Parse parses a language code; unknown codes yield false.
func Parse(value string) (Locale, bool) {
	trimmed := strings.TrimSpace(value)
	for _, locale := range All {
		if string(locale) == trimmed {
			return locale, true
		}
	}
	return "", false
}

// FromCookieHeader extracts the chosen language from a Cookie header value.
func FromCookieHeader(header string) (Locale, bool) {
	for _, pair := range strings.Split(header, ";") {
		name, value, found := strings.Cut(pair, "=")
		if !found || strings.TrimSpace(name) != LocaleCookie {
			continue
		}
		if locale, ok := Parse(value); ok {
			return locale, true
		}
	}
	return "", false
}

// State is the language in force plus whether the user ever picked one.
type State struct {
	Locale Locale
	// Chosen is false until the user picks a language; it drives the first-run prompt.
	Chosen bool
}

// Detect reads the language from a Cookie header value; English when none was chosen.
func Detect(cookieHeader string) State {
	if locale, ok := FromCookieHeader(cookieHeader); ok {
		return State{Locale: locale, Chosen: true}
	}
	return State{Locale: En}
}

type stateKey struct{}

// WithState stores the request language in the context (request-scoped data only).
func WithState(ctx context.Context, state State) context.Context {
	return context.WithValue(ctx, stateKey{}, state)
}

// StateFrom returns the request language, English when the context carries none.
func StateFrom(ctx context.Context) State {
	if state, ok := ctx.Value(stateKey{}).(State); ok {
		return state
	}
	return State{Locale: En}
}

// Msg is a user-facing text in every supported language. Placeholders are written {name}.
type Msg struct {
	En, Es, Zh, De, Fr string
}

// In returns the text in a given language.
func (m Msg) In(locale Locale) string {
	switch locale {
	case Es:
		return m.Es
	case Zh:
		return m.Zh
	case De:
		return m.De
	case Fr:
		return m.Fr
	default:
		return m.En
	}
}

// T returns the text in the language of the request carried by ctx.
func (m Msg) T(ctx context.Context) string { return m.In(StateFrom(ctx).Locale) }

// Fmt returns the request-language text with every {name} replaced by its value.
func (m Msg) Fmt(ctx context.Context, args map[string]string) string {
	text := m.T(ctx)
	for name, value := range args {
		text = strings.ReplaceAll(text, "{"+name+"}", value)
	}
	return text
}
