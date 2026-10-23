// Package inbound holds the HTTP middlewares of the preferences module: the optional access-token
// guard, the cross-site write check, the language cookie sync and the access link helper. Each is
// a plain func(http.Handler) http.Handler so the composition root can stack them.
//
// The access guard is optional and only matters when the app is reachable from the network. When
// enforcing, every request must carry the current token (bearer header, cookie or a one-shot
// `?token=` link, e.g. from a QR code). Anything else gets a 401, with a small login page for
// browsers.
package inbound

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/preferences/domain/models"
)

// AccessCookie is the name of the cookie that keeps the token in the browser.
const AccessCookie = "iw-token"

// LoginPath is the route that receives the login form.
const LoginPath = "/access-login"

const (
	// tokenParam is the query parameter that carries the token in shareable links.
	tokenParam = "token"
	// bearerPrefix is the scheme prefix of the Authorization header, case-sensitive as before.
	bearerPrefix = "Bearer "
	// sessionMaxAgeSecs is one year, in seconds.
	sessionMaxAgeSecs = 31_536_000
	// loginBodyLimitBytes caps the login form: it only holds a 64-character token.
	loginBodyLimitBytes = 4096
	noStore             = "no-store"
	formContentType     = "application/x-www-form-urlencoded"
)

// Login page texts, copied from the Rust build.
var (
	loginTitle = i18n.Msg{
		En: "Access token required",
		Es: "Se requiere un token de acceso",
		Zh: "需要访问令牌",
		De: "Zugriffstoken erforderlich",
		Fr: "Jeton d'accès requis",
	}
	loginLabel = i18n.Msg{
		En: "Access token",
		Es: "Token de acceso",
		Zh: "访问令牌",
		De: "Zugriffstoken",
		Fr: "Jeton d'accès",
	}
	loginSubmit = i18n.Msg{
		En: "Sign in",
		Es: "Entrar",
		Zh: "登录",
		De: "Anmelden",
		Fr: "Se connecter",
	}
	loginInvalid = i18n.Msg{
		En: "Invalid token.",
		Es: "Token no válido.",
		Zh: "令牌无效。",
		De: "Ungültiges Token.",
		Fr: "Jeton invalide.",
	}
)

// AccessGuard holds the current access token (replaceable at runtime) and whether it is enforced.
// Use NewAccessGuard; the zero value is not usable.
type AccessGuard struct {
	mu      sync.RWMutex
	token   models.AccessToken
	enforce bool
}

// NewAccessGuard guards requests with token. When enforce is false every request passes.
func NewAccessGuard(token models.AccessToken, enforce bool) *AccessGuard {
	return &AccessGuard{token: token, enforce: enforce}
}

// SetToken replaces the token; it takes effect for the next request. The old token stops working
// at once.
func (g *AccessGuard) SetToken(token models.AccessToken) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.token = token
}

// IsEnforcing reports whether requests must present the token.
func (g *AccessGuard) IsEnforcing() bool {
	return g.enforce
}

// SessionCookie returns the Set-Cookie value that stores the current token in the browser.
func (g *AccessGuard) SessionCookie() string {
	return fmt.Sprintf("%s=%s; HttpOnly; SameSite=Strict; Path=/; Max-Age=%d",
		AccessCookie, g.current().Value(), sessionMaxAgeSecs)
}

func (g *AccessGuard) current() models.AccessToken {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.token
}

func (g *AccessGuard) accepts(candidate string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.token.Matches(candidate)
}

// tokenSource tells where a presented token was found.
type tokenSource int

const (
	sourceBearer tokenSource = iota
	sourceCookie
	sourceQuery
)

type presentedToken struct {
	source tokenSource
	value  string
}

// presentedTokens lists the tokens the request carries, in priority order.
func presentedTokens(r *http.Request) []presentedToken {
	var found []presentedToken
	if token, ok := bearerToken(r.Header); ok {
		found = append(found, presentedToken{source: sourceBearer, value: token})
	}
	if token, ok := cookieValue(r.Header, AccessCookie); ok {
		found = append(found, presentedToken{source: sourceCookie, value: token})
	}
	if token, ok := queryToken(r.URL.RawQuery); ok {
		found = append(found, presentedToken{source: sourceQuery, value: token})
	}
	return found
}

// acceptedSource returns where the first accepted token was found.
func acceptedSource(guard *AccessGuard, r *http.Request) (tokenSource, bool) {
	for _, presented := range presentedTokens(r) {
		if guard.accepts(presented.value) {
			return presented.source, true
		}
	}
	return sourceBearer, false
}

// bearerToken returns the Authorization bearer value, trimmed.
func bearerToken(header http.Header) (string, bool) {
	value, ok := visibleFirst(header, "Authorization")
	if !ok {
		return "", false
	}
	rest, found := strings.CutPrefix(value, bearerPrefix)
	if !found {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

// cookieValue returns the value of cookie name across all Cookie headers.
func cookieValue(header http.Header, name string) (string, bool) {
	for _, raw := range header.Values("Cookie") {
		value, ok := visibleText(raw)
		if !ok {
			continue
		}
		for _, pair := range strings.Split(value, ";") {
			key, val, found := strings.Cut(strings.TrimSpace(pair), "=")
			if found && strings.TrimSpace(key) == name {
				return strings.TrimSpace(val), true
			}
		}
	}
	return "", false
}

// queryToken returns the value of the token query parameter, if any.
func queryToken(rawQuery string) (string, bool) {
	for _, pair := range strings.Split(rawQuery, "&") {
		key, value, found := strings.Cut(pair, "=")
		if found && key == tokenParam {
			return value, true
		}
	}
	return "", false
}

// stripTokenParam returns the raw query without the token parameter; false when nothing remains.
func stripTokenParam(rawQuery string) (string, bool) {
	var remaining []string
	for _, pair := range strings.Split(rawQuery, "&") {
		if pair == "" {
			continue
		}
		if key, _, _ := strings.Cut(pair, "="); key == tokenParam {
			continue
		}
		remaining = append(remaining, pair)
	}
	if len(remaining) == 0 {
		return "", false
	}
	return strings.Join(remaining, "&"), true
}

// cleanLocation is the request path without the token parameter, ready for a Location header.
// Leading slashes collapse: "//host/" would be read by browsers as another origin.
func cleanLocation(r *http.Request) string {
	path := "/" + strings.TrimLeft(r.URL.EscapedPath(), "/")
	if rest, ok := stripTokenParam(r.URL.RawQuery); ok {
		return path + "?" + rest
	}
	return path
}

// RequireAccess lets a request through when the guard is off or the request carries the current
// token; otherwise it answers 401. The login route is skipped because it checks the token itself.
func RequireAccess(guard *AccessGuard) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only the login submission skips the check: any other method on that path (which the
			// application fallback could answer) still needs a token. The escaped path keeps
			// "/access%2Dlogin" from slipping past as "/access-login".
			isLogin := r.Method == http.MethodPost && r.URL.EscapedPath() == LoginPath
			if !guard.IsEnforcing() || isLogin {
				next.ServeHTTP(w, r)
				return
			}
			source, accepted := acceptedSource(guard, r)
			switch {
			case accepted && source == sourceQuery && (r.Method == http.MethodGet || r.Method == http.MethodHead):
				redirectWithSession(w, guard, cleanLocation(r))
			case accepted:
				next.ServeHTTP(w, r)
			default:
				writeUnauthorized(w, isHTMLGet(r), localeOf(r.Header), false)
			}
		})
	}
}

// AccessRouter answers the login form. Mount it at LoginPath behind RequireAccess.
func AccessRouter(guard *AccessGuard) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		serveLogin(guard, w, r)
	})
}

func serveLogin(guard *AccessGuard, w http.ResponseWriter, r *http.Request) {
	token, ok := readLoginToken(w, r)
	if !ok {
		return
	}
	if !guard.IsEnforcing() || guard.accepts(token) {
		redirectWithSessionIfEnforcing(w, guard)
		return
	}
	writeUnauthorized(w, true, localeOf(r.Header), true)
}

// readLoginToken decodes the urlencoded login form. It answers the client itself and returns false
// when the body is unusable (wrong content type, too large or unreadable).
func readLoginToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != formContentType {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return "", false
	}
	r.Body = http.MaxBytesReader(w, r.Body, loginBodyLimitBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		return "", false
	}
	// ParseQuery keeps every pair it could read even when it reports an error. A malformed body
	// simply carries no usable token, which the caller treats as a wrong one.
	form, _ := url.ParseQuery(string(body))
	return form.Get(tokenParam), true
}

// redirectWithSessionIfEnforcing sends the browser home; the cookie is only set when there is a
// token to protect.
func redirectWithSessionIfEnforcing(w http.ResponseWriter, guard *AccessGuard) {
	if guard.IsEnforcing() {
		redirectWithSession(w, guard, "/")
		return
	}
	w.Header().Set("Location", "/")
	setNoStore(w)
	w.WriteHeader(http.StatusSeeOther)
}

// redirectWithSession answers 303 See Other to location and stores the current token in the browser.
func redirectWithSession(w http.ResponseWriter, guard *AccessGuard, location string) {
	setHeaderIfValid(w.Header(), "Location", location)
	w.Header().Set("Set-Cookie", guard.SessionCookie())
	setNoStore(w)
	w.WriteHeader(http.StatusSeeOther)
}

// writeUnauthorized answers 401. Browsers loading a page get the login form; everything else gets
// an empty body.
func writeUnauthorized(w http.ResponseWriter, html bool, locale i18n.Locale, invalid bool) {
	setNoStore(w)
	if !html {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	// A failed write means the client has gone away; there is nothing left to report to.
	_, _ = io.WriteString(w, loginPage(locale, invalid))
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", noStore)
}

// loginPageTemplate is the self-contained login page. The argument order is: language code,
// title, login action, error paragraph, field label, submit text.
const loginPageTemplate = `<!doctype html><html lang="%[1]s"><head><meta charset="utf-8">` +
	`<meta name="viewport" content="width=device-width,initial-scale=1">` +
	`<meta name="robots" content="noindex"><title>%[2]s</title><style>` +
	`body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;` +
	`background:#0b0d12;color:#e6e8ee;font:16px system-ui,sans-serif}` +
	`form{width:min(22rem,90vw);display:flex;flex-direction:column;gap:.75rem;padding:1.5rem;` +
	`border:1px solid #2a2f3a;border-radius:1rem;background:#141821}` +
	`h1{margin:0;font-size:1.1rem}label{font-size:.85rem;color:#9aa3b5}` +
	`input{padding:.6rem .7rem;border-radius:.6rem;border:1px solid #2a2f3a;background:#0b0d12;` +
	`color:inherit;font:inherit}` +
	`button{padding:.6rem;border:0;border-radius:.6rem;background:#4f7cff;color:#fff;font:inherit;` +
	`cursor:pointer}.error{margin:0;color:#ff7b7b;font-size:.85rem}` +
	`</style></head><body><form method="post" action="%[3]s"><h1>%[2]s</h1>%[4]s` +
	`<label for="token">%[5]s</label>` +
	`<input id="token" name="token" type="password" autocomplete="current-password" ` +
	`autofocus required><button type="submit">%[6]s</button></form></body></html>`

// loginPage renders the login form in locale; invalid adds the "wrong token" message.
func loginPage(locale i18n.Locale, invalid bool) string {
	errorHTML := ""
	if invalid {
		errorHTML = fmt.Sprintf(`<p class="error" role="alert">%s</p>`, loginInvalid.In(locale))
	}
	return fmt.Sprintf(loginPageTemplate,
		locale.Code(),
		loginTitle.In(locale),
		LoginPath,
		errorHTML,
		loginLabel.In(locale),
		loginSubmit.In(locale),
	)
}
