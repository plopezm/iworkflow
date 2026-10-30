package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/components"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &out))
	return out.String()
}

func TestCardRendersTitleAndChildren(t *testing.T) {
	var out bytes.Buffer
	ctx := templ.WithChildren(context.Background(), templ.Raw("<p>body</p>"))
	require.NoError(t, components.Card("Tasks").Render(ctx, &out))

	require.Contains(t, out.String(), `<h2 class="card-title">Tasks</h2>`)
	require.Contains(t, out.String(), "<p>body</p>")
}

func TestCardOmitsTheTitleWhenEmpty(t *testing.T) {
	require.NotContains(t, render(t, components.Card("")), "card-title")
}

func TestBadgeAppliesTheVariantClass(t *testing.T) {
	require.Contains(t, render(t, components.Badge("Done", "ok")), `class="badge badge-ok"`)
}

func TestErrorTextEscapesTheMessage(t *testing.T) {
	html := render(t, components.ErrorText("<b>x</b>"))

	require.Contains(t, html, "&lt;b&gt;x&lt;/b&gt;")
	require.NotContains(t, html, "<b>")
}
