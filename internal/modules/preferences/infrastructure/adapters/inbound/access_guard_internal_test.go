package inbound

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/preferences/domain/models"
)

func headerWith(name string, value string) http.Header {
	header := http.Header{}
	header.Set(name, value)
	return header
}

func mustToken(t *testing.T) models.AccessToken {
	t.Helper()
	token, err := models.GenerateAccessToken()
	require.NoError(t, err)
	return token
}

func TestCookieIsFoundAmongSeveral(t *testing.T) {
	header := headerWith("Cookie", "a=1; iw-token=abc; iw-lang=es")

	value, ok := cookieValue(header, AccessCookie)

	require.True(t, ok)
	require.Equal(t, "abc", value)
	_, ok = cookieValue(header, "missing")
	require.False(t, ok)
}

func TestCookieNameMustMatchExactly(t *testing.T) {
	_, ok := cookieValue(headerWith("Cookie", "x-iw-token=abc"), AccessCookie)

	require.False(t, ok)
}

func TestCookieIsReadAcrossSeveralCookieHeaders(t *testing.T) {
	header := http.Header{}
	header.Add("Cookie", "a=1")
	header.Add("Cookie", "iw-token=second")

	value, ok := cookieValue(header, AccessCookie)

	require.True(t, ok)
	require.Equal(t, "second", value)
}

func TestBearerRequiresThePrefix(t *testing.T) {
	value, ok := bearerToken(headerWith("Authorization", "Bearer abc"))
	require.True(t, ok)
	require.Equal(t, "abc", value)

	_, ok = bearerToken(headerWith("Authorization", "Basic abc"))
	require.False(t, ok)

	_, ok = bearerToken(http.Header{})
	require.False(t, ok)
}

func TestBearerPrefixIsCaseSensitive(t *testing.T) {
	_, ok := bearerToken(headerWith("Authorization", "bearer abc"))

	require.False(t, ok)
}

func TestNonVisibleHeaderValuesCountAsAbsent(t *testing.T) {
	header := http.Header{}
	header["Authorization"] = []string{"Bearer ab\x01c"}

	_, ok := bearerToken(header)

	require.False(t, ok)
}

func TestQueryTokenIsReadAndStripped(t *testing.T) {
	value, ok := queryToken("a=1&token=xyz")
	require.True(t, ok)
	require.Equal(t, "xyz", value)

	_, ok = queryToken("a=1")
	require.False(t, ok)
	_, ok = queryToken("")
	require.False(t, ok)

	rest, ok := stripTokenParam("a=1&token=xyz&b=2")
	require.True(t, ok)
	require.Equal(t, "a=1&b=2", rest)

	_, ok = stripTokenParam("token=xyz")
	require.False(t, ok)

	rest, ok = stripTokenParam("mytoken=1&token=x")
	require.True(t, ok)
	require.Equal(t, "mytoken=1", rest)
}

func TestStripDropsABareTokenKeyButKeepsSimilarNames(t *testing.T) {
	rest, ok := stripTokenParam("tokens=1&token&x=2")

	require.True(t, ok)
	require.Equal(t, "tokens=1&x=2", rest)
}

func TestLocationDropsOnlyTheToken(t *testing.T) {
	withRest := httptest.NewRequest(http.MethodGet, "/tasks?x=1&token=abc", nil)
	require.Equal(t, "/tasks?x=1", cleanLocation(withRest))

	alone := httptest.NewRequest(http.MethodGet, "/tasks?token=abc", nil)
	require.Equal(t, "/tasks", cleanLocation(alone))
}

func TestLocationNeverStartsWithTwoSlashes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "//evil.example/x?token=abc&y=1", nil)
	require.Equal(t, "/evil.example/x?y=1", cleanLocation(req))

	root := httptest.NewRequest(http.MethodGet, "/?token=abc", nil)
	require.Equal(t, "/", cleanLocation(root))
}

func TestPresentedTokensFollowPriorityOrder(t *testing.T) {
	header := headerWith("Authorization", "Bearer b")
	header.Set("Cookie", "iw-token=c")
	req := httptest.NewRequest(http.MethodGet, "/?token=q", nil)
	req.Header = header

	found := presentedTokens(req)

	sources := make([]tokenSource, 0, len(found))
	for _, presented := range found {
		sources = append(sources, presented.source)
	}
	require.Equal(t, []tokenSource{sourceBearer, sourceCookie, sourceQuery}, sources)
}

func TestLoginPageIsLocalizedAndFlagsErrors(t *testing.T) {
	page := loginPage(i18n.De, true)

	require.Contains(t, page, `lang="de"`)
	require.Contains(t, page, loginInvalid.In(i18n.De))
	require.Contains(t, page, `action="/access-login"`)
	require.Contains(t, page, `role="alert"`)
	require.NotContains(t, loginPage(i18n.En, false), `role="alert"`)
}

func TestLocaleFallsBackToEnglishForUnknownCodes(t *testing.T) {
	require.Equal(t, i18n.En, localeOf(headerWith("Cookie", "iw-lang=xx")))
	require.Equal(t, i18n.Fr, localeOf(headerWith("Cookie", "iw-lang=fr")))
}

func TestSessionCookieUsesTheCurrentToken(t *testing.T) {
	first := mustToken(t)
	guard := NewAccessGuard(first, true)
	require.True(t, strings.HasPrefix(guard.SessionCookie(), "iw-token="+first.Value()+"; HttpOnly; SameSite=Strict; Path=/; Max-Age=31536000"))

	second := mustToken(t)
	guard.SetToken(second)

	require.Contains(t, guard.SessionCookie(), second.Value())
	require.False(t, guard.accepts(first.Value()))
	require.True(t, guard.accepts(second.Value()))
}

func TestVisibleTextAllowsTabsAndRejectsControlBytes(t *testing.T) {
	_, ok := visibleText("a\tb c")
	require.True(t, ok)

	_, ok = visibleText("a\x7fb")
	require.False(t, ok)

	require.False(t, validHeaderValue("iw-lang=de\r\nSet-Cookie: x=1"))
	require.True(t, validHeaderValue("iw-lang=de"))
}
