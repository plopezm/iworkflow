package components

import "github.com/a-h/templ"

// Routes of the card. The handlers mount these exact paths.
const (
	settingsPath      = "/engram/settings"
	settingsResetPath = "/engram/settings/reset"
	// cardID is the swap target: every action replaces the whole card.
	cardID = "engram-server-card"
)

// actionAttrs posts to path and swaps the whole card with the answer.
func actionAttrs(path string) templ.Attributes {
	return templ.Attributes{
		"hx-post":         path,
		"hx-target":       "#" + cardID,
		"hx-swap":         "outerHTML",
		"hx-disabled-elt": "this",
	}
}
