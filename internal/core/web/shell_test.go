package web_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
)

func TestShellSetsLanguageTitleAndAssets(t *testing.T) {
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.De, Chosen: true})
	var out bytes.Buffer

	require.NoError(t, web.Shell("Board").Render(ctx, &out))

	html := out.String()
	require.Contains(t, html, `<html lang="de">`)
	require.Contains(t, html, "<title>Board</title>")
	require.Contains(t, html, `/static/app.css`)
	require.Contains(t, html, `/static/vendor/htmx.min.js`)
	require.Contains(t, html, "pi-theme")
}
