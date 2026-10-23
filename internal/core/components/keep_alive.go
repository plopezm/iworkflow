package components

import "github.com/a-h/templ"

// KeepAliveProps configures a pane that loads its content on first activation and then keeps it
// mounted, only hiding it when inactive. Src is the URL of the content: web/static/js/ui.js fetches it
// once, the first time the pane is active. ActiveClass defaults to "contents".
type KeepAliveProps struct {
	Active      bool
	Src         string
	ActiveClass string
	Attrs       templ.Attributes
}

// paneClass is the class of a pane: shown when active, hidden otherwise.
func paneClass(active bool, activeClass string) string {
	if active {
		if activeClass == "" {
			return "contents"
		}
		return activeClass
	}
	return "hidden"
}
