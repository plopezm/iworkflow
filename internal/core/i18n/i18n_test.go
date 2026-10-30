package i18n_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
)

var greeting = i18n.Msg{
	En: "Hello {name}", Es: "Hola {name}", Zh: "你好 {name}", De: "Hallo {name}", Fr: "Bonjour {name}",
}

func TestParseAcceptsEverySupportedCodeAndRejectsOthers(t *testing.T) {
	for _, locale := range i18n.All {
		got, ok := i18n.Parse(locale.Code())
		require.True(t, ok)
		require.Equal(t, locale, got)
	}
	_, ok := i18n.Parse("pt")
	require.False(t, ok)
	_, ok = i18n.Parse("")
	require.False(t, ok)
}

func TestCookieHeaderYieldsTheChosenLanguage(t *testing.T) {
	cases := map[string]struct {
		locale i18n.Locale
		ok     bool
	}{
		"a=1; iw-lang=de; b=2": {i18n.De, true},
		"iw-lang=zh":           {i18n.Zh, true},
		"iw-lang=xx":           {"", false},
		"other-iw-lang=fr":     {"", false},
		"":                     {"", false},
	}
	for header, want := range cases {
		got, ok := i18n.FromCookieHeader(header)
		require.Equal(t, want.ok, ok, header)
		require.Equal(t, want.locale, got, header)
	}
}

func TestDefaultLanguageIsEnglishAndNotChosen(t *testing.T) {
	state := i18n.Detect("")
	require.Equal(t, i18n.En, state.Locale)
	require.False(t, state.Chosen)
	require.True(t, i18n.Detect("iw-lang=fr").Chosen)
}

func TestMsgReturnsTheTextOfEachLanguage(t *testing.T) {
	require.Equal(t, "Hello {name}", greeting.In(i18n.En))
	require.Equal(t, "Hola {name}", greeting.In(i18n.Es))
	require.Equal(t, "你好 {name}", greeting.In(i18n.Zh))
	require.Equal(t, "Hallo {name}", greeting.In(i18n.De))
	require.Equal(t, "Bonjour {name}", greeting.In(i18n.Fr))
}

func TestMsgTUsesEnglishWithoutLocaleInContext(t *testing.T) {
	require.Equal(t, "Hello {name}", greeting.T(context.Background()))
}

func TestMsgTAndFmtFollowTheContextLocale(t *testing.T) {
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Fr, Chosen: true})
	require.Equal(t, "Bonjour {name}", greeting.T(ctx))
	require.Equal(t, "Bonjour Ana", greeting.Fmt(ctx, map[string]string{"name": "Ana"}))
}

func TestFmtReplacesEveryOccurrenceOfEachPlaceholder(t *testing.T) {
	twice := i18n.Msg{En: "{a}-{b}-{a}", Es: "{a}-{b}-{a}", Zh: "{a}-{b}-{a}", De: "{a}-{b}-{a}", Fr: "{a}-{b}-{a}"}
	require.Equal(t, "1-2-1", twice.Fmt(context.Background(), map[string]string{"a": "1", "b": "2"}))
}
