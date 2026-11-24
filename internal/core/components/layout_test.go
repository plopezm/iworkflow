package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
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

func TestWorkspaceNameTintsAColoredWorkspaceAndLeavesTheOthersPlain(t *testing.T) {
	colored := render(t, components.WorkspaceName("Demo", "blue"))
	assert.Contains(t, colored, "ws-name ws-blue")
	assert.Contains(t, colored, "Demo")

	for _, color := range []string{"", "chartreuse"} {
		plain := render(t, components.WorkspaceName("Demo", color))
		assert.NotContains(t, plain, "ws-name")
		assert.Contains(t, plain, "Demo")
	}
}

func TestWorkspaceLabelIsThePillForAKnownColorAndTheBareNameOtherwise(t *testing.T) {
	assert.Contains(t, render(t, components.WorkspaceLabel("Demo", "blue")), "ws-name ws-blue")
	for _, color := range []string{"", "chartreuse"} {
		assert.Equal(t, "Demo", render(t, components.WorkspaceLabel("Demo", color)))
	}
}

func TestColorPickerOffersEveryColorAndChecksOnlyTheSelectedOne(t *testing.T) {
	html := render(t, components.ColorPicker(components.ColorPickerProps{Label: "Color", NoneLabel: "No color", Name: "color", Value: "teal"}))

	for _, key := range components.WorkspaceColorKeys() {
		assert.Contains(t, html, `value="`+key+`"`)
	}
	assert.Equal(t, 1, strings.Count(html, "checked"), "exactly one option is checked")
	assert.Contains(t, html, `value="teal" checked`)

	none := render(t, components.ColorPicker(components.ColorPickerProps{Label: "Color", NoneLabel: "No color", Name: "color"}))
	assert.Contains(t, none, `value="" checked`)
}

func TestCustomHexColorRendersADotThroughAnInlineVariable(t *testing.T) {
	html := render(t, components.WorkspaceName("Demo", "#1a2b3c"))

	assert.Contains(t, html, "ws-name ws-custom")
	assert.Contains(t, html, `style="--ws:#1a2b3c;"`)
	assert.Contains(t, html, `<span class="ws-dot"`)
}

func TestInvalidHexColorRendersThePlainName(t *testing.T) {
	html := render(t, components.WorkspaceName("Demo", "#1A2B3C"))

	assert.NotContains(t, html, "ws-name")
	assert.NotContains(t, html, "style=")
}

func TestColorPickerOffersACustomSwatchSelectedForAHexValue(t *testing.T) {
	props := components.ColorPickerProps{Label: "Color", NoneLabel: "None", Name: "color", Value: "#1a2b3c"}
	html := render(t, components.ColorPicker(props))

	assert.Contains(t, html, `value="#1a2b3c" checked`)
	assert.Contains(t, html, `data-ws-custom-input`)
	assert.Equal(t, 1, strings.Count(html, " checked"))

	palette := render(t, components.ColorPicker(components.ColorPickerProps{Label: "Color", NoneLabel: "None", Name: "color", Value: "blue"}))
	assert.Contains(t, palette, `data-ws-custom-radio`)
	assert.NotContains(t, palette, `value="#`+"1a2b3c")
	assert.Equal(t, 1, strings.Count(palette, " checked"))
}
