package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/logs/presentation/components"
)

func renderCard(t *testing.T, p components.LogsCardProps) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, components.LogsCard(p).Render(context.Background(), &buf))
	return buf.String()
}

func TestLogsCardRefreshesWithAMorphOfItsContent(t *testing.T) {
	html := renderCard(t, components.LogsCardProps{Title: "Logs", Lines: []string{"a"}})
	require.Contains(t, html, `hx-swap="morph:innerHTML"`)
	require.Contains(t, html, `hx-sync="this:replace"`)
	require.Contains(t, html, `hx-get="/logs/card"`)
}

func TestLogsCardShowsLinesOrTheEmptyTextOrTheError(t *testing.T) {
	html := renderCard(t, components.LogsCardProps{Title: "Logs", Lines: []string{"first <b>", "second"}})
	require.Contains(t, html, "first &lt;b&gt;")
	require.Contains(t, html, "second")
	require.Contains(t, renderCard(t, components.LogsCardProps{Title: "Logs", Empty: "nothing"}), "nothing")
	failed := renderCard(t, components.LogsCardProps{Title: "Logs", Error: "boom", Lines: []string{"hidden"}})
	require.Contains(t, failed, "boom")
	require.NotContains(t, failed, "hidden")
}
