package components

import (
	"encoding/json"
	"net/url"

	"github.com/a-h/templ"
)

// valsJSON encodes the hx-vals of a request. The values are JSON-encoded, so a profile name can never
// break the markup.
func valsJSON(values map[string]string) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// oobAttrs marks an element as an out-of-band swap target. The attribute must be absent, not empty,
// on a normal swap: htmx treats the mere presence of hx-swap-oob as out of band.
func oobAttrs(oob bool) templ.Attributes {
	if oob {
		return templ.Attributes{"hx-swap-oob": "true"}
	}
	return templ.Attributes{}
}

// chipClass is the class of a profile chip; the selected one is highlighted.
func chipClass(selected bool) string {
	if selected {
		return "chip chip-selected w-full justify-between"
	}
	return "chip w-full justify-between"
}

// deleteURLWithSelection puts the selection in the query string. htmx sends the parameters of a DELETE
// request in the URL, while the body of a DELETE is not read as a form by the server.
func deleteURLWithSelection(deleteURL string, selected string) string {
	return deleteURL + "?" + url.Values{"selected": {selected}}.Encode()
}

// nameHintTrigger is the trigger of a live name hint. "load" fires once when htmx binds the field: htmx
// binds a swapped-in dialog a few milliseconds after the swap, so an input that arrives in that window
// (a fast fill or paste) fires no "input" event. The load request reads the current value and fixes the
// hint and the submit state. Without it, such a value leaves the submit button disabled.
const nameHintTrigger = "input changed delay:150ms, load"

// hashID is the htmx selector of an element id.
func hashID(id string) string { return "#" + id }

// nameHintID and nameSubmitID name the hint and the submit button of a name form, so a live hint
// response swaps exactly those two elements.
func nameHintID(prefix string) string   { return prefix + "-name-hint" }
func nameSubmitID(prefix string) string { return prefix + "-name-submit" }

// deleteMessageArgs are the placeholders of the delete confirmation.
func deleteMessageArgs(name string) map[string]string { return map[string]string{"name": name} }
