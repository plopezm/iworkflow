package dock

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
)

// Handlers serves the dock fragments htmx swaps into the page. The layout is sent by the browser
// with every request (it is the browser's copy of the state) and the response carries the new one.
type Handlers struct {
	registry *Registry
	launcher templ.Component
	empty    templ.Component
}

// NewHandlers builds the dock handlers. launcher is the entry list of the "+" menu; empty is the
// body of an empty group. Either may be nil.
func NewHandlers(registry *Registry, launcher, empty templ.Component) *Handlers {
	return &Handlers{registry: registry, launcher: launcher, empty: empty}
}

// Routes mounts the dock endpoints:
//
//	POST /dock/layout            restore a layout (form: layout, boot)
//	POST /dock/action/{op}       apply one action (form: layout + the action's fields)
//	GET  /dock/pane?path=...     the page of a tab, loaded the first time the tab is shown
func (h *Handlers) Routes(r chi.Router) {
	r.Post("/dock/layout", h.restore)
	r.Post("/dock/action/{op}", h.action)
	r.Get("/dock/pane", h.pane)
}

// restore renders the saved layout. A boot path (the URL of a page opened directly) is opened as a
// tab; the root path opens nothing, so the dock stays empty like the Rust version.
func (h *Handlers) restore(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	layout := layoutOrNew(r.PostForm.Get("layout"))
	if boot := r.PostForm.Get("boot"); boot != "" && boot != "/" {
		layout.Open(boot, h.registry.Title(r.Context(), boot), 0)
	}
	h.render(w, r, layout) //nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
}

// action applies one operation and renders the resulting layout.
func (h *Handlers) action(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	req, err := ParseRequest(r.PostForm)
	if err != nil {
		http.Error(w, "invalid dock request", http.StatusBadRequest)
		return
	}
	layout := layoutOrNew(r.PostForm.Get("layout"))
	if err := Apply(r.Context(), layout, h.registry, chi.URLParam(r, "op"), req); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrUnknownAction) {
			status = http.StatusBadRequest
		}
		http.Error(w, "dock action failed", status)
		return
	}
	h.render(w, r, layout) //nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
}

// pane renders the page of a tab. Unknown paths render the not-found page.
func (h *Handlers) pane(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, http.StatusOK, h.registry.Render(r.Context(), r.URL.Query().Get("path")))
}

func (h *Handlers) render(w http.ResponseWriter, r *http.Request, layout *Layout) {
	view, err := NewView(layout, h.registry, h.launcher, h.empty)
	if err != nil {
		http.Error(w, fmt.Sprintf("render dock: %v", err), http.StatusInternalServerError)
		return
	}
	web.Render(w, r, http.StatusOK, Root(view))
}

// layoutOrNew restores a layout sent by the browser. Missing or inconsistent data starts from an
// empty layout, as the Rust version does.
func layoutOrNew(saved string) *Layout {
	if saved == "" {
		return New()
	}
	if layout, ok := FromJSON(saved); ok {
		return layout
	}
	return New()
}
