package components

import (
	"fmt"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// NodeStyle returns the CSS style attribute string that positions a node card on the canvas
// with the standard width (264px) and height (140px).
func NodeStyle(position state.Point) string {
	return fmt.Sprintf("left: %.1fpx; top: %.1fpx; width: %.0fpx; height: %.0fpx;", position.X, position.Y, state.NodeWidth, state.NodeHeight)
}

// NodeClass returns the CSS classes for a node card reflecting its selection, connection target,
// and attention states.
func NodeClass(selected, connectTarget, needsAttention bool) string {
	class := "graph-node group"
	if selected {
		class += " graph-node-selected"
	}
	if connectTarget {
		class += " graph-node-target"
	}
	if needsAttention {
		class += " graph-node-attention"
	}
	return class
}
