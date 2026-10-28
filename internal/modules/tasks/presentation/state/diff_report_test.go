package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// Port of the Rust tests of src/modules/tasks/presentation/state/diff_report.rs (the pure logic of the printable
// diff report).

var isErrorFalse = false

func TestExplanationPromptAsksForTheExplanationsInSpanish(t *testing.T) {
	t.Parallel()

	assert.Contains(t, state.ExplanationPrompt, "markdown")
	assert.Contains(t, state.ExplanationPrompt, "alternativas descartadas")
}

func TestSplitPatchPerFileSplitsOnEveryDiffHeader(t *testing.T) {
	t.Parallel()

	patch := "diff --git a/src/a.rs b/src/a.rs\n--- a/src/a.rs\n+++ b/src/a.rs\n@@ -1 +1 @@\n-a\n+b\ndiff --git a/b.txt b/b.txt\n+z"

	files := state.SplitPatchByFile(patch)

	assert.Len(t, files, 2)
	assert.Equal(t, "src/a.rs", files[0].Name)
	assert.Len(t, files[0].Lines, 6)
	assert.Equal(t, []string{"diff --git a/src/a.rs b/src/a.rs", "--- a/src/a.rs", "+++ b/src/a.rs", "@@ -1 +1 @@", "-a", "+b"}, files[0].Lines)
	assert.Equal(t, "b.txt", files[1].Name)
	assert.Equal(t, []string{"diff --git a/b.txt b/b.txt", "+z"}, files[1].Lines)
}

func TestSplitPatchWithoutHeaderIsANamelessChunk(t *testing.T) {
	t.Parallel()

	files := state.SplitPatchByFile("+a\n-b")

	assert.Len(t, files, 1)
	assert.Empty(t, files[0].Name)
	assert.Equal(t, []string{"+a", "-b"}, files[0].Lines)
}

func TestSplitPatchKeepsTheLinesBeforeTheFirstHeader(t *testing.T) {
	t.Parallel()

	files := state.SplitPatchByFile("preamble\ndiff --git a/x b/x\n+1")

	assert.Len(t, files, 2)
	assert.Empty(t, files[0].Name)
	assert.Equal(t, "x", files[1].Name)
}

func TestSplitPatchOfAnEmptyPatchHasNoFiles(t *testing.T) {
	t.Parallel()

	assert.Empty(t, state.SplitPatchByFile(""))
}

func TestSplitPatchFallsBackToTheTextOfAnUnparsableHeader(t *testing.T) {
	t.Parallel()

	files := state.SplitPatchByFile("diff --git weird")

	assert.Len(t, files, 1)
	assert.Equal(t, "weird", files[0].Name)
}

func TestFindExplanationRejectsNegativeSince(t *testing.T) {
	t.Parallel()

	transcript := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "uno"},
	}}

	answer, ok := state.FindExplanation(transcript, "P", -1)

	assert.False(t, ok)
	assert.Empty(t, answer)
}

func TestFindExplanationTakesTheLastAssistantAnswerAfterThePrompt(t *testing.T) {
	t.Parallel()

	transcript := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "hola"},
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "uno"},
		state.AssistantItem{Text: "dos"},
	}}

	answer, ok := state.FindExplanation(transcript, "P", 0)

	assert.True(t, ok)
	assert.Equal(t, "uno\n\ndos", answer)
}

func TestFindExplanationIsNotReadyWithoutATrailingAssistantAnswer(t *testing.T) {
	t.Parallel()

	transcript := state.Transcript{Items: []state.TranscriptItem{
		state.AssistantItem{Text: "viejo"},
		state.UserItem{Text: "P"},
	}}

	answer, ok := state.FindExplanation(transcript, "P", 0)

	assert.False(t, ok)
	assert.Empty(t, answer)
}

func TestFindExplanationIsNotReadyWhenThePromptIsMissing(t *testing.T) {
	t.Parallel()

	transcript := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "otro"},
		state.AssistantItem{Text: "x"},
	}}

	answer, ok := state.FindExplanation(transcript, "P", 0)

	assert.False(t, ok)
	assert.Empty(t, answer)
}

func TestFindExplanationIsNotReadyWhileAToolIsRunning(t *testing.T) {
	t.Parallel()

	running := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "a"},
		state.ToolItem{CallID: "c1", Name: "bash", Args: "", Output: "", IsError: nil},
	}}
	answer, ok := state.FindExplanation(running, "P", 0)

	assert.False(t, ok)
	assert.Empty(t, answer)

	runningBeforeAnswer := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.ToolItem{CallID: "c1", Name: "bash", Args: "", Output: "", IsError: nil},
		state.AssistantItem{Text: "a"},
	}}
	answer, ok = state.FindExplanation(runningBeforeAnswer, "P", 0)

	assert.False(t, ok)
	assert.Empty(t, answer)
}

func TestFindExplanationIsReadyWhenTheToolFinishedBeforeTheAnswer(t *testing.T) {
	t.Parallel()

	finished := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.ToolItem{CallID: "c1", Name: "bash", Args: "", Output: "", IsError: &isErrorFalse},
		state.AssistantItem{Text: "a"},
	}}

	answer, ok := state.FindExplanation(finished, "P", 0)

	assert.True(t, ok)
	assert.Equal(t, "a", answer)
}

func TestFindExplanationIsNotReadyWhenTheToolFinishedAfterTheAnswer(t *testing.T) {
	t.Parallel()

	finished := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "a"},
		state.ToolItem{CallID: "c1", Name: "bash", Args: "", Output: "", IsError: &isErrorFalse},
	}}

	answer, ok := state.FindExplanation(finished, "P", 0)

	assert.False(t, ok)
	assert.Empty(t, answer)
}

func TestFindExplanationIsNotReadyWhileStreaming(t *testing.T) {
	t.Parallel()

	streaming := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "a"},
	}, StreamingText: "sigue"}
	answer, ok := state.FindExplanation(streaming, "P", 0)

	assert.False(t, ok)
	assert.Empty(t, answer)
}

func TestFindExplanationIgnoresStatusItems(t *testing.T) {
	t.Parallel()

	transcript := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "a"},
		state.StatusItem{Status: "running"},
	}}

	answer, ok := state.FindExplanation(transcript, "P", 0)

	assert.True(t, ok)
	assert.Equal(t, "a", answer)
}

func TestFindExplanationTakesTheLastMatchingPrompt(t *testing.T) {
	t.Parallel()

	transcript := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "viejo"},
		state.UserItem{Text: "P"},
	}}

	answer, ok := state.FindExplanation(transcript, "P", 0)

	assert.False(t, ok)
	assert.Empty(t, answer)
}

func TestFindExplanationIgnoresTheAnswersBeforeTheBaseline(t *testing.T) {
	t.Parallel()

	transcript := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "viejo"},
	}}

	answer, ok := state.FindExplanation(transcript, "P", 2)
	assert.False(t, ok)
	assert.Empty(t, answer)

	answer, ok = state.FindExplanation(transcript, "P", 9)
	assert.False(t, ok)
	assert.Empty(t, answer)
}

func TestFindExplanationIsNotReadyForABlankAnswer(t *testing.T) {
	t.Parallel()

	transcript := state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: "P"},
		state.AssistantItem{Text: "  "},
	}}

	answer, ok := state.FindExplanation(transcript, "P", 0)

	assert.False(t, ok)
	assert.Empty(t, answer)
}
