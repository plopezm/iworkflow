package inbound

import (
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"
)

// qrQuietZone is the blank border around the code, in modules, as the QR standard asks.
const qrQuietZone = 4

// RenderQRSVG returns an inline SVG of the QR code for url. ok is false when the text cannot be
// encoded. The modules are drawn as one path so the markup stays small.
func RenderQRSVG(url string) (svg string, ok bool) {
	code, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", false
	}
	bitmap := code.Bitmap()
	size := len(bitmap) + 2*qrQuietZone

	var path strings.Builder
	for y, row := range bitmap {
		for x, dark := range row {
			if dark {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", x+qrQuietZone, y+qrQuietZone)
			}
		}
	}
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %[1]d %[1]d" shape-rendering="crispEdges">`+
			`<rect width="%[1]d" height="%[1]d" fill="#ffffff"/><path fill="#000000" d="%[2]s"/></svg>`,
		size, path.String(),
	), true
}
