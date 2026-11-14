package components

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubmitProgressRendersTitleStepsPhasesAndHint(t *testing.T) {
	var out bytes.Buffer
	err := SubmitProgress(SubmitProgressProps{
		Title:    "Creating",
		Steps:    []ProgressStep{{Label: "Save"}, {Label: "Worktree", Phase: "start"}},
		SlowHint: "Slow <now>",
	}).Render(context.Background(), &out)
	require.NoError(t, err)
	html := out.String()
	require.Contains(t, html, `data-submit-progress`)
	require.Contains(t, html, `Creating`)
	require.Contains(t, html, `data-progress-phase="start"`)
	require.Contains(t, html, `Worktree`)
	require.Contains(t, html, `data-progress-slow hidden`)
	require.Contains(t, html, `Slow &lt;now&gt;`)
}
