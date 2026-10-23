package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
)

// IsHTMX reports whether the request comes from htmx (fragment requested).
func IsHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

// Render writes a templ component as an HTML response.
func Render(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

// Trigger sets the HX-Trigger header so the client reacts to a server event.
func Trigger(w http.ResponseWriter, event string) { w.Header().Add("HX-Trigger", event) }

// SSE writes server-sent events on a response; construct it with NewSSE.
type SSE struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSE prepares the response for a text/event-stream; false when streaming is unsupported.
func NewSSE(w http.ResponseWriter) (*SSE, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &SSE{w: w, flusher: flusher}, true
}

// Send writes one event (id may be empty) and flushes it.
func (s *SSE) Send(ctx context.Context, id, event, data string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("sse context: %w", err)
	}
	if id != "" {
		if _, err := fmt.Fprintf(s.w, "id: %s\n", id); err != nil {
			return fmt.Errorf("write sse id: %w", err)
		}
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return fmt.Errorf("write sse event: %w", err)
	}
	s.flusher.Flush()
	return nil
}
