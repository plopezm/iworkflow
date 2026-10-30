package components

import (
	"fmt"
	"strings"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// DeleteRadius is the radius of the delete button for a selected edge.
const DeleteRadius = 12.0

// Base names for edge arrow marker SVG definitions.
const (
	ArrowDone    = "iwf-arrow-done"
	ArrowWorking = "iwf-arrow-working"
	ArrowPending = "iwf-arrow-pending"
	ArrowActive  = "iwf-arrow-active"
)

// EdgeClass returns the CSS class for an edge given its state and whether it is selected.
func EdgeClass(edgeState state.EdgeState, selected bool) string {
	var variant string
	switch edgeState {
	case state.EdgeStateDone:
		variant = "graph-edge-done"
	case state.EdgeStateWorking:
		variant = "graph-edge-working"
	default:
		variant = "graph-edge-pending"
	}
	if selected {
		return "graph-edge " + variant + " graph-edge-selected"
	}
	return "graph-edge " + variant
}

// MarkerScope creates a deterministic SVG-safe marker scope identifier from a tab path.
// Non-alphanumeric runes are hex-escaped so multiple canvas instances do not collide.
func MarkerScope(path string) string {
	if path == "" {
		return "board"
	}
	var b strings.Builder
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			fmt.Fprintf(&b, "_%x_", r)
		}
	}
	return b.String()
}

// MarkerID computes the scoped marker ID for SVG marker definitions.
func MarkerID(scope, name string) string {
	return name + "-" + scope
}

// ArrowMarker returns the SVG marker-end URL attribute value for an edge.
func ArrowMarker(edgeState state.EdgeState, selected bool, scope string) string {
	var name string
	if selected {
		name = ArrowActive
	} else {
		switch edgeState {
		case state.EdgeStateDone:
			name = ArrowDone
		case state.EdgeStateWorking:
			name = ArrowWorking
		default:
			name = ArrowPending
		}
	}
	return fmt.Sprintf("url(#%s)", MarkerID(scope, name))
}
