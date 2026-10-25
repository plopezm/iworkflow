// Package link renders a navigation link that opens its target as a dock tab instead of
// navigating. It lives apart from the dock package so that core components (sidebar, shell)
// can use it without importing the dock, which itself depends on those components.
//
// The markup is the contract with web/static/js/dock.js: a plain click is intercepted and
// posted as a dock action; modifier clicks keep the browser's native behaviour.
package link

import (
	"strings"

	"github.com/a-h/templ"
)

// Props configures a tab link.
type Props struct {
	// Href is the target path, optionally with a fragment (`/#lane-1`). The fragment is dropped
	// for the tab, because tabs are identified by path only.
	Href string
	// Class is applied to the anchor.
	Class string
	// Title is the tab title to use when the target is not open yet (empty: the route default).
	Title string
	// Beside opens the tab in the group on the right of the current one.
	Beside bool
	// Reuse is the name of a route: if a tab of that route is open, it is activated instead of
	// opening Href (e.g. returning to the board whatever its workspace).
	Reuse string
	// Role is the ARIA role of the anchor, for a link that acts as a button ("button").
	Role string
}

// Path returns href without its fragment: `/#lane-1` becomes `/`.
func Path(href string) string {
	path, _, _ := strings.Cut(href, "#")
	return path
}

// Attributes returns the data attributes the dock script reads. Empty values are omitted.
func (p Props) Attributes() templ.Attributes {
	attrs := templ.Attributes{
		"data-dock-link": "",
		"data-dock-path": Path(p.Href),
	}
	if p.Title != "" {
		attrs["data-dock-title"] = p.Title
	}
	if p.Beside {
		attrs["data-dock-beside"] = "true"
	}
	if p.Reuse != "" {
		attrs["data-dock-reuse"] = p.Reuse
	}
	if p.Role != "" {
		attrs["role"] = p.Role
	}
	return attrs
}
