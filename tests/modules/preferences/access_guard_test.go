package preferences_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound"
)

// hitCounter counts how many requests reached the protected application.
type hitCounter struct {
	count atomic.Int64
}

func (h *hitCounter) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h.count.Add(1)
		_, _ = w.Write([]byte("hello"))
	})
}

// guardedServer mirrors the real wiring: the guard wraps a mux that holds the login route and the
// protected application. The fallback answers any path nothing else matched, as the Leptos app does.
type guardedServer struct {
	*httptest.Server
	hits   *hitCounter
	client *http.Client
}

func newGuardedServer(t *testing.T, guard *inbound.AccessGuard) guardedServer {
	t.Helper()
	hits := &hitCounter{}
	mux := http.NewServeMux()
	mux.Handle(inbound.LoginPath, inbound.AccessRouter(guard))
	mux.Handle("/hello", hits.handler())
	mux.Handle("/other", hits.handler())
	mux.Handle("/", hits.handler())
	testRemote := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sim := r.Header.Get("X-Test-Remote-Addr"); sim != "" {
			r.RemoteAddr = sim
		} else {
			r.RemoteAddr = "192.0.2.1:12345"
		}
		inbound.RequireAccess(guard)(mux).ServeHTTP(w, r)
	})
	server := httptest.NewServer(testRemote)
	t.Cleanup(server.Close)
	return guardedServer{
		Server: server,
		hits:   hits,
		client: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// response is a fully read reply, so tests never juggle open bodies.
type response struct {
	status int
	header http.Header
	body   string
}

// send performs the request and reads the whole reply before returning.
func send(t *testing.T, client *http.Client, req *http.Request) response {
	t.Helper()
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return response{status: resp.StatusCode, header: resp.Header, body: string(raw)}
}

// do sends a request with optional headers to the guarded server.
func (s guardedServer) do(t *testing.T, method string, url string, body string, headers map[string]string) response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, s.URL+url, strings.NewReader(body))
	require.NoError(t, err)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return send(t, s.client, req)
}

func newToken(t *testing.T) models.AccessToken {
	t.Helper()
	token, err := models.GenerateAccessToken()
	require.NoError(t, err)
	return token
}

func cookieFor(token models.AccessToken) string {
	return "iw-token=" + token.Value()
}

func expectedSetCookie(token models.AccessToken) string {
	return "iw-token=" + token.Value() + "; HttpOnly; SameSite=Strict; Path=/; Max-Age=31536000"
}

func TestWithoutEnforcementEverythingPasses(t *testing.T) {
	server := newGuardedServer(t, inbound.NewAccessGuard(newToken(t), false))

	resp := server.do(t, http.MethodGet, "/hello", "", nil)

	require.Equal(t, http.StatusOK, resp.status)
	require.Equal(t, int64(1), server.hits.count.Load())
}

func TestBearerAndCookieAmongOthersPass(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	bearer := server.do(t, http.MethodGet, "/hello", "", map[string]string{
		"Authorization": "Bearer " + token.Value(),
	})
	require.Equal(t, http.StatusOK, bearer.status)

	cookie := server.do(t, http.MethodGet, "/hello", "", map[string]string{
		"Cookie": "a=1; " + cookieFor(token) + "; iw-lang=es",
	})
	require.Equal(t, http.StatusOK, cookie.status)
	require.Equal(t, int64(2), server.hits.count.Load())
}

func TestQueryTokenRedirectsWithoutTheTokenAndSetsTheCookie(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	resp := server.do(t, http.MethodGet, "/hello?x=1&token="+token.Value()+"&y=2", "", nil)

	require.Equal(t, http.StatusSeeOther, resp.status)
	require.Equal(t, "/hello?x=1&y=2", resp.header.Get("Location"))
	require.Equal(t, expectedSetCookie(token), resp.header.Get("Set-Cookie"))
	require.Equal(t, "no-store", resp.header.Get("Cache-Control"))

	only := server.do(t, http.MethodGet, "/hello?token="+token.Value(), "", nil)
	require.Equal(t, http.StatusSeeOther, only.status)
	require.Equal(t, "/hello", only.header.Get("Location"))
	require.Equal(t, int64(0), server.hits.count.Load())
}

func TestWrongQueryTokenIsRejectedAndPostAuthenticatesWithoutRedirect(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	wrong := server.do(t, http.MethodGet, "/hello?token="+strings.Repeat("0", 64), "", nil)
	require.Equal(t, http.StatusUnauthorized, wrong.status)
	require.Empty(t, wrong.header.Get("Set-Cookie"))
	require.Equal(t, int64(0), server.hits.count.Load())

	post := server.do(t, http.MethodPost, "/other?token="+token.Value(), "", nil)
	require.Equal(t, http.StatusOK, post.status)
	require.Equal(t, int64(1), server.hits.count.Load())
}

func TestRejectsHTMLWithLoginPageAndOthersWithEmptyBody(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	page := server.do(t, http.MethodGet, "/hello", "", map[string]string{
		"Accept": "text/html,application/xhtml+xml",
		"Cookie": "iw-lang=de",
	})
	require.Equal(t, http.StatusUnauthorized, page.status)
	require.Equal(t, "text/html; charset=utf-8", page.header.Get("Content-Type"))
	require.Equal(t, "no-store", page.header.Get("Cache-Control"))
	require.Contains(t, page.body, `<form method="post" action="/access-login">`)
	require.Contains(t, page.body, `name="token"`)
	require.Contains(t, page.body, `type="password"`)
	require.Contains(t, page.body, "Zugriffstoken")
	require.NotContains(t, page.body, token.Value())

	defaultLanguage := server.do(t, http.MethodGet, "/hello", "", map[string]string{"Accept": "text/html"})
	require.Contains(t, defaultLanguage.body, "Access token required")

	api := server.do(t, http.MethodGet, "/hello", "", nil)
	require.Equal(t, http.StatusUnauthorized, api.status)
	require.Equal(t, "no-store", api.header.Get("Cache-Control"))
	require.Empty(t, api.body)

	postHTML := server.do(t, http.MethodPost, "/hello", "", map[string]string{"Accept": "text/html"})
	require.Equal(t, http.StatusUnauthorized, postHTML.status)
	require.Empty(t, postHTML.body, "only GET pages get the login form")
	require.Equal(t, int64(0), server.hits.count.Load())
}

func TestLoginFormSetsTheCookieOrShowsThePageAgain(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))
	form := "application/x-www-form-urlencoded"

	ok := server.do(t, http.MethodPost, "/access-login", "token="+token.Value(), map[string]string{
		"Content-Type": form,
	})
	require.Equal(t, http.StatusSeeOther, ok.status)
	require.Equal(t, "/", ok.header.Get("Location"))
	require.Equal(t, expectedSetCookie(token), ok.header.Get("Set-Cookie"))
	require.Equal(t, "no-store", ok.header.Get("Cache-Control"))

	bad := server.do(t, http.MethodPost, "/access-login", "token=nope", map[string]string{
		"Content-Type": form,
		"Cookie":       "iw-lang=fr",
	})
	require.Equal(t, http.StatusUnauthorized, bad.status)
	require.Empty(t, bad.header.Get("Set-Cookie"))
	require.Equal(t, "text/html; charset=utf-8", bad.header.Get("Content-Type"))
	require.Contains(t, bad.body, "Jeton invalide.")
	require.NotContains(t, bad.body, "nope")

	oversized := server.do(t, http.MethodPost, "/access-login", "token="+strings.Repeat("a", 10_000), map[string]string{
		"Content-Type": form,
	})
	require.Equal(t, http.StatusRequestEntityTooLarge, oversized.status)
}

func TestLoginFormWithoutAFormContentTypeIsUnsupported(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	resp := server.do(t, http.MethodPost, "/access-login", "token="+token.Value(), map[string]string{
		"Content-Type": "text/plain",
	})

	require.Equal(t, http.StatusUnsupportedMediaType, resp.status)
	require.Equal(t, int64(0), server.hits.count.Load())
}

func TestLoginRouteRedirectsHomeWhenNotEnforcing(t *testing.T) {
	server := newGuardedServer(t, inbound.NewAccessGuard(newToken(t), false))

	resp := server.do(t, http.MethodPost, "/access-login", "token=whatever", map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})

	require.Equal(t, http.StatusSeeOther, resp.status)
	require.Equal(t, "/", resp.header.Get("Location"))
	require.Empty(t, resp.header.Get("Set-Cookie"))
}

func TestArbitraryRouteNeedsTheCookie(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	denied := server.do(t, http.MethodGet, "/hello", "", nil)
	require.Equal(t, http.StatusUnauthorized, denied.status)
	require.Equal(t, int64(0), server.hits.count.Load())

	allowed := server.do(t, http.MethodGet, "/hello", "", map[string]string{"Cookie": cookieFor(token)})
	require.Equal(t, http.StatusOK, allowed.status)
	require.Equal(t, int64(1), server.hits.count.Load())
}

func TestRotatingTheTokenInvalidatesTheOldOneAtOnce(t *testing.T) {
	old := newToken(t)
	guard := inbound.NewAccessGuard(old, true)
	server := newGuardedServer(t, guard)

	first := server.do(t, http.MethodGet, "/hello", "", map[string]string{"Cookie": cookieFor(old)})
	require.Equal(t, http.StatusOK, first.status)

	fresh := newToken(t)
	guard.SetToken(fresh)
	stale := server.do(t, http.MethodGet, "/hello", "", map[string]string{"Cookie": cookieFor(old)})
	require.Equal(t, http.StatusUnauthorized, stale.status)

	current := server.do(t, http.MethodGet, "/hello", "", map[string]string{"Cookie": cookieFor(fresh)})
	require.Equal(t, http.StatusOK, current.status)
	require.Equal(t, int64(2), server.hits.count.Load())
}

func TestWrongTokensOfAnyShapeGetTheSameAnswer(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))
	almost := token.Value()[:63] + "0"
	if almost == token.Value() {
		almost = token.Value()[:63] + "1"
	}

	for _, candidate := range []string{almost, "", "short"} {
		resp := server.do(t, http.MethodGet, "/hello", "", map[string]string{
			"Authorization": "Bearer " + candidate,
		})
		require.Equal(t, http.StatusUnauthorized, resp.status, candidate)
		require.Empty(t, resp.body, candidate)
	}
	require.Equal(t, int64(0), server.hits.count.Load())
}

func TestLoginPathWithAnotherMethodStillNeedsAToken(t *testing.T) {
	server := newGuardedServer(t, inbound.NewAccessGuard(newToken(t), true))

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		resp := server.do(t, method, "/access-login", "", map[string]string{"Accept": "text/html"})
		require.Equal(t, http.StatusUnauthorized, resp.status, method)
	}
	require.Equal(t, int64(0), server.hits.count.Load())
}

func TestEncodedLoginPathDoesNotBypassTheGuard(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	// The correct token is submitted on purpose: a bypass would then answer 303, not 401.
	resp := server.do(t, http.MethodPost, "/access%2Dlogin", "token="+token.Value(), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})

	require.Equal(t, http.StatusUnauthorized, resp.status)
	require.Empty(t, resp.header.Get("Set-Cookie"))
}

func TestTokenRedirectNeverPointsToAnotherHost(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	resp := server.do(t, http.MethodGet, "//evil.example/?token="+token.Value(), "", nil)

	require.Equal(t, http.StatusSeeOther, resp.status)
	require.Equal(t, "/evil.example/", resp.header.Get("Location"))
}

func TestCrossSiteWritesAreRefusedBeforeReachingTheApp(t *testing.T) {
	hits := &hitCounter{}
	mux := http.NewServeMux()
	mux.Handle("/hello", hits.handler())
	server := httptest.NewServer(inbound.RejectCrossSiteWrites(mux))
	t.Cleanup(server.Close)
	client := &http.Client{}

	postWithOrigin := func(origin string) response {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/hello", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", origin)
		return send(t, client, req)
	}

	foreign := postWithOrigin("https://evil.example")
	require.Equal(t, http.StatusForbidden, foreign.status)
	require.Equal(t, int64(0), hits.count.Load())

	same := postWithOrigin(server.URL)
	require.Equal(t, http.StatusOK, same.status)

	readReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/hello", nil)
	require.NoError(t, err)
	readReq.Header.Set("Origin", "https://evil.example")
	read := send(t, client, readReq)
	require.Equal(t, http.StatusOK, read.status)
}

func TestSameIPPassesWithoutTokenWhenEnforcing(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	// Loopback IPv4 passes without token
	resp := server.do(t, http.MethodGet, "/hello", "", map[string]string{
		"X-Test-Remote-Addr": "127.0.0.1:54321",
	})
	require.Equal(t, http.StatusOK, resp.status)
	require.Equal(t, int64(1), server.hits.count.Load())

	// Loopback IPv6 passes without token
	resp6 := server.do(t, http.MethodGet, "/hello", "", map[string]string{
		"X-Test-Remote-Addr": "[::1]:54321",
	})
	require.Equal(t, http.StatusOK, resp6.status)
	require.Equal(t, int64(2), server.hits.count.Load())

	// External IP requires token
	ext := server.do(t, http.MethodGet, "/hello", "", map[string]string{
		"X-Test-Remote-Addr": "192.0.2.100:54321",
	})
	require.Equal(t, http.StatusUnauthorized, ext.status)
	require.Equal(t, int64(2), server.hits.count.Load())
}

func TestSameIPWithQueryTokenRedirectsToCleanURL(t *testing.T) {
	token := newToken(t)
	server := newGuardedServer(t, inbound.NewAccessGuard(token, true))

	resp := server.do(t, http.MethodGet, "/hello?token="+token.Value(), "", map[string]string{
		"X-Test-Remote-Addr": "127.0.0.1:54321",
	})
	require.Equal(t, http.StatusSeeOther, resp.status)
	require.Equal(t, "/hello", resp.header.Get("Location"))
	require.Equal(t, expectedSetCookie(token), resp.header.Get("Set-Cookie"))
}
