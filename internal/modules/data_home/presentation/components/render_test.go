package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/data_home/presentation/components"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &buf))
	return buf.String()
}

func baseProps() components.DataHomeCardProps {
	return components.DataHomeCardProps{
		Title:                  "Data folder",
		Help:                   "Moves the data folder.",
		Current:                "/data/iwf",
		CurrentLabel:           "Current folder",
		SourceText:             "Default folder",
		DestinationLabel:       "New folder",
		DestinationPlaceholder: "/path/to/folder",
		MoveLabel:              "Move folder",
		CancelLabel:            "Cancel the pending move",
	}
}

func TestDataHomeCardRendersTheMoveFormPostingToTheMoveRoute(t *testing.T) {
	body := render(t, components.DataHomeCard(baseProps()))

	require.Contains(t, body, `id="data-home-card"`)
	require.Contains(t, body, `hx-post="/data-home/move"`)
	require.Contains(t, body, `name="destination"`)
	require.Contains(t, body, "Move folder")
	require.Contains(t, body, "/data/iwf")
}

func TestDataHomeCardIsReadOnlyWhenIWFHomeIsSet(t *testing.T) {
	props := baseProps()
	props.EnvLocked = true
	props.EnvNote = "IWF_HOME is set."

	body := render(t, components.DataHomeCard(props))

	require.Contains(t, body, "IWF_HOME is set.")
	require.NotContains(t, body, `hx-post="/data-home/move"`)
}

func TestDataHomeCardOffersCancelWhileAMoveIsPending(t *testing.T) {
	props := baseProps()
	props.PendingText = "A move to /mnt/iwf waits for the next start."

	body := render(t, components.DataHomeCard(props))

	require.Contains(t, body, "waits for the next start")
	require.Contains(t, body, `hx-post="/data-home/cancel"`)
	require.Contains(t, body, "Cancel the pending move")
}

func TestDataHomeCardPollsWhileTheServerRestarts(t *testing.T) {
	props := baseProps()
	props.Restarting = true
	props.RestartingText = "Restarting to move the data folder."

	body := render(t, components.DataHomeCard(props))

	require.Contains(t, body, `hx-get="/data-home/card"`)
	require.Contains(t, body, `hx-trigger="every 2s"`)
	require.Contains(t, body, "Restarting to move the data folder.")
	require.NotContains(t, body, `hx-post="/data-home/move"`)
}

func TestDataHomeCardShowsOnlyTheReadErrorWhenTheFolderCannotBeRead(t *testing.T) {
	props := components.DataHomeCardProps{Title: "Data folder", Error: "pointer is corrupt"}

	body := render(t, components.DataHomeCard(props))

	require.Contains(t, body, "pointer is corrupt")
	require.NotContains(t, body, `hx-post="/data-home/move"`)
}

func TestDataHomeCardEscapesTheNoticeAndWarnings(t *testing.T) {
	props := baseProps()
	props.Notice = "<script>alert(1)</script>"
	props.Warnings = []string{"<b>kept</b>"}

	body := render(t, components.DataHomeCard(props))

	require.NotContains(t, body, "<script>alert(1)</script>")
	require.True(t, strings.Contains(body, "&lt;script&gt;"))
	require.NotContains(t, body, "<b>kept</b>")
}
