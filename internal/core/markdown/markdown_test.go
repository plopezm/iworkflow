package markdown

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustRender(t *testing.T, source string) string {
	t.Helper()
	html, err := Render(source)
	require.NoError(t, err)
	return html
}

func TestRendersBoldAndLists(t *testing.T) {
	html := mustRender(t, "**Advertencias**\n- **Fallo:** texto")

	assert.Contains(t, html, "<strong>Advertencias</strong>")
	assert.Contains(t, html, "<li><strong>Fallo:</strong> texto</li>")
}

func TestEscapesRawHTML(t *testing.T) {
	html := mustRender(t, "<script>alert(1)</script>")

	assert.NotContains(t, html, "<script>")
	assert.Contains(t, html, "&lt;script&gt;")
}

func TestEscapesInlineRawHTML(t *testing.T) {
	html := mustRender(t, "before <b onclick=\"x\">bold</b> after")

	assert.NotContains(t, html, "<b ")
	assert.Contains(t, html, "&lt;b onclick=&quot;x&quot;&gt;")
}

func TestNeutralizesUnsafeLinks(t *testing.T) {
	html := mustRender(t, "[x](javascript:alert(1)) [ok](https://a.dev)")

	assert.NotContains(t, html, "javascript:")
	assert.Contains(t, html, `href="https://a.dev"`)
}

func TestRejectsMixedCaseDataAndProtocolRelativeURLs(t *testing.T) {
	for _, url := range []string{"JavaScript:alert(1)", "data:text/html,x", "//evil.com/x", "/\\evil.com/x"} {
		t.Run(url, func(t *testing.T) {
			link := mustRender(t, fmt.Sprintf("[x](%s)", url))
			assert.Contains(t, link, `href="#"`)

			image := mustRender(t, fmt.Sprintf("![x](%s)", url))
			assert.Contains(t, image, `src="#"`)
		})
	}
}

func TestNeutralizesUnsafeAutolinks(t *testing.T) {
	html := mustRender(t, "<javascript:alert(1)>")

	assert.NotContains(t, html, `href="javascript:`)
	assert.Contains(t, html, `href="#"`)
}

func TestKeepsRootRelativeURLs(t *testing.T) {
	html := mustRender(t, "[x](/tasks/1)")

	assert.Contains(t, html, `href="/tasks/1"`)
}

func TestEnablesTablesStrikethroughAndTaskLists(t *testing.T) {
	html := mustRender(t, "| a | b |\n|---|---|\n| 1 | 2 |\n\n~~gone~~\n\n- [x] done")

	assert.Contains(t, html, "<table>")
	assert.Contains(t, html, "<del>gone</del>")
	assert.Contains(t, html, `type="checkbox"`)
}
