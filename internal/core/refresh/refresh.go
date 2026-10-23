// Package refresh announces that shared data changed, so the lists that show it reload.
//
// The original kept a client-side signal per page. With htmx the server announces the change
// with an HX-Trigger event and the interested fragments re-request themselves.
package refresh

import "net/http"

// DataChangedEvent is the HX-Trigger name announcing a change to shared data.
const DataChangedEvent string = "data-changed"

// Trigger tells the client that shared data changed in the response.
func Trigger(w http.ResponseWriter) {
	w.Header().Add("HX-Trigger", DataChangedEvent)
}
