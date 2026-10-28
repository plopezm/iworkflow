package components

import (
	"fmt"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// GridSize is the spacing between dots of the background grid in canvas coordinates.
const GridSize = 24.0

// GridStyle returns the background CSS style string that matches the viewport pan and zoom.
func GridStyle(viewport state.Viewport) string {
	size := GridSize * viewport.Zoom
	return fmt.Sprintf("background-size: %.2fpx %.2fpx; background-position: %.2fpx %.2fpx;", size, size, viewport.X, viewport.Y)
}
