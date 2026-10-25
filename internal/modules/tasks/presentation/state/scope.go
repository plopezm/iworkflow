package state

import (
	"net/url"
	"regexp"
)

// scopePattern is the shape a board scope may take: short lowercase letters and digits, which are safe inside an
// element id, a CSS selector and a query value.
var scopePattern = regexp.MustCompile(`^[a-z0-9]{1,32}$`)

// SanitizeScope keeps the scope a request carries when it has the shape a board names its elements with, and drops
// anything else, so a crafted scope never becomes part of a selector. A dropped scope means the plain ids.
func SanitizeScope(raw string) string {
	if scopePattern.MatchString(raw) {
		return raw
	}
	return ""
}

// Scoped is the id of an element of one board instance (a board tab or an edit page): the plain name followed by the
// instance scope. Two instances open in the same document therefore never render the same id, and htmx and the
// page script resolve each one to its own element. An empty scope keeps the plain name.
func Scoped(name string, scope string) string {
	if scope == "" {
		return name
	}
	return name + "-" + scope
}

// ScopedPath is an address that carries the instance scope as its scope query value, so the answer names the elements
// of the instance that asked. An empty scope keeps the address as it is.
func ScopedPath(path string, scope string) string {
	if scope == "" {
		return path
	}
	return path + "?" + url.Values{"scope": {scope}}.Encode()
}
