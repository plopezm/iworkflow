package components

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
)

// Port of the Rust tests of src/modules/tasks/presentation/components/transcript_view.rs, plus the image label
// tests of src/modules/tasks/presentation/state/transcript.rs.

func transcriptCtx(locale i18n.Locale) context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
}

func TestImagesLabelIsSingularForOneImageAndPluralOtherwise(t *testing.T) {
	t.Parallel()
	ctx := transcriptCtx(i18n.En)

	assert.Equal(t, "📎 1 image", imagesLabel(ctx, 1))
	assert.Equal(t, "📎 2 images", imagesLabel(ctx, 2))
}

func TestLaunchNoticesWithArgsOrCwdAreCollapsedUnderALocalizedSummary(t *testing.T) {
	t.Parallel()
	summary, collapsed := launchNoticeSummary(transcriptCtx(i18n.Es), "Arrancando agente args=[--x] cwd=/wt")

	require.True(t, collapsed)
	assert.Equal(t, "Detalles de arranque", summary)

	_, collapsed = launchNoticeSummary(transcriptCtx(i18n.Es), "Conflictos en a.rs")
	assert.False(t, collapsed)
}

func TestArgsSummaryPrefersThePathOverTheRawJSON(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "src/a.rs", argsSummary(`{"limit":230,"offset":40,"path":"src/a.rs"}`))
	assert.Equal(t, "no es json", argsSummary("no es json"))
}

// Rule from source: without a known key the summary is empty, so no unrelated value (which could be a secret) shows.
func TestArgsSummaryIgnoresUnknownStringKeys(t *testing.T) {
	t.Parallel()
	assert.Empty(t, argsSummary(`{"token":"s3cr3t","limit":3}`))
}

func TestArgRowsFlattensTheJSONInKeyOrder(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []argRow{{Key: "command", Value: "ls -la"}, {Key: "limit", Value: "3"}}, argRows(`{"limit":3,"command":"ls -la"}`))
	assert.Empty(t, argRows(""))
	assert.Equal(t, []argRow{{Key: "", Value: "texto"}}, argRows("texto"))
}

func TestPreviewCollapsesWhitespaceAndTruncatesToEightyCharacters(t *testing.T) {
	t.Parallel()
	assert.Equal(t, `{ "a": 1 }`, previewText("{\n  \"a\": 1\n}"))

	long := previewText(strings.Repeat("x", 200))
	assert.True(t, strings.HasSuffix(long, "…"))
	assert.Equal(t, 81, len([]rune(long)), "eighty characters and the ellipsis")
}

// The five languages are complete for every text of the transcript.
func TestTranscriptTextsExistInEveryLanguage(t *testing.T) {
	t.Parallel()
	texts := map[string]i18n.Msg{
		"launch details":  msgTranscriptLaunchDetails,
		"you":             msgTranscriptRoleUser,
		"agent":           msgTranscriptRoleAgent,
		"reasoning":       msgTranscriptReasoning,
		"running":         msgTranscriptToolRunning,
		"ok":              msgTranscriptToolOK,
		"error":           msgTranscriptToolError,
		"answered":        msgTranscriptDialogAnswered,
		"pending":         msgTranscriptDialogPending,
		"stopped code":    msgTranscriptStoppedCode,
		"stopped no code": msgTranscriptStoppedNoCode,
		"no activity":     msgTranscriptNoActivity,
		"writing":         msgTranscriptWriting,
		"one image":       msgTranscriptOneImage,
		"many images":     msgTranscriptManyImages,
	}
	for name, msg := range texts {
		for _, locale := range i18n.All {
			assert.NotEmpty(t, msg.In(locale), "%s in %s", name, locale)
		}
	}
}
