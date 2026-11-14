package app

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnEmptyGroupOffersTheDestinationsAsATileGrid(t *testing.T) {
	var out strings.Builder
	require.NoError(t, emptyGroup().Render(context.Background(), &out))

	html := out.String()
	require.Contains(t, html, `class="launcher-grid"`)
	require.Contains(t, html, `href="/workspaces"`)
	require.Contains(t, html, `href="/settings"`)
}
