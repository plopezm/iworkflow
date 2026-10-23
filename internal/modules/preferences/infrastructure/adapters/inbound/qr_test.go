package inbound

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQRSVGIsAnInlineSVGWithTheQuietZone(t *testing.T) {
	svg, ok := RenderQRSVG("http://192.168.1.5:3000/?token=abc")

	require.True(t, ok)
	require.True(t, strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg"`))
	require.True(t, strings.HasSuffix(svg, `</svg>`))
	require.Contains(t, svg, `<path fill="#000000" d="M`)
	require.Contains(t, svg, `fill="#ffffff"`)
}

func TestQRSVGIsDeterministicForTheSameURL(t *testing.T) {
	first, _ := RenderQRSVG("http://10.0.0.2:3000")
	second, _ := RenderQRSVG("http://10.0.0.2:3000")

	require.Equal(t, first, second)
}

func TestQRSVGChangesWithTheURL(t *testing.T) {
	first, _ := RenderQRSVG("http://10.0.0.2:3000/?token=aaa")
	second, _ := RenderQRSVG("http://10.0.0.2:3000/?token=bbb")

	require.NotEqual(t, first, second)
}
