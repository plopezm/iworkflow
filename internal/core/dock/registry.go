package dock

import (
	"context"
	"fmt"
	"strings"

	"github.com/a-h/templ"

	"iworkflow/internal/core/i18n"
)

// Params holds the captures of a route pattern: `{id}` is params["id"], a trailing `{path...}`
// captures the rest of the path (percent-decoded).
type Params map[string]string

// Route is one kind of tab page. A module registers its routes; the registry picks the first one
// whose Pattern matches the tab path.
type Route struct {
	// Name identifies the route for reuse (dock.Link with Reuse: "<name>"). Empty means not reusable.
	Name string
	// Pattern is the path template, e.g. "/tasks/{id}/files/{path...}".
	Pattern string
	// Icon is the one-glyph icon shown on the tab button and in the launcher.
	Icon string
	// Title is the default tab title for a path. Nil falls back to the not-found title.
	Title func(ctx context.Context, params Params) string
	// Render builds the page of a tab. Nil renders the not-found page.
	Render func(ctx context.Context, params Params) templ.Component
}

// segment is one part of a compiled pattern.
type segment struct {
	literal string
	name    string
	rest    bool
	capture bool
}

type compiled struct {
	route    Route
	segments []segment
}

// Registry maps tab paths to routes. Build it once at start-up and pass it to the dock handlers.
type Registry struct {
	routes   []compiled
	notFound Route
}

// NewRegistry compiles the routes. It fails on an invalid pattern or a duplicated route name, so
// a mistake shows at start-up rather than as a wrong tab later.
func NewRegistry(routes ...Route) (*Registry, error) {
	registry := &Registry{notFound: notFoundRoute()}
	names := map[string]bool{}
	for _, route := range routes {
		segments, err := compilePattern(route.Pattern)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", route.Pattern, err)
		}
		if route.Name != "" {
			if names[route.Name] {
				return nil, fmt.Errorf("duplicate route name %q", route.Name)
			}
			names[route.Name] = true
		}
		registry.routes = append(registry.routes, compiled{route: route, segments: segments})
	}
	return registry, nil
}

func compilePattern(pattern string) ([]segment, error) {
	parts := splitPath(pattern)
	segments := make([]segment, 0, len(parts))
	for i, part := range parts {
		switch {
		case strings.HasPrefix(part, "{") && strings.HasSuffix(part, "...}"):
			if i != len(parts)-1 {
				return nil, fmt.Errorf("rest capture %q must be the last segment", part)
			}
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "...}")
			if name == "" {
				return nil, fmt.Errorf("empty capture name in %q", part)
			}
			segments = append(segments, segment{name: name, rest: true})
		case strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}"):
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if name == "" {
				return nil, fmt.Errorf("empty capture name in %q", part)
			}
			segments = append(segments, segment{name: name, capture: true})
		default:
			segments = append(segments, segment{literal: part})
		}
	}
	return segments, nil
}

// Match returns the route for path and its captures. When no route matches it returns the
// not-found route and false.
func (r *Registry) Match(path string) (Route, Params, bool) {
	parts := splitPath(path)
	for _, c := range r.routes {
		if params, ok := matchSegments(c.segments, parts); ok {
			return c.route, params, true
		}
	}
	return r.notFound, Params{}, false
}

// MatchesName reports whether path is served by the route called name.
func (r *Registry) MatchesName(path, name string) bool {
	if name == "" {
		return false
	}
	route, _, ok := r.Match(path)
	return ok && route.Name == name
}

// Title returns the default title of path.
func (r *Registry) Title(ctx context.Context, path string) string {
	route, params, _ := r.Match(path)
	if route.Title == nil {
		return msgNotFoundTitle.T(ctx)
	}
	return route.Title(ctx, params)
}

// Icon returns the icon of path.
func (r *Registry) Icon(path string) string {
	route, _, _ := r.Match(path)
	return route.Icon
}

// Render returns the page of path.
func (r *Registry) Render(ctx context.Context, path string) templ.Component {
	route, params, _ := r.Match(path)
	if route.Render == nil {
		return notFoundPage()
	}
	return route.Render(ctx, params)
}

func matchSegments(segments []segment, parts []string) (Params, bool) {
	params := Params{}
	for i, seg := range segments {
		switch {
		case seg.rest:
			if i >= len(parts) {
				return nil, false
			}
			params[seg.name] = decodePath(strings.Join(parts[i:], "/"))
			return params, true
		case i >= len(parts):
			return nil, false
		case seg.capture:
			if parts[i] == "" {
				return nil, false
			}
			params[seg.name] = decodePath(parts[i])
		case seg.literal != parts[i]:
			return nil, false
		}
	}
	return params, len(parts) == len(segments)
}

// splitPath splits a path into its non-empty segments: "/a//b/" is ["a", "b"].
func splitPath(path string) []string {
	parts := []string{}
	for _, part := range strings.Split(path, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// decodePath reverts percent-encoding. Invalid escape sequences are kept as written, so a
// malformed path still renders instead of failing.
func decodePath(encoded string) string {
	var out strings.Builder
	for i := 0; i < len(encoded); i++ {
		if encoded[i] == '%' && i+2 < len(encoded) {
			if value, ok := unhex(encoded[i+1], encoded[i+2]); ok {
				out.WriteByte(value)
				i += 2
				continue
			}
		}
		out.WriteByte(encoded[i])
	}
	return out.String()
}

func unhex(high, low byte) (byte, bool) {
	h, okHigh := hexValue(high)
	l, okLow := hexValue(low)
	return h<<4 | l, okHigh && okLow
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// User-facing texts of the not-found page; they copy the Rust TITLE_NOT_FOUND and PAGE_NOT_FOUND.
var (
	msgNotFoundTitle = i18n.Msg{En: "Not found", Es: "No encontrada", Zh: "未找到", De: "Nicht gefunden", Fr: "Introuvable"}
	msgNotFoundText  = i18n.Msg{En: "Page not found.", Es: "Página no encontrada.", Zh: "未找到页面。", De: "Seite nicht gefunden.", Fr: "Page introuvable."}
)

func notFoundRoute() Route {
	return Route{
		Icon:  "?",
		Title: func(ctx context.Context, _ Params) string { return msgNotFoundTitle.T(ctx) },
	}
}
