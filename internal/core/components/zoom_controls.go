package components

import (
	"fmt"
	"math"

	"github.com/a-h/templ"
)

// ZoomPercentLabel shows a zoom level as a rounded percentage (1.0 becomes "100%").
func ZoomPercentLabel(zoom float64) string {
	return fmt.Sprintf("%d%%", int64(math.Round(zoom*100)))
}

// ZoomControlsProps configures the floating zoom-in / zoom-out pill of a zoomable canvas.
type ZoomControlsProps struct {
	Zoom         float64
	ZoomInLabel  string
	ZoomOutLabel string
	ZoomInAttrs  templ.Attributes
	ZoomOutAttrs templ.Attributes
}
