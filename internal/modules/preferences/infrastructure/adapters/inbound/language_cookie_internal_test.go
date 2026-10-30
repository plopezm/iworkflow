package inbound

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/preferences/domain/models"
)

func storedLanguage(t *testing.T, code string) *models.UILanguage {
	t.Helper()
	language, err := models.ParseUILanguage(code)
	require.NoError(t, err)
	return &language
}

func localePtr(locale i18n.Locale) *i18n.Locale {
	return &locale
}

func TestDatabaseLanguageWinsOverAMissingOrDifferentCookie(t *testing.T) {
	require.Equal(t, syncDecision{kind: syncRestore, code: "de"}, reconcile(storedLanguage(t, "de"), nil))
	require.Equal(t, syncDecision{kind: syncRestore, code: "de"}, reconcile(storedLanguage(t, "de"), localePtr(i18n.Es)))
}

func TestMatchingCookieAndDatabaseNeedNothing(t *testing.T) {
	require.Equal(t, syncDecision{kind: syncKeep}, reconcile(storedLanguage(t, "de"), localePtr(i18n.De)))
	require.Equal(t, syncDecision{kind: syncKeep}, reconcile(nil, nil))
}

func TestACookieChosenBeforeStorageIsPersisted(t *testing.T) {
	require.Equal(t, syncDecision{kind: syncPersist, code: "fr"}, reconcile(nil, localePtr(i18n.Fr)))
}

func TestCookieHeaderReplacesOnlyTheLanguagePair(t *testing.T) {
	require.Equal(t, "a=1; b=2; iw-lang=de", cookieHeaderWithLanguage("a=1; iw-lang=es; b=2", "de"))
	require.Equal(t, "iw-lang=fr", cookieHeaderWithLanguage("", "fr"))
	require.Equal(t, "other-iw-lang=x; iw-lang=zh", cookieHeaderWithLanguage("other-iw-lang=x", "zh"))
}

func TestRewrittenHeaderIsReadBackAsTheStoredLanguage(t *testing.T) {
	header := cookieHeaderWithLanguage("iw-lang=es; a=1", "zh")

	locale, ok := i18n.FromCookieHeader(header)

	require.True(t, ok)
	require.Equal(t, i18n.Zh, locale)
}

func TestLanguageSetCookieMatchesTheRustValue(t *testing.T) {
	require.Equal(t, "iw-lang=de; Path=/; Max-Age=31536000; SameSite=Lax", languageSetCookie("de"))
}
