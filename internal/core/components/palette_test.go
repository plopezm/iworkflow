package components_test

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
)

func TestParsePaletteFallsBackToTheDefault(t *testing.T) {
	for _, option := range components.PaletteOptions() {
		require.Equal(t, option.Palette, components.ParsePalette(string(option.Palette)))
	}
	require.Equal(t, components.PaletteDefault, components.ParsePalette("neon"))
	require.Equal(t, components.PaletteDefault, components.ParsePalette(""))
	require.Equal(t, "pi-palette", components.PaletteStorageKey)
}

func TestPaletteCardOffersEveryPaletteAndTheDefaultFirst(t *testing.T) {
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	var out bytes.Buffer

	require.NoError(t, components.PaletteCard().Render(ctx, &out))

	html := out.String()
	for _, value := range []string{"default", "ocean", "forest", "sunset", "rose"} {
		require.Contains(t, html, `data-palette-choice="`+value+`"`)
	}
	require.Less(t, bytes.Index(out.Bytes(), []byte(`data-palette-choice="default"`)), bytes.Index(out.Bytes(), []byte(`data-palette-choice="ocean"`)))
	require.Contains(t, html, "Tema de color")
	require.Contains(t, html, "Predeterminado")
}

func TestPaletteListMatchesTheClientScript(t *testing.T) {
	source, err := os.ReadFile("../../../web/static/js/ui.js")
	require.NoError(t, err)
	match := regexp.MustCompile(`PALETTES = \[([^\]]*)\]`).FindSubmatch(source)
	require.NotNil(t, match, "ui.js must declare PALETTES")

	var client []string
	for _, quoted := range strings.Split(string(match[1]), ",") {
		client = append(client, strings.Trim(strings.TrimSpace(quoted), "'\""))
	}
	var server []string
	for _, option := range components.PaletteOptions() {
		server = append(server, string(option.Palette))
	}
	require.Equal(t, server, client)
}
