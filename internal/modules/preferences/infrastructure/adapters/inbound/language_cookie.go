package inbound

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/preferences/application"
	"iworkflow/internal/modules/preferences/domain/models"
)

// syncKind is what to do with the language of a page request.
type syncKind int

const (
	// syncKeep: the cookie and the database already agree, or neither knows a language.
	syncKeep syncKind = iota
	// syncRestore: the database wins; this browser gets its stored language.
	syncRestore
	// syncPersist: only the cookie knows the language (chosen before it was stored); save it.
	syncPersist
)

type syncDecision struct {
	kind syncKind
	code string
}

// reconcile compares the stored language with the cookie.
func reconcile(stored *models.UILanguage, cookie *i18n.Locale) syncDecision {
	switch {
	case stored != nil && cookie != nil && cookie.Code() == stored.Code():
		return syncDecision{kind: syncKeep}
	case stored != nil:
		return syncDecision{kind: syncRestore, code: stored.Code()}
	case cookie != nil:
		return syncDecision{kind: syncPersist, code: cookie.Code()}
	default:
		return syncDecision{kind: syncKeep}
	}
}

// cookieHeaderWithLanguage returns the Cookie header with the language pair replaced by code.
func cookieHeaderWithLanguage(header string, code string) string {
	var pairs []string
	for _, pair := range strings.Split(header, ";") {
		trimmed := strings.TrimSpace(pair)
		if trimmed == "" || isLanguagePair(trimmed) {
			continue
		}
		pairs = append(pairs, trimmed)
	}
	pairs = append(pairs, i18n.LocaleCookie+"="+code)
	return strings.Join(pairs, "; ")
}

func isLanguagePair(pair string) bool {
	name, _, found := strings.Cut(pair, "=")
	return found && strings.TrimSpace(name) == i18n.LocaleCookie
}

// SyncLanguageCookie keeps the language cookie in step with the language stored in the database,
// so a browser that never picked a language (new browser, cleared cookies) opens in the stored
// one instead of asking again. It only runs on page loads, before the page reads the cookie.
// Storage failures are logged and the request goes on with whatever the cookie says.
func SyncLanguageCookie(
	logger *slog.Logger,
	getLanguage *application.GetLanguage,
	saveLanguage *application.SaveLanguage,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isHTMLGet(r) {
				next.ServeHTTP(w, r)
				return
			}
			header, _ := visibleFirst(r.Header, "Cookie")
			var cookie *i18n.Locale
			if locale, found := i18n.FromCookieHeader(header); found {
				cookie = &locale
			}
			stored, err := getLanguage.Execute(r.Context())
			if err != nil {
				logger.Warn("could not read the saved language", "error", err)
				next.ServeHTTP(w, r)
				return
			}
			decision := reconcile(stored, cookie)
			switch decision.kind {
			case syncRestore:
				rewritten := cookieHeaderWithLanguage(header, decision.code)
				if validHeaderValue(rewritten) {
					r.Header.Set("Cookie", rewritten)
					writer := &languageCookieWriter{ResponseWriter: w, setCookie: languageSetCookie(decision.code)}
					next.ServeHTTP(writer, r)
					writer.finish()
					return
				}
			case syncPersist:
				if _, saveErr := saveLanguage.Execute(r.Context(), decision.code); saveErr != nil {
					logger.Warn("could not save the chosen language", "error", saveErr)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// languageSetCookie is the Set-Cookie value that rewrites the language cookie for a year.
func languageSetCookie(code string) string {
	return fmt.Sprintf("%s=%s; Path=/; Max-Age=%d; SameSite=Lax",
		i18n.LocaleCookie, code, i18n.CookieMaxAgeSecs)
}

// languageCookieWriter adds the Set-Cookie header just before the response headers go out, so it
// lands after whatever the handler set, as the Rust build appended it after the handler returned.
type languageCookieWriter struct {
	http.ResponseWriter
	setCookie string
	written   bool
}

func (w *languageCookieWriter) WriteHeader(status int) {
	w.addCookieOnce()
	w.ResponseWriter.WriteHeader(status)
}

func (w *languageCookieWriter) Write(p []byte) (int, error) {
	w.addCookieOnce()
	return w.ResponseWriter.Write(p) //nolint:wrapcheck // the response writer error goes to net/http unchanged
}

// Unwrap lets http.ResponseController reach the underlying writer (flushing, deadlines).
func (w *languageCookieWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// finish adds the cookie when the handler wrote nothing: headers then go out after it returns.
func (w *languageCookieWriter) finish() {
	w.addCookieOnce()
}

func (w *languageCookieWriter) addCookieOnce() {
	if w.written {
		return
	}
	w.written = true
	w.Header().Add("Set-Cookie", w.setCookie)
}
