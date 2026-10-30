package inbound

import (
	"net/http"
	"strings"
)

// IsCrossSiteWrite reports whether the request is a write that a browser sent from a different
// origin than the host it targets. A page on another site can make a browser POST to this app
// (server functions accept plain form bodies, so no CORS preflight happens), but browsers always
// label such requests with an Origin that differs from the Host. Requests without Origin (curl,
// native clients) are not browser cross-site traffic.
func IsCrossSiteWrite(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	origin, ok := visibleFirst(r.Header, "Origin")
	if !ok {
		return false
	}
	authority, ok := originAuthority(origin)
	// A null origin, an unknown scheme or a missing Host cannot be proven same-origin.
	if !ok || r.Host == "" {
		return true
	}
	return !strings.EqualFold(authority, r.Host)
}

// originAuthority strips the scheme of an http(s) origin, leaving "host[:port]".
func originAuthority(origin string) (string, bool) {
	if rest, found := strings.CutPrefix(origin, "http://"); found {
		return rest, true
	}
	return strings.CutPrefix(origin, "https://")
}

// RejectCrossSiteWrites answers 403 Forbidden to cross-site writes before they reach the app.
func RejectCrossSiteWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsCrossSiteWrite(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
