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

func TestLinksBareURLsInANewTab(t *testing.T) {
	html := mustRender(t, "ver https://a.dev/x?q=1 y www.b.dev.")

	assert.Contains(t, html, `<a href="https://a.dev/x?q=1" target="_blank" rel="noopener noreferrer">`)
	assert.Contains(t, html, `href="http://www.b.dev"`)
}

func TestExplicitHTTPLinksOpenInANewTab(t *testing.T) {
	html := mustRender(t, "[ok](https://a.dev) [local](/tasks/1)")

	assert.Contains(t, html, `target="_blank"`)
	assert.Contains(t, html, `<a href="/tasks/1">local</a>`)
}

func TestLinksPathsOfTheDiskInEveryOS(t *testing.T) {
	cases := map[string]string{
		"abre /home/u/app/main.go ahora":       "/home/u/app/main.go",
		"mira ~/notas/a.md":                    "~/notas/a.md",
		"en ./src/lib.rs y listo":              "./src/lib.rs",
		"edita src/main.go.":                   "src/main.go",
		"falla en /srv/app/x.go:42:7 hoy":      "/srv/app/x.go:42:7",
		"en C:\\Users\\me\\app\\main.go ok":    "C:\\Users\\me\\app\\main.go",
		"en D:/work/app/main.go ok":            "D:/work/app/main.go",
		"**/var/log/syslog**":                  "/var/log/syslog",
		"`\\\\server\\share\\doc.txt`":         "\\\\server\\share\\doc.txt",
		"`C:\\Program Files`":                  "",
		"`internal/core/markdown/markdown.go`": "internal/core/markdown/markdown.go",
	}
	for source, path := range cases {
		t.Run(source, func(t *testing.T) {
			html := mustRender(t, source)
			if path == "" {
				assert.NotContains(t, html, "data-file-path")
				return
			}
			assert.Contains(t, html, `class="file-link" data-file-path="`+path+`"`)
			assert.Contains(t, html, `href="#"`)
		})
	}
}

func TestDoesNotLinkProseThatIsNotAPath(t *testing.T) {
	for _, source := range []string{"y/o 1/2 and TCP/IP", "usa `fmt.Println` o `go test`", "https://a.dev/some/file.go"} {
		t.Run(source, func(t *testing.T) {
			assert.NotContains(t, mustRender(t, source), "data-file-path")
		})
	}
}

func TestPathLinksKeepEscapingAndContext(t *testing.T) {
	html := mustRender(t, "[doc](/docs/a.md) y `/tmp/<x>.go`")

	assert.NotContains(t, html, "data-file-path")
	html = mustRender(t, "ver /tmp/a&b.go")
	assert.Contains(t, html, `data-file-path="/tmp/a&amp;b.go"`)
}
