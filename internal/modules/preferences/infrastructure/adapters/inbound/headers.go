package inbound

import (
	"net/http"
	"strings"

	"iworkflow/internal/core/i18n"
)

// visibleFirst returns the first value of a header, provided it is visible ASCII. A value with
// other bytes counts as absent, as it did in the Rust build.
func visibleFirst(header http.Header, name string) (string, bool) {
	values := header.Values(name)
	if len(values) == 0 {
		return "", false
	}
	return visibleText(values[0])
}

// visibleText reports whether value holds only visible ASCII, spaces or tabs.
func visibleText(value string) (string, bool) {
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b < 0x20 || b > 0x7e) && b != '\t' {
			return "", false
		}
	}
	return value, true
}

// validHeaderValue reports whether value can be sent as a header: no control characters except tab.
func validHeaderValue(value string) bool {
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b < 0x20 && b != '\t') || b == 0x7f {
			return false
		}
	}
	return true
}

// setHeaderIfValid sets the header only when its value can be sent; otherwise it is skipped.
func setHeaderIfValid(header http.Header, name string, value string) {
	if validHeaderValue(value) {
		header.Set(name, value)
	}
}

// isHTMLGet reports whether the request is a GET that accepts HTML, i.e. a browser loading a page.
func isHTMLGet(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	accept, ok := visibleFirst(r.Header, "Accept")
	return ok && strings.Contains(accept, "text/html")
}

// localeOf returns the language chosen in the cookie, English when there is none.
func localeOf(header http.Header) i18n.Locale {
	cookie, ok := visibleFirst(header, "Cookie")
	if !ok {
		return i18n.En
	}
	if locale, found := i18n.FromCookieHeader(cookie); found {
		return locale
	}
	return i18n.En
}
